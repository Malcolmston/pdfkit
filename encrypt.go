package pdfkit

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"encoding/binary"
	"fmt"
	"html"
	"strings"
)

// passPad is the 32-byte password padding string from the PDF standard security
// handler (Algorithm 2, step (a)).
var passPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41,
	0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80,
	0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

// EncryptOptions configures standard PDF document encryption. An empty
// UserPassword produces a document that opens without a password but still has
// its permission flags and (optionally) an owner password enforced.
type EncryptOptions struct {
	// UserPassword must be supplied to open the document (may be empty).
	UserPassword string
	// OwnerPassword grants full access; defaults to UserPassword if empty.
	OwnerPassword string
	// UseAES selects AES-128 (V4/R4) instead of 128-bit RC4 (V2/R3).
	UseAES bool

	// Permission flags. When all are false the document is maximally locked
	// (subject to viewer honor).
	AllowPrinting    bool
	AllowModify      bool
	AllowCopying     bool
	AllowAnnotations bool
}

// encryptConfig is the internal, prepared encryption state.
type encryptConfig struct {
	opts   EncryptOptions
	perm   int32
	key    []byte // file encryption key (16 bytes)
	o, u   []byte // /O and /U entries (32 bytes each)
	useAES bool
	r, v   int
}

// SetEncryption enables standard security on the document using the given
// options. It must be called before Write/Bytes/Save. RC4-128 is used by
// default; set EncryptOptions.UseAES for AES-128.
func (d *Document) SetEncryption(opts EncryptOptions) {
	perm := int32(-1)
	perm &^= 0b11 // reserved bits 1,2 must be 0
	perm &^= 4 | 8 | 16 | 32 | 256 | 512 | 1024 | 2048
	if opts.AllowPrinting {
		perm |= 4 | 2048
	}
	if opts.AllowModify {
		perm |= 8 | 1024
	}
	if opts.AllowCopying {
		perm |= 16 | 512
	}
	if opts.AllowAnnotations {
		perm |= 32 | 256
	}
	cfg := &encryptConfig{opts: opts, perm: perm, useAES: opts.UseAES, r: 3, v: 2}
	if opts.UseAES {
		cfg.r, cfg.v = 4, 4
	}
	d.enc = cfg
}

// padPassword pads or truncates pw to the standard 32-byte block.
func padPassword(pw string) []byte {
	b := make([]byte, 32)
	n := copy(b, pw)
	copy(b[n:], passPad)
	return b
}

// prepare computes the /O and /U entries and the file encryption key using the
// document ID as the first element of the file identifier.
func (e *encryptConfig) prepare(id []byte) {
	owner := e.opts.OwnerPassword
	if owner == "" {
		owner = e.opts.UserPassword
	}

	// Algorithm 3: compute /O.
	ownerPad := padPassword(owner)
	sum := md5.Sum(ownerPad)
	digest := sum[:]
	for i := 0; i < 50; i++ { // R>=3
		s := md5.Sum(digest[:16])
		digest = s[:]
	}
	rc4Key := digest[:16]
	o := make([]byte, 32)
	copy(o, padPassword(e.opts.UserPassword))
	rc4Apply(rc4Key, o)
	for i := 1; i <= 19; i++ {
		k := make([]byte, len(rc4Key))
		for j := range rc4Key {
			k[j] = rc4Key[j] ^ byte(i)
		}
		rc4Apply(k, o)
	}
	e.o = o

	// Algorithm 2: compute file key.
	h := md5.New()
	h.Write(padPassword(e.opts.UserPassword))
	h.Write(e.o)
	var p [4]byte
	binary.LittleEndian.PutUint32(p[:], uint32(e.perm))
	h.Write(p[:])
	h.Write(id)
	key := h.Sum(nil)[:16]
	for i := 0; i < 50; i++ { // R>=3
		s := md5.Sum(key)
		key = s[:16]
	}
	e.key = key

	// Algorithm 5: compute /U (R>=3).
	uh := md5.New()
	uh.Write(passPad)
	uh.Write(id)
	u := uh.Sum(nil) // 16 bytes
	rc4Apply(e.key, u)
	for i := 1; i <= 19; i++ {
		k := make([]byte, len(e.key))
		for j := range e.key {
			k[j] = e.key[j] ^ byte(i)
		}
		rc4Apply(k, u)
	}
	full := make([]byte, 32)
	copy(full, u)
	e.u = full
}

