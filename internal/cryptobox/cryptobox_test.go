package cryptobox

import (
	"bytes"
	"testing"
)

func key(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestRoundTrip(t *testing.T) {
	box, err := New(key(1))
	if err != nil {
		t.Fatal(err)
	}
	ct, err := box.Encrypt([]byte("access-token"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := box.Decrypt(ct)
	if err != nil || string(pt) != "access-token" {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
}

func TestEncrypt_UsesFreshNonce(t *testing.T) {
	box, _ := New(key(1))
	a, _ := box.Encrypt([]byte("same"))
	b, _ := box.Encrypt([]byte("same"))
	if bytes.Equal(a, b) {
		t.Error("two encryptions of the same plaintext must differ")
	}
}

func TestDecrypt_RejectsTamperingWrongKeyAndShortInput(t *testing.T) {
	box, _ := New(key(1))
	ct, _ := box.Encrypt([]byte("secret"))

	tampered := append([]byte(nil), ct...)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Decrypt(tampered); err == nil {
		t.Error("tampered ciphertext should fail")
	}
	other, _ := New(key(2))
	if _, err := other.Decrypt(ct); err == nil {
		t.Error("wrong key should fail")
	}
	if _, err := box.Decrypt([]byte{1, 2, 3}); err == nil {
		t.Error("short ciphertext should fail")
	}
}

func TestNew_RequiresAES256Key(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Error("16-byte key should be rejected")
	}
}
