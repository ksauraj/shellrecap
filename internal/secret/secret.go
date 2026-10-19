// internal/secret/secret.go

// Package secret seals the API keys built into release binaries, so they
// don't sit in the binary as plain text for strings, grep or the bots that
// scan published binaries for keys to harvest.
//
// This is obfuscation, not encryption: the program has to be able to read
// its own keys, so someone determined to dig them out of a binary can. Keys
// built in should have quotas or spend limits that make that not worth it.
package secret

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"strings"
)

// prefix marks a sealed value and its format version
const prefix = "s1."

const (
	saltSize = 16
	tagSize  = 8
)

// pepper is mixed into every keystream. It's assembled at run time rather
// than stored as one string, so it doesn't show up next to the keys.
func pepper() []byte {
	parts := []string{"shell", "recap", ":", "seal", ":", "v1"}
	return []byte(strings.Join(parts, "\x1f"))
}

// keystream derives n bytes from the salt with SHA-256 in counter mode
func keystream(salt []byte, n int) []byte {
	out := make([]byte, 0, n+sha256.Size)
	var counter [4]byte
	for i := uint32(0); len(out) < n; i++ {
		binary.BigEndian.PutUint32(counter[:], i)
		h := sha256.New()
		h.Write(salt)
		h.Write(pepper())
		h.Write(counter[:])
		out = h.Sum(out)
	}
	return out[:n]
}

// tag checks that a value was sealed by Seal and opened intact
func tag(salt, plain []byte) []byte {
	h := sha256.New()
	h.Write(pepper())
	h.Write(salt)
	h.Write(plain)
	return h.Sum(nil)[:tagSize]
}

// Seal obfuscates a key for building into a binary. Each call uses a fresh
// random salt, so sealing the same key twice gives different values.
func Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	data := []byte(plain)
	stream := keystream(salt, len(data))
	sealed := make([]byte, 0, saltSize+len(data)+tagSize)
	sealed = append(sealed, salt...)
	for i, b := range data {
		sealed = append(sealed, b^stream[i])
	}
	sealed = append(sealed, tag(salt, data)...)
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open recovers a key sealed by Seal. Anything else, including a key built
// in as plain text, gives "", as if no key was built in.
func Open(sealed string) string {
	if !strings.HasPrefix(sealed, prefix) {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(sealed[len(prefix):])
	if err != nil || len(raw) <= saltSize+tagSize {
		return ""
	}
	salt, body, want := raw[:saltSize], raw[saltSize:len(raw)-tagSize], raw[len(raw)-tagSize:]
	stream := keystream(salt, len(body))
	plain := make([]byte, len(body))
	for i, b := range body {
		plain[i] = b ^ stream[i]
	}
	if subtle.ConstantTimeCompare(tag(salt, plain), want) != 1 {
		return ""
	}
	return string(plain)
}
