package lineage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ─── Recorder interface ────────────────────────────────────────────────────────

// Recorder defines the immutable lineage recording interface.
//
// All implementations must:
// - Block duplicates (same ID should not be recorded twice)
// - Maintain insertion order (entries ordered by timestamp)
// - Preserve the hash chain (each entry's hash depends on the previous)
// - Be thread-safe for concurrent Record calls
type Recorder interface {
	// Record appends an entry to the lineage.
	// The entry's Hash field is computed and set by the recorder.
	// Returns error if entry already exists or if recording fails.
	Record(ctx context.Context, entry *LineageEntry) error

	// Query retrieves entries matching the query filters.
	// Results are ordered by timestamp (ascending).
	Query(ctx context.Context, q LineageQuery) ([]*LineageEntry, error)

	// Verify checks the hash chain integrity from start to end.
	// Returns LineageIntegrityResult with details of any breaks.
	Verify(ctx context.Context) LineageIntegrityResult

	// Close releases resources (for PostgreSQL, closes connection pool).
	Close() error
}

// ─── InMemoryLineageRecorder ──────────────────────────────────────────────────

// InMemoryLineageRecorder stores entries in memory with hash chain integrity.
//
// Thread-safe via RWMutex. Designed for testing and local deployments.
// Does not persist across process restarts.
type InMemoryLineageRecorder struct {
	mu      sync.RWMutex
	entries []*LineageEntry
	byID    map[string]*LineageEntry // for duplicate detection
	lastIdx int                      // index of last recorded entry
}

// NewInMemoryRecorder constructs a new in-memory recorder.
func NewInMemoryRecorder() *InMemoryLineageRecorder {
	return &InMemoryLineageRecorder{
		entries: make([]*LineageEntry, 0),
		byID:    make(map[string]*LineageEntry),
		lastIdx: -1,
	}
}

