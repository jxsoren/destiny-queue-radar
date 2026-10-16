// Package cryptobox provides AES-256-GCM authenticated encryption for secrets such as OAuth tokens.
package cryptobox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// Box encrypts and decrypts small secrets with AES-256-GCM. It is safe for concurrent use.
type Box struct {
	gcm cipher.AEAD
}

// New returns a Box using a 32-byte AES-256 key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("cryptobox: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cryptobox: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cryptobox: %w", err)
	}
	return &Box{gcm: gcm}, nil
}

// Encrypt seals plaintext and returns nonce || ciphertext || tag, using a fresh random nonce.
// aad is authenticated but not encrypted or stored; the same aad must be supplied to Decrypt.
// Use it to bind a ciphertext to its context (for example the owning user and kind of secret).
func (b *Box) Encrypt(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cryptobox: nonce: %w", err)
	}
	return b.gcm.Seal(nonce, nonce, plaintext, aad), nil
}

// Decrypt opens a value produced by Encrypt (nonce || ciphertext || tag). It fails if the data was
// modified, the key is wrong, or aad differs from the aad given to Encrypt.
func (b *Box) Decrypt(ciphertext, aad []byte) ([]byte, error) {
	ns := b.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("cryptobox: ciphertext too short")
	}
	pt, err := b.gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], aad)
	if err != nil {
		return nil, fmt.Errorf("cryptobox: decrypt: %w", err)
	}
	return pt, nil
}
