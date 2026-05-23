package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"
)

func setupEnvKey(t *testing.T) *EnvKeyResolver {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	t.Setenv("GENIE_KEK_BASE64", base64.StdEncoding.EncodeToString(key))
	return NewEnvKeyResolver("test-kek-v1")
}

func TestEnvelope_Roundtrip(t *testing.T) {
	r := setupEnvKey(t)
	enc := New(r)
	plain := []byte("private financial doc")
	ep, err := enc.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := enc.Decrypt(ep)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("plaintext mismatch: %q", got)
	}
	if ep.KEKID != "test-kek-v1" {
		t.Errorf("kek id: %q", ep.KEKID)
	}
}

func TestEnvelope_MissingEnv(t *testing.T) {
	_ = os.Unsetenv("GENIE_KEK_BASE64")
	r := NewEnvKeyResolver("test-kek")
	if _, err := New(r).Encrypt([]byte("x")); err == nil {
		t.Fatal("expected env error")
	}
}

func TestEnvelope_WrongKEKID(t *testing.T) {
	r := setupEnvKey(t)
	enc := New(r)
	ep, _ := enc.Encrypt([]byte("x"))
	ep.KEKID = "other-kek"
	if _, err := enc.Decrypt(ep); err == nil {
		t.Fatal("expected kek id mismatch error")
	}
}
