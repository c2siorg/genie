package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrTokenNotFound is returned when an MCP token lookup misses.
var ErrTokenNotFound = errors.New("mcp token not found")

// MCPToken is the per-user, per-provider session token vault entry.
type MCPToken struct {
	ID        string
	UserID    string
	Provider  string
	Endpoint  string
	Payload   crypto.EncryptedPayload
	CreatedAt time.Time
}

// MCPTokenRepo is the storage surface for MCP session tokens.
type MCPTokenRepo interface {
	Upsert(ctx context.Context, t MCPToken) (MCPToken, error)
	Get(ctx context.Context, userID, provider string) (MCPToken, error)
}

// PgMCPTokenRepo is the pgx-backed implementation.
type PgMCPTokenRepo struct{ DB *DB }

func NewMCPTokenRepo(db *DB) *PgMCPTokenRepo { return &PgMCPTokenRepo{DB: db} }

func (r *PgMCPTokenRepo) Upsert(ctx context.Context, t MCPToken) (MCPToken, error) {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(t.Payload)
	if err != nil {
		return MCPToken{}, err
	}
	_, err = r.DB.Pool.Exec(ctx,
		`INSERT INTO mcp_tokens (id, user_id, provider, endpoint, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, provider) DO UPDATE SET
		   endpoint = EXCLUDED.endpoint,
		   payload = EXCLUDED.payload,
		   created_at = EXCLUDED.created_at`,
		t.ID, t.UserID, t.Provider, t.Endpoint, body, t.CreatedAt,
	)
	if err != nil {
		return MCPToken{}, err
	}
	return t, nil
}

func (r *PgMCPTokenRepo) Get(ctx context.Context, userID, provider string) (MCPToken, error) {
	row := r.DB.Pool.QueryRow(ctx,
		`SELECT id, user_id, provider, endpoint, payload, created_at
		   FROM mcp_tokens WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	)
	var t MCPToken
	var raw []byte
	if err := row.Scan(&t.ID, &t.UserID, &t.Provider, &t.Endpoint, &raw, &t.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MCPToken{}, ErrTokenNotFound
		}
		return MCPToken{}, err
	}
	if err := json.Unmarshal(raw, &t.Payload); err != nil {
		return MCPToken{}, err
	}
	return t, nil
}
