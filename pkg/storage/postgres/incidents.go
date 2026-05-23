package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/PratikDhanave/multi-agent-reference-architecture-go/pkg/incidents"
)

// PgIncidentStore satisfies incidents.Store over pgxpool.
type PgIncidentStore struct{ DB *DB }

// NewIncidentStore constructs the repo.
func NewIncidentStore(db *DB) *PgIncidentStore { return &PgIncidentStore{DB: db} }

func (s *PgIncidentStore) Create(ctx context.Context, i incidents.Incident) (incidents.Incident, error) {
	if err := (&i).Validate(); err != nil {
		return incidents.Incident{}, err
	}
	var metaJSON []byte
	if len(i.Metadata) > 0 {
		var err error
		metaJSON, err = json.Marshal(i.Metadata)
		if err != nil {
			return incidents.Incident{}, err
		}
	}
	_, err := s.DB.Pool.Exec(ctx,
		`INSERT INTO incidents
		 (id, occurred_at, detected_at, use_case, model, third_party_vendor,
		  description, affected_stakeholders, severity, failure_mode,
		  root_cause, response_actions, status, actor_id, metadata)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		i.ID, i.OccurredAt, i.DetectedAt, i.UseCase, i.Model, i.ThirdPartyVendor,
		i.Description, i.AffectedStakeholders, string(i.Severity), string(i.FailureMode),
		i.RootCause, i.ResponseActions, string(i.Status), i.ActorID, metaJSON,
	)
	if err != nil {
		return incidents.Incident{}, err
	}
	return i, nil
}

func (s *PgIncidentStore) List(ctx context.Context, limit int) ([]incidents.Incident, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.Pool.Query(ctx,
		`SELECT id, occurred_at, detected_at, use_case, model, third_party_vendor,
		        description, affected_stakeholders, severity, failure_mode,
		        root_cause, response_actions, status, actor_id, metadata
		   FROM incidents ORDER BY occurred_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []incidents.Incident
	for rows.Next() {
		var i incidents.Incident
		var sev, mode, status string
		var meta []byte
		if err := rows.Scan(
			&i.ID, &i.OccurredAt, &i.DetectedAt, &i.UseCase, &i.Model, &i.ThirdPartyVendor,
			&i.Description, &i.AffectedStakeholders, &sev, &mode,
			&i.RootCause, &i.ResponseActions, &status, &i.ActorID, &meta,
		); err != nil {
			return nil, err
		}
		i.Severity = incidents.Severity(sev)
		i.FailureMode = incidents.FailureMode(mode)
		i.Status = incidents.Status(status)
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &i.Metadata)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (s *PgIncidentStore) CountByModeSince(ctx context.Context, mode incidents.FailureMode, since time.Time) (int, error) {
	var n int
	err := s.DB.Pool.QueryRow(ctx,
		`SELECT count(*) FROM incidents WHERE failure_mode = $1 AND occurred_at >= $2`,
		string(mode), since,
	).Scan(&n)
	return n, err
}