// Record appends a new entry with hash chain integrity.
func (r *InMemoryLineageRecorder) Record(ctx context.Context, entry *LineageEntry) error {
	if entry == nil {
		return fmt.Errorf("lineage: entry cannot be nil")
	}
	if entry.ID == "" {
		return fmt.Errorf("lineage: entry ID cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check duplicate
	if _, exists := r.byID[entry.ID]; exists {
		return fmt.Errorf("lineage: entry %q already exists", entry.ID)
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Compute hash chain
	prevHash := ""
	if r.lastIdx >= 0 && r.lastIdx < len(r.entries) {
		prevHash = r.entries[r.lastIdx].Hash
	}
	entry.PrevHash = prevHash
	entry.Hash = entry.ComputeHash(prevHash)

	// Append entry
	r.entries = append(r.entries, entry)
	r.byID[entry.ID] = entry
	r.lastIdx = len(r.entries) - 1

	return nil
}

// Query retrieves entries matching the query filters.
func (r *InMemoryLineageRecorder) Query(ctx context.Context, q LineageQuery) ([]*LineageEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*LineageEntry

	for _, entry := range r.entries {
		// Apply filters
		if q.UserID != "" && entry.UserID != q.UserID {
			continue
		}
		if q.ResourceID != "" && entry.ResourceID != q.ResourceID {
			continue
		}
		if q.ResourceType != "" && entry.ResourceType != q.ResourceType {
			continue
		}
		if q.Action != "" && entry.Action != q.Action {
			continue
		}
		if q.Decision != "" && entry.Decision != q.Decision {
			continue
		}
		if !q.Since.IsZero() && entry.Timestamp.Before(q.Since) {
			continue
		}
		if !q.Until.IsZero() && entry.Timestamp.After(q.Until) {
			continue
		}

		results = append(results, entry)
	}

	// Apply offset and limit
	if q.Offset > 0 && q.Offset < len(results) {
		results = results[q.Offset:]
	} else if q.Offset > 0 {
		results = []*LineageEntry{}
	}

	if q.Limit > 0 && len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return results, nil
}

// Verify checks the hash chain integrity.
func (r *InMemoryLineageRecorder) Verify(ctx context.Context) LineageIntegrityResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := LineageIntegrityResult{
		Valid:        true,
		TotalEntries: len(r.entries),
	}

	if len(r.entries) == 0 {
		return result
	}

	// Check first entry's prevHash is empty
	if r.entries[0].PrevHash != "" {
		result.Valid = false
		result.BrokenAt = r.entries[0].ID
		result.Error = fmt.Errorf("lineage: first entry has non-empty prevHash")
		return result
	}

	// Verify hash chain
	prevHash := ""
	for _, entry := range r.entries {
		expectedHash := entry.ComputeHash(prevHash)
		if entry.Hash != expectedHash {
			result.Valid = false
			result.BrokenAt = entry.ID
			result.Error = fmt.Errorf("lineage: hash mismatch at entry %q: expected %s, got %s",
				entry.ID, expectedHash, entry.Hash)
			return result
		}
		prevHash = entry.Hash
	}

	return result
}

// Close is a no-op for in-memory recorder.
func (r *InMemoryLineageRecorder) Close() error {
	return nil
}

// ─── PostgreSQL Lineage Recorder ──────────────────────────────────────────────

// PostgreSQLLineageRecorder stores entries in a PostgreSQL database.
//
// Thread-safe via database connection pooling. Designed for production deployments
// where durability and query performance are required.
type PostgreSQLLineageRecorder struct {
	db *sql.DB
}

// NewPostgreSQLRecorder constructs a new PostgreSQL recorder.
// The database connection must already be created and opened.
//
// Call EnsureSchema to create the required tables.
func NewPostgreSQLRecorder(db *sql.DB) *PostgreSQLLineageRecorder {
	return &PostgreSQLLineageRecorder{db: db}
}

// EnsureSchema creates the lineage table if it doesn't exist.
func (r *PostgreSQLLineageRecorder) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS lineage_entries (
		id TEXT PRIMARY KEY,
		timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
		user_id TEXT NOT NULL,
		resource_id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		action TEXT NOT NULL,
		decision TEXT NOT NULL,
		reason_code TEXT NOT NULL,
		policy_rule TEXT,
		agent_id TEXT,
		session_id TEXT,
		trace_id TEXT,
		hash TEXT NOT NULL,
		prev_hash TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_lineage_user_id ON lineage_entries(user_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_lineage_resource_id ON lineage_entries(resource_id, timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_lineage_timestamp ON lineage_entries(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_lineage_decision ON lineage_entries(decision);
	CREATE INDEX IF NOT EXISTS idx_lineage_session_id ON lineage_entries(session_id);
	`

	_, err := r.db.ExecContext(ctx, query)
	return err
}

// Record appends a new entry to the database with hash chain integrity.
func (r *PostgreSQLLineageRecorder) Record(ctx context.Context, entry *LineageEntry) error {
	if entry == nil {
		return fmt.Errorf("lineage: entry cannot be nil")
	}
	if entry.ID == "" {
		return fmt.Errorf("lineage: entry ID cannot be empty")
	}

	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Get the previous entry's hash for the chain
	var prevHash sql.NullString
	query := `SELECT hash FROM lineage_entries ORDER BY timestamp DESC, id DESC LIMIT 1`
	err := r.db.QueryRowContext(ctx, query).Scan(&prevHash)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("lineage: failed to get previous hash: %w", err)
	}

	prevHashStr := ""
	if prevHash.Valid {
		prevHashStr = prevHash.String
	}

	entry.PrevHash = prevHashStr
	entry.Hash = entry.ComputeHash(prevHashStr)

	// Insert entry
	insertQuery := `
	INSERT INTO lineage_entries (
		id, timestamp, user_id, resource_id, resource_type, action, decision,
		reason_code, policy_rule, agent_id, session_id, trace_id, hash, prev_hash
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.ExecContext(ctx, insertQuery,
		entry.ID, entry.Timestamp, entry.UserID, entry.ResourceID, entry.ResourceType,
		string(entry.Action), string(entry.Decision), entry.ReasonCode, entry.PolicyRule,
		entry.AgentID, entry.SessionID, entry.TraceID, entry.Hash, entry.PrevHash,
	)

	if err != nil {
		return fmt.Errorf("lineage: failed to insert entry: %w", err)
	}

	return nil
}

// Query retrieves entries matching the query filters.
func (r *PostgreSQLLineageRecorder) Query(ctx context.Context, q LineageQuery) ([]*LineageEntry, error) {
	query := `
	SELECT id, timestamp, user_id, resource_id, resource_type, action, decision,
		   reason_code, policy_rule, agent_id, session_id, trace_id, hash, prev_hash
	FROM lineage_entries
	WHERE 1=1
	`
	var args []any

	if q.UserID != "" {
		query += ` AND user_id = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.UserID)
	}
	if q.ResourceID != "" {
		query += ` AND resource_id = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.ResourceID)
	}
	if q.ResourceType != "" {
		query += ` AND resource_type = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.ResourceType)
	}
	if q.Action != "" {
		query += ` AND action = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, string(q.Action))
	}
	if q.Decision != "" {
		query += ` AND decision = $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, string(q.Decision))
	}
	if !q.Since.IsZero() {
		query += ` AND timestamp >= $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.Since)
	}
	if !q.Until.IsZero() {
		query += ` AND timestamp < $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.Until)
	}

	query += ` ORDER BY timestamp ASC, id ASC`

	if q.Offset > 0 {
		query += ` OFFSET $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.Offset)
	}
	if q.Limit > 0 {
		query += ` LIMIT $` + fmt.Sprintf("%d", len(args)+1)
		args = append(args, q.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lineage: query failed: %w", err)
	}
	defer rows.Close()

	var results []*LineageEntry
	for rows.Next() {
		var entry LineageEntry
		var action, decision string
		err := rows.Scan(
			&entry.ID, &entry.Timestamp, &entry.UserID, &entry.ResourceID, &entry.ResourceType,
			&action, &decision, &entry.ReasonCode, &entry.PolicyRule,
			&entry.AgentID, &entry.SessionID, &entry.TraceID, &entry.Hash, &entry.PrevHash,
		)
		if err != nil {
			return nil, fmt.Errorf("lineage: scan failed: %w", err)
		}
		entry.Action = Action(action)
		entry.Decision = Decision(decision)
		results = append(results, &entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("lineage: row iteration failed: %w", err)
	}

	return results, nil
}

// Verify checks the hash chain integrity across the database.
func (r *PostgreSQLLineageRecorder) Verify(ctx context.Context) LineageIntegrityResult {
	query := `
	SELECT id, timestamp, user_id, resource_id, resource_type, action, decision,
		   reason_code, policy_rule, agent_id, session_id, trace_id, hash, prev_hash
	FROM lineage_entries
	ORDER BY timestamp ASC, id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return LineageIntegrityResult{
			Valid: false,
			Error: fmt.Errorf("lineage: verify query failed: %w", err),
		}
	}
	defer rows.Close()

	result := LineageIntegrityResult{Valid: true}
	prevHash := ""

	for rows.Next() {
		var entry LineageEntry
		var action, decision string
		err := rows.Scan(
			&entry.ID, &entry.Timestamp, &entry.UserID, &entry.ResourceID, &entry.ResourceType,
			&action, &decision, &entry.ReasonCode, &entry.PolicyRule,
			&entry.AgentID, &entry.SessionID, &entry.TraceID, &entry.Hash, &entry.PrevHash,
		)
		if err != nil {
			result.Valid = false
			result.Error = fmt.Errorf("lineage: scan failed: %w", err)
			return result
		}

		entry.Action = Action(action)
		entry.Decision = Decision(decision)
		result.TotalEntries++

		// Verify hash chain
		expectedHash := entry.ComputeHash(prevHash)
		if entry.Hash != expectedHash {
			result.Valid = false
			result.BrokenAt = entry.ID
			result.Error = fmt.Errorf("lineage: hash mismatch at entry %q", entry.ID)
			return result
		}

		prevHash = entry.Hash
	}

	if err = rows.Err(); err != nil {
		result.Valid = false
		result.Error = fmt.Errorf("lineage: row iteration failed: %w", err)
		return result
	}

	return result
}

// Close closes the database connection.
func (r *PostgreSQLLineageRecorder) Close() error {
	return r.db.Close()
}
