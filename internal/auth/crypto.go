// Package auth implements password-derived AES-256-GCM encryption for the
// on-disk store. The master password is never persisted; an Argon2id KDF
// derives the key on each unlock.
//
// File format:
//
//	magic(4) "CLY1" || version(1)=1 || salt(16) || nonce(12) || ciphertext || tag(16)
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	magic       = "CLY1"
	formatVer   = 1
	saltLen     = 16
	nonceLen    = 12
	keyLen      = 32
	argonTime   = 3
	argonMemKiB = 64 * 1024 // 64 MiB
	argonPar    = 4
)

var ErrBadPassword = errors.New("bad password or corrupted store")

// DeriveKey runs Argon2id over (password, salt) → 32-byte key.
func DeriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonTime, argonMemKiB, argonPar, keyLen)
}

// Seal encrypts plaintext under a freshly random salt + nonce and returns the
// full file blob ready to write to disk.
func Seal(password, plaintext []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	key := DeriveKey(password, salt)
	return sealWithKey(key, salt, plaintext)
}

// SealWith re-seals plaintext reusing an existing salt+key (e.g. for
// subsequent saves within a session so we don't pay Argon2 every write).
func SealWith(key, salt, plaintext []byte) ([]byte, error) {
	if len(salt) != saltLen {
		return nil, errors.New("auth: bad salt length")
	}
	return sealWithKey(key, salt, plaintext)
}

func sealWithKey(key, salt, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)

	out := make([]byte, 0, len(magic)+1+saltLen+nonceLen+len(ct))
	out = append(out, magic...)
	out = append(out, byte(formatVer))
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, nil
}

// Open parses a blob, derives the key from password, and returns
// (plaintext, key, salt). Key + salt are returned so callers can re-seal
// future writes without re-running Argon2id.
func Open(password, blob []byte) (plaintext, key, salt []byte, err error) {
	hdrLen := len(magic) + 1 + saltLen + nonceLen
	if len(blob) < hdrLen+16 {
		return nil, nil, nil, ErrBadPassword
	}
	if string(blob[:4]) != magic {
		return nil, nil, nil, errors.New("auth: not a coachly store")
	}
	if blob[4] != formatVer {
		return nil, nil, nil, errors.New("auth: unsupported format version")
	}
	salt = append([]byte(nil), blob[5:5+saltLen]...)
	nonce := blob[5+saltLen : 5+saltLen+nonceLen]
	ct := blob[hdrLen:]

	key = DeriveKey(password, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, nil, err
	}
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, nil, nil, ErrBadPassword
	}
	return pt, key, salt, nil
}
