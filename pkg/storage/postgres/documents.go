package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/crypto"
	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/protocol"
)

// ErrDocumentNotFound is returned when a document lookup fails.
var ErrDocumentNotFound = errors.New("document not found")

// Document is the persisted form of an encrypted upload (e.g. a transactions CSV
// or an Aadhaar offline-KYC XML).
type Document struct {
	ID             string
	UserID         string
	AccountID      string
	DocType        string // kyc.DocType — "aadhaar_offline_kyc", "pan", "csv", ...
	Classification protocol.Classification
	Description    string
	Payload        crypto.EncryptedPayload
	// MaskedMeta holds only non-sensitive derived values (e.g. {"aadhaar_last4":
	// "2346"}). It MUST NOT contain a full Aadhaar number or any secret payload.
	MaskedMeta map[string]string
	CreatedAt  time.Time
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
	if d.DocType == "" {
		d.DocType = "other"
	}
	if d.MaskedMeta == nil {
		d.MaskedMeta = map[string]string{}
	}
	payloadJSON, err := json.Marshal(d.Payload)
	if err != nil {
		return Document{}, err
	}
	maskedJSON, err := json.Marshal(d.MaskedMeta)
	if err != nil {
		return Document{}, err
	}
	var accountID any
	if d.AccountID != "" {
		accountID = d.AccountID
	}
	_, err = r.DB.Pool.Exec(ctx,
		`INSERT INTO documents (id, user_id, account_id, doc_type, classification, description, payload, masked_meta, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		d.ID, d.UserID, accountID, d.DocType, string(d.Classification), d.Description, payloadJSON, maskedJSON, d.CreatedAt,
	)
	if err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *PgDocumentRepo) GetByID(ctx context.Context, id string) (Document, error) {
	row := r.DB.Pool.QueryRow(ctx,
		`SELECT id, user_id, COALESCE(account_id::text, ''), doc_type, classification, description, payload, masked_meta, created_at
		   FROM documents WHERE id = $1`,
		id,
	)
	var d Document
	var classStr string
	var payloadJSON, maskedJSON []byte
	if err := row.Scan(&d.ID, &d.UserID, &d.AccountID, &d.DocType, &classStr, &d.Description, &payloadJSON, &maskedJSON, &d.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Document{}, ErrDocumentNotFound
		}
		return Document{}, err
	}
	d.Classification = protocol.Classification(classStr)
	if err := json.Unmarshal(payloadJSON, &d.Payload); err != nil {
		return Document{}, err
	}
	if len(maskedJSON) > 0 {
		if err := json.Unmarshal(maskedJSON, &d.MaskedMeta); err != nil {
			return Document{}, err
		}
	}
	return d, nil
}
