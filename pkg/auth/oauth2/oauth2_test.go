package oauth2

import (
	"errors"
	"testing"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
)

func TestPKCE_FullFlow(t *testing.T) {
	srv := New(auth.NewIssuer([]byte("k"), "genie", nil, time.Hour))
	verifier, challenge, err := GenerateVerifier()
	if err != nil {
		t.Fatal(err)
	}
	authResp, err := srv.Authorize(AuthorizeRequest{
		ClientID:            "cli",
		CodeChallenge:       challenge,
		CodeChallengeMethod: MethodS256,
	}, "u-1", "u@x.com", []auth.Role{auth.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	tok, err := srv.Token(TokenRequest{
		Code:         authResp.Code,
		CodeVerifier: verifier,
		ClientID:     "cli",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken == "" {
		t.Fatal("missing token")
	}
}

func TestPKCE_WrongVerifier(t *testing.T) {
	srv := New(auth.NewIssuer([]byte("k"), "genie", nil, time.Hour))
	_, challenge, _ := GenerateVerifier()
	authResp, _ := srv.Authorize(AuthorizeRequest{
		CodeChallenge:       challenge,
		CodeChallengeMethod: MethodS256,
	}, "u-1", "u@x.com", []auth.Role{auth.RoleUser})
	_, err := srv.Token(TokenRequest{Code: authResp.Code, CodeVerifier: "wrong"})
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("expected invalid_grant, got %v", err)
	}
}

func TestPKCE_RejectsPlainMethod(t *testing.T) {
	srv := New(auth.NewIssuer([]byte("k"), "genie", nil, time.Hour))
	_, err := srv.Authorize(AuthorizeRequest{
		CodeChallenge:       "anything",
		CodeChallengeMethod: MethodPlain,
	}, "u-1", "u@x.com", nil)
	if !errors.Is(err, ErrUnsupportedChallenge) {
		t.Fatalf("OAuth 2.1 must reject plain; got %v", err)
	}
}
