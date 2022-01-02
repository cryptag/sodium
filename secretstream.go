package sodium

// #cgo pkg-config: libsodium
// #include <stdlib.h>
// #include <sodium.h>
import "C"

import (
	"io"
)

var (
	cryptoSecretStreamXChaCha20Poly1305KeyBytes  = int(C.crypto_secretstream_xchacha20poly1305_keybytes())
	cryptoSecretStreamXChaCha20Poly1305HeaderBytes  = int(C.crypto_secretstream_xchacha20poly1305_headerbytes())
)

// SecretStreamTag can be set to encoder for modify stream state or can be get from decoder
type SecretStreamTag uint8

const (
	// normal message chunk
	SecretStreamTag_Message = iota
	// message boundary, the last chunk of message
	SecretStreamTag_Sync 
	// explicity rekeying
	SecretStreamTag_Rekey 
)

func (tag *SecretStreamTag) fromCtag(ctag C.uchar) {
	switch ctag {
	case C.crypto_secretstream_xchacha20poly1305_tag_push():
		*tag = SecretStreamTag_Sync
	case C.crypto_secretstream_xchacha20poly1305_tag_rekey():
		*tag = SecretStreamTag_Rekey
	default:
		*tag = SecretStreamTag_Message
	}
	return
}

func (tag SecretStreamTag) toCtag() (ctag C.uchar) {
	switch tag {
	case SecretStreamTag_Sync:
		ctag = C.crypto_secretstream_xchacha20poly1305_tag_push()
	case SecretStreamTag_Rekey:
		ctag = C.crypto_secretstream_xchacha20poly1305_tag_rekey()
	default:
		ctag = C.crypto_secretstream_xchacha20poly1305_tag_message()
	}
	return
}

type SecretStreamXCPKey struct {
	Bytes
}

func (SecretStreamXCPKey) Size() int {
	return cryptoSecretStreamXChaCha20Poly1305KeyBytes
}

// MakeSecretStreamXCPKey initilize the key
func MakeSecretStreamXCPKey() SecretStreamXCPKey {
	b := make([]byte, cryptoSecretStreamXChaCha20Poly1305KeyBytes);
	C.crypto_secretstream_xchacha20poly1305_keygen((*C.uchar)(&b[0]))
	return SecretStreamXCPKey{b}
}

// SecretStreamXCPHeader generated by encoder and can be transferred in plain text. It must set to decoder before decoding
type SecretStreamXCPHeader struct {
	Bytes
}

func (SecretStreamXCPHeader) Size() int {
	return cryptoSecretStreamXChaCha20Poly1305HeaderBytes
}

type SecretStreamEncoder interface {
	io.WriteCloser
	Header() SecretStreamXCPHeader
	SetAdditionData(ad []byte)
	SetTag(SecretStreamTag)
	WriteAndClose(b []byte) (n int, err error)
}

type SecretStreamDecoder interface {
	io.Reader
	SetAdditionData(ad []byte)
	Tag() SecretStreamTag
}

type SecretStreamXCPEncoder struct {
	out io.Writer
	header SecretStreamXCPHeader
	state C.crypto_secretstream_xchacha20poly1305_state
	ad Bytes
	tag SecretStreamTag
	final bool
}

type SecretStreamXCPDecoder struct {
	in io.Reader
	state C.crypto_secretstream_xchacha20poly1305_state
	ad Bytes
	tag SecretStreamTag
	final bool
 }

// Header get the header from encoder
func (e SecretStreamXCPEncoder) Header() SecretStreamXCPHeader {
	return e.header
}

func (e *SecretStreamXCPEncoder) SetAdditionData(ad []byte) {
	e.ad = ad[:]
}

func (e *SecretStreamXCPEncoder) SetTag(t SecretStreamTag) {
	e.tag = t
}

// Write encrypts the b as a message and write to the wrapped io.Writer
func (e *SecretStreamXCPEncoder) Write(b []byte) (n int, err error) {
	if e.final {
		return n, ErrInvalidState
	}
	mp, ml := plen(b)
	c := make([]byte, ml + int(C.crypto_secretstream_xchacha20poly1305_abytes()))
	cp, _ := plen(c)
	adp, adl := plen(e.ad)
	if int(C.crypto_secretstream_xchacha20poly1305_push(&e.state,
		(*C.uchar)(cp),
		(*C.ulonglong)(nil),
		(*C.uchar)(mp),
		(C.ulonglong)(ml),
		(*C.uchar)(adp),
		(C.ulonglong)(adl),
		e.tag.toCtag())) != 0 {
		return 0, ErrUnknown
	}
	n, err = e.out.Write(c)
	return
}

