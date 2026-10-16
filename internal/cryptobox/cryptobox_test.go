package cryptobox

import (
	"bytes"
	"strings"
	"testing"
)

func key(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

var aad = []byte("user-1|access")

func newBox(t *testing.T, b byte) *Box {
	t.Helper()
	box, err := New(key(b))
	if err != nil {
		t.Fatal(err)
	}
	return box
}

func TestRoundTrip(t *testing.T) {
	box := newBox(t, 1)
	ct, err := box.Encrypt([]byte("access-token"), aad)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := box.Decrypt(ct, aad)
	if err != nil || string(pt) != "access-token" {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
}

func TestRoundTrip_EmptyPlaintext(t *testing.T) {
	box := newBox(t, 1)
	ct, err := box.Encrypt(nil, aad)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := box.Decrypt(ct, aad)
	if err != nil || len(pt) != 0 {
		t.Fatalf("decrypt = %q, %v", pt, err)
	}
}

func TestEncrypt_UsesFreshNonce(t *testing.T) {
	box := newBox(t, 1)
	a, err := box.Encrypt([]byte("same"), aad)
	if err != nil {
		t.Fatal(err)
	}
	b, err := box.Encrypt([]byte("same"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a, b) {
		t.Error("two encryptions of the same plaintext must differ")
	}
}

func TestDecrypt_RejectsTamperingWrongKeyAndShortInput(t *testing.T) {
	box := newBox(t, 1)
	ct, err := box.Encrypt([]byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}

	tampered := append([]byte(nil), ct...)
	tampered[len(tampered)-1] ^= 1
	if _, err := box.Decrypt(tampered, aad); err == nil {
		t.Error("tampered ciphertext should fail")
	}
	if _, err := newBox(t, 2).Decrypt(ct, aad); err == nil {
		t.Error("wrong key should fail")
	}
	if _, err := box.Decrypt([]byte{1, 2, 3}, aad); err == nil || !strings.Contains(err.Error(), "too short") {
		t.Errorf("short ciphertext: %v", err)
	}
}

func TestDecrypt_RejectsDifferentAAD(t *testing.T) {
	box := newBox(t, 1)
	ct, err := box.Encrypt([]byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := box.Decrypt(ct, []byte("user-2|access")); err == nil {
		t.Error("different aad should fail")
	}
	if _, err := box.Decrypt(ct, nil); err == nil {
		t.Error("missing aad should fail")
	}
}

func TestDecrypt_NonceOnlyFailsAuthentication(t *testing.T) {
	box := newBox(t, 1)
	_, err := box.Decrypt(make([]byte, 12), aad)
	if err == nil {
		t.Fatal("nonce-only input should fail")
	}
	if strings.Contains(err.Error(), "too short") {
		t.Errorf("want an Open failure, got the length check: %v", err)
	}
}

func TestNew_RequiresAES256Key(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Error("16-byte key should be rejected")
	}
}
