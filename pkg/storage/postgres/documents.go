package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrDocumentNotFound is returned when a document lookup fails.
var ErrDocumentNotFound = errors.New("document not found")

// Document is the persisted form of an encrypted upload (e.g. a transactions CSV).
type Document struct {
	ID             string
	UserID         string
	AccountID      string
	Classification protocol.Classification
	Description    string
	Payload        crypto.EncryptedPayload
	CreatedAt      time.Time
}

// DocumentRepo is the data-access surface for encrypted documents.
type DocumentRepo interface {
	Create(ctx context.Context, d Document) (Document, error)
	GetByID(ctx context.Context, id string) (Document, error)
}

// PgDocumentRepo is the pgx-backed implementation.
type PgDocumentRepo struct{ DB *DB }

func NewDocumentRepo(db *DB) *PgDocumentRepo { return &PgDocumentRepo{DB: db} }

func (r *PgDocumentRepo) Create(ctx context.Context, d Document) (Document, error) {
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Classification == "" {
		d.Classification = protocol.ClassPII
	}
	payloadJSON, err := json.Marshal(d.Payload)
	if err != nil {
		return Document{}, err
	}
	var accountID any
	if d.AccountID != "" {
		accountID = d.AccountID
	}
	_, err = r.DB.Pool.Exec(ctx,
		`INSERT INTO documents (id, user_id, account_id, classification, description, payload, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, d.UserID, accountID, string(d.Classification), d.Description, payloadJSON, d.CreatedAt,
	)
	if err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *PgDocumentRepo) GetByID(ctx context.Context, id string) (Document, error) {
	row := r.DB.Pool.QueryRow(ctx,
		`SELECT id, user_id, COALESCE(account_id::text, ''), classification, description, payload, created_at
		   FROM documents WHERE id = $1`,
		id,
	)
	var d Document
	var classStr string
	var payloadJSON []byte
	if err := row.Scan(&d.ID, &d.UserID, &d.AccountID, &classStr, &d.Description, &payloadJSON, &d.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrDocumentNotFound
		}
		return Document{}, err
	}
	d.Classification = protocol.Classification(classStr)
	if err := json.Unmarshal(payloadJSON, &d.Payload); err != nil {
		return Document{}, err
	}
	return d, nil
}
