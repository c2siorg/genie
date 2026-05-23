// Package oauth2 implements the OAuth 2.1 authorization-code grant with
// mandatory PKCE — Genie's recommended way for third-party clients
// (notebook, mobile, browser SPA) to obtain a Genie API token.
//
// OAuth 2.1 drops the implicit + password grants and *requires* PKCE for
// public clients. The flow:
//
//  1. Client computes verifier + challenge (S256), calls Authorize().
//  2. Genie returns an authorization code bound to (verifier, scope, user).
//  3. Client calls Token() with code + verifier; Genie checks
//     SHA256(verifier) == challenge and issues a JWT via pkg/auth.Issuer.
package oauth2

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

// Method describes which challenge transform was used. OAuth 2.1 only
// allows S256; "plain" is here as a defensive check (and is rejected).
type Method string

const (
	MethodS256  Method = "S256"
	MethodPlain Method = "plain"
)

// AuthorizeRequest is what a client posts to start the flow.
type AuthorizeRequest struct {
	ClientID            string
	Scope               []string
	CodeChallenge       string
	CodeChallengeMethod Method
	RedirectURI         string
}

// AuthorizeResponse is the redirect target Genie sends back.
type AuthorizeResponse struct {
	Code        string
	RedirectURI string
}

// TokenRequest is the token-endpoint payload.
type TokenRequest struct {
	Code         string
	CodeVerifier string
	ClientID     string
	RedirectURI  string
}

// TokenResponse is the bearer JWT + metadata.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// Errors are returned with OAuth 2.1 error codes so clients can branch.
var (
	ErrInvalidRequest        = errors.New("invalid_request")
	ErrInvalidGrant          = errors.New("invalid_grant")
	ErrUnsupportedChallenge  = errors.New("unsupported_code_challenge_method")
)

// Server is the in-memory issuer. Production stores codes in Redis with TTL.
type Server struct {
	Issuer  *auth.Issuer
	CodeTTL time.Duration

	mu    sync.Mutex
	codes map[string]*pendingCode
}

type pendingCode struct {
	createdAt   time.Time
	challenge   string
	method      Method
	subject     string
	email       string
	roles       []auth.Role
	scope       []string
	redirectURI string
	clientID    string
}

// New constructs the server.
func New(issuer *auth.Issuer) *Server {
	return &Server{Issuer: issuer, CodeTTL: 60 * time.Second, codes: map[string]*pendingCode{}}
}

// Authorize binds an authorization code to a logged-in user. Genie's HTTP
// layer calls this after the user authenticated via passkey / password.
// scope is the list the user consented to.
func (s *Server) Authorize(req AuthorizeRequest, subject, email string, roles []auth.Role) (AuthorizeResponse, error) {
	if req.CodeChallenge == "" {
		return AuthorizeResponse{}, ErrInvalidRequest
	}
	if req.CodeChallengeMethod != MethodS256 {
		return AuthorizeResponse{}, ErrUnsupportedChallenge
	}
	code, err := randomToken(32)
	if err != nil {
		return AuthorizeResponse{}, err
	}
	s.mu.Lock()
	s.codes[code] = &pendingCode{
		createdAt:   time.Now().UTC(),
		challenge:   req.CodeChallenge,
		method:      req.CodeChallengeMethod,
		subject:     subject,
		email:       email,
		roles:       roles,
		scope:       req.Scope,
		redirectURI: req.RedirectURI,
		clientID:    req.ClientID,
	}
	s.mu.Unlock()
	return AuthorizeResponse{Code: code, RedirectURI: req.RedirectURI}, nil
}

// Token exchanges an authorization code + verifier for a JWT.
func (s *Server) Token(req TokenRequest) (TokenResponse, error) {
	s.mu.Lock()
	pc, ok := s.codes[req.Code]
	if ok {
		delete(s.codes, req.Code) // one-shot
	}
	s.mu.Unlock()
	if !ok {
		return TokenResponse{}, ErrInvalidGrant
	}
	if time.Since(pc.createdAt) > s.CodeTTL {
		return TokenResponse{}, ErrInvalidGrant
	}
	if pc.clientID != "" && pc.clientID != req.ClientID {
		return TokenResponse{}, ErrInvalidGrant
	}
	if pc.redirectURI != "" && pc.redirectURI != req.RedirectURI {
		return TokenResponse{}, ErrInvalidGrant
	}
	// Verify PKCE — SHA256(verifier) base64url == challenge.
	sum := sha256.Sum256([]byte(req.CodeVerifier))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != pc.challenge {
		return TokenResponse{}, ErrInvalidGrant
	}
	tok, claims, err := s.Issuer.Issue(pc.subject, pc.email, pc.roles)
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: tok,
		TokenType:   "Bearer",
		ExpiresIn:   int(time.Until(time.Unix(claims.ExpiresAt, 0)).Seconds()),
		Scope:       joinScope(pc.scope),
	}, nil
}

// GenerateVerifier produces a fresh PKCE verifier + S256 challenge — handy
// for clients and tests. Verifier is the client's secret; challenge gets
// sent to Authorize().
func GenerateVerifier() (verifier, challenge string, err error) {
	verifier, err = randomToken(32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func joinScope(scope []string) string {
	if len(scope) == 0 {
		return ""
	}
	out := scope[0]
	for _, s := range scope[1:] {
		out += " " + s
	}
	return out
}
