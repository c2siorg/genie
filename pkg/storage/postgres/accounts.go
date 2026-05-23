package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAccountNotFound is returned when an account lookup fails.
var ErrAccountNotFound = errors.New("account not found")

// Account is a per-user labelled bucket of transactions/documents.
type Account struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

// AccountRepo is the data-access surface for accounts.
type AccountRepo interface {
	Create(ctx context.Context, userID, name, currency string) (Account, error)
	ListByUser(ctx context.Context, userID string) ([]Account, error)
	GetByID(ctx context.Context, id string) (Account, error)
}

// PgAccountRepo is the pgx-backed implementation.
type PgAccountRepo struct{ DB *DB }

func NewAccountRepo(db *DB) *PgAccountRepo { return &PgAccountRepo{DB: db} }

func (r *PgAccountRepo) Create(ctx context.Context, userID, name, currency string) (Account, error) {
	if currency == "" {
		currency = "INR"
	}
	a := Account{
		ID:        uuid.NewString(),
		UserID:    userID,
		Name:      name,
		Currency:  currency,
		CreatedAt: time.Now().UTC(),
	}
	_, err := r.DB.Pool.Exec(ctx,
		`INSERT INTO accounts (id, user_id, name, currency, created_at) VALUES ($1, $2, $3, $4, $5)`,
		a.ID, a.UserID, a.Name, a.Currency, a.CreatedAt,
	)
	if err != nil {
		return Account{}, err
	}
	return a, nil
}

func (r *PgAccountRepo) ListByUser(ctx context.Context, userID string) ([]Account, error) {
	rows, err := r.DB.Pool.Query(ctx,
		`SELECT id, user_id, name, currency, created_at FROM accounts WHERE user_id = $1 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Currency, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PgAccountRepo) GetByID(ctx context.Context, id string) (Account, error) {
	row := r.DB.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, currency, created_at FROM accounts WHERE id = $1`, id,
	)
	var a Account
	if err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.Currency, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, err
	}
	return a, nil
}