// Write encrypts the b as a message and write to the wrapped io.Writer and then write the closing signal
func (e *SecretStreamXCPEncoder) WriteAndClose(b []byte) (n int, err error) {
	if e.final {
		return n, ErrInvalidState
	}
	mp, ml := plen(b)
	c := make([]byte, ml + int(C.crypto_secretstream_xchacha20poly1305_abytes()))
	cp, _ := plen(c)
	adp, adl := plen(e.ad)
	if int(C.crypto_secretstream_xchacha20poly1305_push(&e.state,
		(*C.uchar)(cp),
		(*C.ulonglong)(nil),
		(*C.uchar)(mp),
		(C.ulonglong)(ml),
		(*C.uchar)(adp),
		(C.ulonglong)(adl),
		C.crypto_secretstream_xchacha20poly1305_tag_final())) != 0 {
		return 0, ErrUnknown
	}
	n, err = e.out.Write(c)
	e.final = true
	return
}

// Write encrypts the closing signal and write to the wrapped io.Writer and then close it
func (e *SecretStreamXCPEncoder) Close() error {
	mac := make([]byte, int(C.crypto_secretstream_xchacha20poly1305_abytes()))
	ap, _ := plen(mac)
	adp, adl := plen(e.ad)
	if int(C.crypto_secretstream_xchacha20poly1305_push(&e.state,
		(*C.uchar)(ap),
		(*C.ulonglong)(nil),
		(*C.uchar)(nil),
		0,
		(*C.uchar)(adp),
		(C.ulonglong)(adl),
		C.crypto_secretstream_xchacha20poly1305_tag_final())) != 0 {
		return ErrUnknown
	}
	_, err := e.out.Write(mac)
	e.final = true
	return err
}

func MakeSecretStreamXCPEncoder (key SecretStreamXCPKey, out io.Writer) SecretStreamEncoder {
	checkTypedSize(&key, "secret stream key")
	encoder := SecretStreamXCPEncoder{
		out: out,
		header: SecretStreamXCPHeader{make([]byte, cryptoSecretStreamXChaCha20Poly1305HeaderBytes)},
	}
	if int(C.crypto_secretstream_xchacha20poly1305_init_push(
		&encoder.state,
		(*C.uchar)(&encoder.header.Bytes[0]),
		(*C.uchar)(&key.Bytes[0]))) != 0 {
		panic("see libsodium")
	}
	return &encoder
}

// Read decrypts the message with length len(b) and save in b. It returns io.EOF when receiving a closing signal
func (e *SecretStreamXCPDecoder) Read(b []byte) (n int, err error) {
	if e.final {
		return n, ErrInvalidState
	}
	bp, bl := plen(b)
	c := make([]byte, bl + int(C.crypto_secretstream_xchacha20poly1305_abytes()))

	var l int
	l, err = e.in.Read(c)
	for l < int(C.crypto_secretstream_xchacha20poly1305_abytes()) {
		if err != nil {
			return 0, ErrDecryptSS
		}
		var more int
		more, err = e.in.Read(c[:l])
		l += more
	}
	adp, adl := plen(e.ad)
	var tag C.uchar
	if int(C.crypto_secretstream_xchacha20poly1305_pull(
		&e.state,
		(*C.uchar)(bp),
		(*C.ulonglong)(nil),
		&tag,
		(*C.uchar)(&c[0]),
		(C.ulonglong)(l),
		(*C.uchar)(adp),
		(C.ulonglong)(adl))) != 0 {
		err = ErrDecryptSS
	}
	n = l - int(C.crypto_secretstream_xchacha20poly1305_abytes())
	e.tag.fromCtag(tag)
	if tag == C.crypto_secretstream_xchacha20poly1305_tag_final() {
		err = io.EOF
		e.final = true
	}
	return
}

func (e *SecretStreamXCPDecoder) SetAdditionData(ad []byte) {
	e.ad = ad[:]
}

func (e SecretStreamXCPDecoder) Tag() SecretStreamTag {
	return e.tag
}

func MakeSecretStreamXCPDecoder (key SecretStreamXCPKey, in io.Reader, header SecretStreamXCPHeader) (SecretStreamDecoder, error) {
	checkTypedSize(&key, "secret stream key")
	checkTypedSize(&header, "secret stream header")
	decoder := SecretStreamXCPDecoder{
		in:    in,
	}
	if int(C.crypto_secretstream_xchacha20poly1305_init_pull(
		&decoder.state,
		(*C.uchar)(&header.Bytes[0]),
		(*C.uchar)(&key.Bytes[0]))) != 0 {
		return nil, ErrInvalidHeader
	}
	return &decoder, nil
}
