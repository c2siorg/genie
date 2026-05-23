package postgres

import (
	"context"
	"time"
)

// PurgeExpired deletes documents and MCP tokens whose expires_at is in the
// past. Returns rows-deleted counts so the caller can log them.
//
// Implements the Annexure V "retention_days" obligation and Recommendation
// 15 (Data Lifecycle Governance) from the FREE-AI report.
func (db *DB) PurgeExpired(ctx context.Context) (docsDeleted, tokensDeleted int64, err error) {
	now := time.Now().UTC()
	d, err := db.Pool.Exec(ctx, `DELETE FROM documents WHERE expires_at IS NOT NULL AND expires_at < $1`, now)
	if err != nil {
		return 0, 0, err
	}
	t, err := db.Pool.Exec(ctx, `DELETE FROM mcp_tokens WHERE expires_at IS NOT NULL AND expires_at < $1`, now)
	if err != nil {
		return d.RowsAffected(), 0, err
	}
	return d.RowsAffected(), t.RowsAffected(), nil
}

// StartRetentionJob runs PurgeExpired on a recurring tick until ctx is done.
// Suitable for cmd/api to launch as a background goroutine. The first run
// happens immediately so retention takes effect on boot.
func (db *DB) StartRetentionJob(ctx context.Context, interval time.Duration, log func(string, ...any)) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	go func() {
		t := time.NewTimer(0)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				d, k, err := db.PurgeExpired(ctx)
				if log != nil {
					if err != nil {
						log("retention.purge error: %v", err)
					} else if d > 0 || k > 0 {
						log("retention.purge documents=%d mcp_tokens=%d", d, k)
					}
				}
				t.Reset(interval)
			}
		}
	}()
}