// objectKey derives the per-object encryption key (Algorithm 1).
func (e *encryptConfig) objectKey(num, gen int) []byte {
	h := md5.New()
	h.Write(e.key)
	var b [5]byte
	b[0] = byte(num)
	b[1] = byte(num >> 8)
	b[2] = byte(num >> 16)
	b[3] = byte(gen)
	b[4] = byte(gen >> 8)
	h.Write(b[:])
	if e.useAES {
		h.Write([]byte{0x73, 0x41, 0x6C, 0x54}) // "sAlT"
	}
	sum := h.Sum(nil)
	n := len(e.key) + 5
	if n > 16 {
		n = 16
	}
	return sum[:n]
}

// encrypt encrypts data for object (num, gen) using the configured cipher.
func (e *encryptConfig) encrypt(num, gen int, data []byte) []byte {
	key := e.objectKey(num, gen)
	if !e.useAES {
		out := make([]byte, len(data))
		copy(out, data)
		rc4Apply(key, out)
		return out
	}
	// AES-128 CBC with a random 16-byte IV prepended and PKCS#7 padding.
	block, err := aes.NewCipher(key)
	if err != nil {
		return data
	}
	iv := make([]byte, aes.BlockSize)
	_, _ = rand.Read(iv)
	padLen := aes.BlockSize - len(data)%aes.BlockSize
	padded := append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padLen)}, padLen)...)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return append(iv, out...)
}

// rc4Apply applies RC4 with key to data in place.
func rc4Apply(key, data []byte) {
	c, err := rc4.NewCipher(key)
	if err != nil {
		return
	}
	c.XORKeyStream(data, data)
}

// dict renders the /Encrypt dictionary body.
func (e *encryptConfig) dict() string {
	var b strings.Builder
	fmt.Fprintf(&b, "<< /Filter /Standard /V %d /R %d /Length 128", e.v, e.r)
	if e.useAES {
		b.WriteString(" /CF << /StdCF << /CFM /AESV2 /Length 16 /AuthEvent /DocOpen >> >> /StmF /StdCF /StrF /StdCF")
	}
	fmt.Fprintf(&b, " /O (%s) /U (%s) /P %d >>",
		escapeString(string(e.o)), escapeString(string(e.u)), e.perm)
	return b.String()
}

// EnableXMP generates and attaches an XMP metadata packet built from the current
// /Info metadata (Title, Author, Subject, Keywords, Creator, Producer). Call it
// after setting metadata and before writing.
func (d *Document) EnableXMP() {
	d.xmp = generateXMP(d.info)
}

// SetXMP attaches a custom XMP metadata packet verbatim, overriding EnableXMP.
func (d *Document) SetXMP(xmp string) {
	d.xmp = xmp
}

// generateXMP builds a minimal Dublin Core / XMP packet from info fields.
func generateXMP(info map[string]string) string {
	esc := func(s string) string { return html.EscapeString(s) }
	var b strings.Builder
	b.WriteString(`<?xpacket begin="` + "" + `" id="W5M0MpCehiHzreSzNTczkc9d"?>` + "\n")
	b.WriteString(`<x:xmpmeta xmlns:x="adobe:ns:meta/">` + "\n")
	b.WriteString(`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` + "\n")
	b.WriteString(`<rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:pdf="http://ns.adobe.com/pdf/1.3/" xmlns:xmp="http://ns.adobe.com/xap/1.0/">` + "\n")
	if t := info["Title"]; t != "" {
		b.WriteString(`<dc:title><rdf:Alt><rdf:li xml:lang="x-default">` + esc(t) + `</rdf:li></rdf:Alt></dc:title>` + "\n")
	}
	if a := info["Author"]; a != "" {
		b.WriteString(`<dc:creator><rdf:Seq><rdf:li>` + esc(a) + `</rdf:li></rdf:Seq></dc:creator>` + "\n")
	}
	if s := info["Subject"]; s != "" {
		b.WriteString(`<dc:description><rdf:Alt><rdf:li xml:lang="x-default">` + esc(s) + `</rdf:li></rdf:Alt></dc:description>` + "\n")
	}
	if k := info["Keywords"]; k != "" {
		b.WriteString(`<pdf:Keywords>` + esc(k) + `</pdf:Keywords>` + "\n")
	}
	if c := info["Creator"]; c != "" {
		b.WriteString(`<xmp:CreatorTool>` + esc(c) + `</xmp:CreatorTool>` + "\n")
	}
	if pr := info["Producer"]; pr != "" {
		b.WriteString(`<pdf:Producer>` + esc(pr) + `</pdf:Producer>` + "\n")
	}
	b.WriteString(`</rdf:Description>` + "\n</rdf:RDF>\n</x:xmpmeta>\n")
	b.WriteString(`<?xpacket end="w"?>`)
	return b.String()
}

// metadataBody renders the /Metadata XML stream.
func metadataBody(xmp string, num int, enc *encryptConfig) []byte {
	return streamObject(num, "/Type /Metadata /Subtype /XML", []byte(xmp), enc)
}
