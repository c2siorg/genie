package incidents

import (
	"context"
	"time"
)

// Liability records the report's graded-liability classification (Rec 8).
// "First offense" of a given failure mode within the lookback window is a
// warning; subsequent ones within the same window escalate the severity.
type Liability struct {
	IsFirstOffense bool
	RecentCount    int
	WindowDays     int
}

// Grade returns the liability classification for a fresh incident.
//
// Default window: 30 days, aligned with the report's "first time / one-off
// aberration" phrasing in para 4.4.30. Stores can pass a custom window.
func Grade(ctx context.Context, store Store, mode FailureMode, windowDays int) (Liability, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	since := time.Now().UTC().AddDate(0, 0, -windowDays)
	n, err := store.CountByModeSince(ctx, mode, since)
	if err != nil {
		return Liability{}, err
	}
	return Liability{
		IsFirstOffense: n == 0,
		RecentCount:    n,
		WindowDays:     windowDays,
	}, nil
}
