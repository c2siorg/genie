package oauth_device

import (
	"errors"
	"testing"
	"time"
)

func TestDeviceFlow_Approve(t *testing.T) {
	svc := New("https://verify.example/", 5*time.Minute)
	begin, err := svc.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if begin.UserCode == "" || begin.DeviceCode == "" {
		t.Fatal("missing codes")
	}
	// Poll should be pending first.
	if _, err := svc.Poll(begin.DeviceCode); !errors.Is(err, ErrPending) {
		t.Fatalf("expected ErrPending, got %v", err)
	}
	if err := svc.Approve(begin.UserCode, "kite-session-token"); err != nil {
		t.Fatal(err)
	}
	tok, err := svc.Poll(begin.DeviceCode)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "kite-session-token" {
		t.Fatalf("token mismatch: %+v", tok)
	}
	// Second poll should now show expired (one-shot).
	if _, err := svc.Poll(begin.DeviceCode); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired after consume, got %v", err)
	}
}

func TestDeviceFlow_Deny(t *testing.T) {
	svc := New("https://verify.example/", time.Minute)
	begin, _ := svc.Begin()
	_ = svc.Deny(begin.UserCode)
	if _, err := svc.Poll(begin.DeviceCode); !errors.Is(err, ErrDenied) {
		t.Fatalf("expected ErrDenied, got %v", err)
	}
}

func TestDeviceFlow_Expired(t *testing.T) {
	svc := New("https://verify.example/", 1*time.Nanosecond)
	begin, _ := svc.Begin()
	time.Sleep(2 * time.Millisecond)
	if _, err := svc.Poll(begin.DeviceCode); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}
