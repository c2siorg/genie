package afg

import (
	"context"
	"strings"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/web/mid"
)

// identityFromContext reads the authenticated identity attached to ctx by
// mid.Auth (JWT middleware). It returns the JWT subject as userID and a
// comma-joined list of roles, matching the wire format RBACPolicy /
// extractRoles expects on protocol.Message.Metadata[protocol.MetaKeyUserRoles].
//
// ok is false when no claims are present on the context (the caller should
// treat this as "no identity" — in practice unreachable once mid.Auth is
// wired ahead of the handler, since Auth itself returns 401 first).
func identityFromContext(ctx context.Context) (userID, rolesCSV string, ok bool) {
	claims, ok := mid.ClaimsFrom(ctx)
	if !ok {
		return "", "", false
	}
	roles := make([]string, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		roles = append(roles, string(r))
	}
	return claims.Subject, strings.Join(roles, ","), true
}
