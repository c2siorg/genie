package auth

import (
	"errors"
	"testing"
	"time"
)

func TestIssueVerify_Roundtrip(t *testing.T) {
	iss := NewIssuer([]byte("test-secret"), "genie", []string{"genie-api"}, time.Minute)
	tok, claims, err := iss.Issue("u-1", "a@b.com", []Role{RoleUser, RoleAdvisor})
	if err != nil {
		t.Fatal(err)
	}
	if !claims.HasRole(RoleAdvisor) {
		t.Fatalf("claims missing role: %+v", claims)
	}

	got, err := iss.Verify(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "u-1" {
		t.Fatalf("subject mismatch: %q", got.Subject)
	}
}

func TestVerify_BadSignature(t *testing.T) {
	good := NewIssuer([]byte("k1"), "genie", nil, time.Minute)
	bad := NewIssuer([]byte("k2"), "genie", nil, time.Minute)
	tok, _, _ := good.Issue("u-1", "a@b.com", []Role{RoleUser})
	if _, err := bad.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestVerify_Expired(t *testing.T) {
	iss := NewIssuer([]byte("k"), "genie", nil, -time.Second)
	tok, _, _ := iss.Issue("u-1", "a@b.com", nil)
	if _, err := iss.Verify(tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "hunter2"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyPassword(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
}
