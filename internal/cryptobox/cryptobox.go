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

// Encrypt returns nonce || ciphertext.
func (b *Box) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cryptobox: nonce: %w", err)
	}
	return b.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (b *Box) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := b.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("cryptobox: ciphertext too short")
	}
	pt, err := b.gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("cryptobox: decrypt: %w", err)
	}
	return pt, nil
}
