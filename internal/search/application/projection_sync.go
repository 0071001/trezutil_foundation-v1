package application

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// ListingChange is a listing row version observed by the sync loop.
type ListingChange struct {
	ID        string
	Status    domain.ListingStatus
	UpdatedAt time.Time
}

// ListingChangeSource lists listings ordered by (updated_at, id) strictly after
// the given keyset position. PostgreSQL is the source of truth.
type ListingChangeSource interface {
	ListChangedSince(ctx context.Context, afterUpdatedAt time.Time, afterID string, limit int) ([]ListingChange, error)
}

// ListingReindexer rebuilds and indexes one listing document.
type ListingReindexer interface {
	ReindexListing(ctx context.Context, listingID string) error
}

// ProjectionSync keeps the OpenSearch read projection consistent with
// PostgreSQL without requiring an event bus: it tails listings by
// (updated_at, id), indexes published ones and removes everything else.
// Indexing is idempotent, so re-scanning an overlap window is safe.
type ProjectionSync struct {
	source    ListingChangeSource
	reindexer ListingReindexer
	index     domain.SearchIndexPort
	logger    zerolog.Logger
	batch     int
	overlap   time.Duration

	cursorTS time.Time
	cursorID string
}

// NewProjectionSync creates a ProjectionSync starting from the beginning (a
// full resync on first run).
func NewProjectionSync(source ListingChangeSource, reindexer ListingReindexer, index domain.SearchIndexPort, logger zerolog.Logger) *ProjectionSync {
	return &ProjectionSync{
		source:    source,
		reindexer: reindexer,
		index:     index,
		logger:    logger,
		batch:     500,
		overlap:   5 * time.Second,
	}
}

// SyncOnce drains all pending changes and returns how many listings were
// projected. Individual listing failures are logged and retried on the next
// full resync; they do not block the rest of the backlog.
func (s *ProjectionSync) SyncOnce(ctx context.Context) (int, error) {
	processed := 0
	for {
		changes, err := s.source.ListChangedSince(ctx, s.cursorTS, s.cursorID, s.batch)
		if err != nil {
			return processed, err
		}
		for _, c := range changes {
			if err := s.apply(ctx, c); err != nil {
				s.logger.Warn().Err(err).Str("listing_id", c.ID).Msg("search projection update failed")
			} else {
				processed++
			}
			s.cursorTS, s.cursorID = c.UpdatedAt, c.ID
		}
		if len(changes) < s.batch {
			break
		}
	}
	// Rewind by a small window so rows rewritten within the same second as the
	// last observed change are picked up on the next pass.
	if !s.cursorTS.IsZero() {
		s.cursorTS, s.cursorID = s.cursorTS.Add(-s.overlap), ""
	}
	return processed, nil
}

// ResetCursor forces the next SyncOnce to rescan every listing.
func (s *ProjectionSync) ResetCursor() {
	s.cursorTS, s.cursorID = time.Time{}, ""
}

func (s *ProjectionSync) apply(ctx context.Context, c ListingChange) error {
	if c.Status == domain.ListingStatusPublished {
		return s.reindexer.ReindexListing(ctx, c.ID)
	}
	return s.index.DeleteListingDocument(ctx, c.ID)
}

// Run syncs every interval and performs a full resync every fullEvery
// (catching changes to related rows such as media or locations that do not
// touch listings.updated_at). It returns when ctx is cancelled.
func (s *ProjectionSync) Run(ctx context.Context, interval, fullEvery time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	lastFull := time.Now()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if n, err := s.SyncOnce(ctx); err != nil {
			s.logger.Warn().Err(err).Msg("search projection sync failed")
		} else if n > 0 {
			s.logger.Info().Int("listings", n).Msg("search projection synced")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if fullEvery > 0 && time.Since(lastFull) >= fullEvery {
			s.ResetCursor()
			lastFull = time.Now()
		}
	}
}
