// Package oauth_device implements the OAuth 2.0 device authorisation grant
// (RFC 8628). Used to onboard Genie users to a third-party MCP server
// (Zerodha Kite, etc.) without making them paste session tokens.
//
// Flow:
//
//  1. Client calls Begin() — returns a verification_uri + user_code +
//     device_code + polling interval.
//  2. User opens verification_uri on their phone, types user_code, logs in.
//  3. The external IdP calls Approve(device_code, token) when the user
//     finishes.
//  4. Client polls Poll(device_code) every interval seconds; once approved
//     it returns the token.
//
// Genie's role here is the *client*: we initiate the flow with the remote
// IdP, expose user_code to the user, and poll. For Kite specifically the
// IdP-side approval is whatever Zerodha provides; this package's
// in-memory Service is also handy for tests + when Genie itself hosts the
// IdP (e.g. for cross-Genie A2A onboarding).
package oauth_device

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"
)

// AuthorizationResponse is what Begin returns to the client.
type AuthorizationResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the eventual token + metadata.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ErrPending is returned by Poll while the user has not yet approved.
var ErrPending = errors.New("authorization_pending")

// ErrSlowDown asks the client to back off and lengthen its polling interval.
var ErrSlowDown = errors.New("slow_down")

// ErrExpired means the device_code expired.
var ErrExpired = errors.New("expired_token")

// ErrDenied means the user explicitly rejected.
var ErrDenied = errors.New("access_denied")

// Service is Genie's in-process IdP — useful for tests and for Genie
// instances offering device flow to siblings. Production should swap with a
// proper OAuth server (Hydra, Auth0, Keycloak).
type Service struct {
	VerificationURI string
	TTL             time.Duration
	Interval        int // seconds; default 5

	mu      sync.Mutex
	pending map[string]*pendingAuth
}

type pendingAuth struct {
	userCode  string
	createdAt time.Time
	approved  bool
	denied    bool
	token     string
}

// New constructs an IdP service.
func New(verificationURI string, ttl time.Duration) *Service {
	if ttl == 0 {
		ttl = 10 * time.Minute
	}
	return &Service{
		VerificationURI: verificationURI,
		TTL:             ttl,
		Interval:        5,
		pending:         map[string]*pendingAuth{},
	}
}

// Begin starts a new authorisation. Returns the response the client should
// display to the user. The device_code is the long opaque blob the client
// keeps; the user_code is the short human-typable string.
func (s *Service) Begin() (AuthorizationResponse, error) {
	deviceCode, err := randomToken(40)
	if err != nil {
		return AuthorizationResponse{}, err
	}
	userCode := randomUserCode()
	s.mu.Lock()
	s.pending[deviceCode] = &pendingAuth{userCode: userCode, createdAt: time.Now().UTC()}
	s.mu.Unlock()
	return AuthorizationResponse{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURI: s.VerificationURI,
		ExpiresIn:       int(s.TTL.Seconds()),
		Interval:        s.Interval,
	}, nil
}

// Approve completes the flow on the user's behalf. Caller must look up the
// device_code by user_code (which it received over the wire from the user).
func (s *Service) Approve(userCode, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pending {
		if p.userCode == userCode {
			if s.expired(p) {
				return ErrExpired
			}
			p.approved = true
			p.token = token
			return nil
		}
	}
	return errors.New("oauth_device: unknown user_code")
}

// Deny marks an attempt as rejected by the user.
func (s *Service) Deny(userCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.pending {
		if p.userCode == userCode {
			p.denied = true
			return nil
		}
	}
	return errors.New("oauth_device: unknown user_code")
}

// Poll is what the client calls until the flow completes.
func (s *Service) Poll(deviceCode string) (TokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[deviceCode]
	if !ok {
		return TokenResponse{}, ErrExpired
	}
	if s.expired(p) {
		delete(s.pending, deviceCode)
		return TokenResponse{}, ErrExpired
	}
	if p.denied {
		delete(s.pending, deviceCode)
		return TokenResponse{}, ErrDenied
	}
	if !p.approved {
		return TokenResponse{}, ErrPending
	}
	// One-shot — clear after delivery.
	delete(s.pending, deviceCode)
	return TokenResponse{
		AccessToken: p.token,
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	}, nil
}

func (s *Service) expired(p *pendingAuth) bool {
	return time.Since(p.createdAt) > s.TTL
}

// randomToken returns a base64url-encoded random blob.
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// randomUserCode returns an 8-character Crockford-style base32 code,
// chunked with a dash: e.g. "WDJF-K9X3".
func randomUserCode() string {
	b := make([]byte, 5) // 40 bits
	_, _ = rand.Read(b)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
	enc = strings.ToUpper(strings.NewReplacer("0", "X", "1", "Y", "8", "Z").Replace(enc))
	if len(enc) < 8 {
		return enc
	}
	return enc[:4] + "-" + enc[4:8]
}
