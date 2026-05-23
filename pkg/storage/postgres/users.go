package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrUserNotFound is returned when a user lookup fails.
var ErrUserNotFound = errors.New("user not found")

// UserRepo is the data-access surface the HTTP user handler needs.
type UserRepo interface {
	Create(ctx context.Context, email, name, passwordHash string, roles []auth.Role) (auth.User, error)
	GetByEmail(ctx context.Context, email string) (auth.User, error)
	GetByID(ctx context.Context, id string) (auth.User, error)
}

// PgUserRepo is the pgx-backed implementation.
type PgUserRepo struct{ DB *DB }

func NewUserRepo(db *DB) *PgUserRepo { return &PgUserRepo{DB: db} }

func (r *PgUserRepo) Create(ctx context.Context, email, name, hash string, roles []auth.Role) (auth.User, error) {
	if len(roles) == 0 {
		roles = []auth.Role{auth.RoleUser}
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	roleStrings := rolesToStrings(roles)
	_, err := r.DB.Pool.Exec(ctx,
		`INSERT INTO users (id, email, name, password_hash, roles, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $6)`,
		id, email, name, hash, roleStrings, now,
	)
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: id, Email: email, Name: name, PasswordHash: hash, Roles: roles, CreatedAt: now, UpdatedAt: now}, nil
}

func (r *PgUserRepo) GetByEmail(ctx context.Context, email string) (auth.User, error) {
	return r.queryOne(ctx, `SELECT id, email, name, password_hash, roles, created_at, updated_at FROM users WHERE email = $1`, email)
}

func (r *PgUserRepo) GetByID(ctx context.Context, id string) (auth.User, error) {
	return r.queryOne(ctx, `SELECT id, email, name, password_hash, roles, created_at, updated_at FROM users WHERE id = $1`, id)
}

func (r *PgUserRepo) queryOne(ctx context.Context, q string, args ...any) (auth.User, error) {
	row := r.DB.Pool.QueryRow(ctx, q, args...)
	var u auth.User
	var roleStrings []string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &roleStrings, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, ErrUserNotFound
		}
		return auth.User{}, err
	}
	u.Roles = stringsToRoles(roleStrings)
	return u, nil
}

func rolesToStrings(rs []auth.Role) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return out
}

func stringsToRoles(ss []string) []auth.Role {
	out := make([]auth.Role, len(ss))
	for i, s := range ss {
		out[i] = auth.Role(s)
	}
	return out
}
