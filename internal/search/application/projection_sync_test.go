package application

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type memChangeSource struct{ rows []ListingChange }

func (m *memChangeSource) ListChangedSince(ctx context.Context, ts time.Time, id string, limit int) ([]ListingChange, error) {
	sort.Slice(m.rows, func(i, j int) bool {
		if m.rows[i].UpdatedAt.Equal(m.rows[j].UpdatedAt) {
			return m.rows[i].ID < m.rows[j].ID
		}
		return m.rows[i].UpdatedAt.Before(m.rows[j].UpdatedAt)
	})
	var out []ListingChange
	for _, r := range m.rows {
		if r.UpdatedAt.After(ts) || (r.UpdatedAt.Equal(ts) && r.ID > id) {
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

type memIndex struct {
	indexed map[string]int
	deleted map[string]int
}

func (m *memIndex) ReindexListing(ctx context.Context, id string) error { m.indexed[id]++; return nil }
func (m *memIndex) IndexListingDocument(ctx context.Context, doc interface{}) error {
	return nil
}
func (m *memIndex) DeleteListingDocument(ctx context.Context, id string) error {
	m.deleted[id]++
	return nil
}

func TestProjectionSync_IndexesPublishedAndRemovesOthers(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	src := &memChangeSource{rows: []ListingChange{
		{ID: "a", Status: domain.ListingStatusPublished, UpdatedAt: t0},
		{ID: "b", Status: domain.ListingStatusDraft, UpdatedAt: t0},
		{ID: "c", Status: domain.ListingStatusPaused, UpdatedAt: t0.Add(time.Second)},
	}}
	idx := &memIndex{indexed: map[string]int{}, deleted: map[string]int{}}
	s := NewProjectionSync(src, idx, idx, zerolog.Nop())
	s.batch = 2 // force keyset pagination

	n, err := s.SyncOnce(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("expected 3 processed, got %d (%v)", n, err)
	}
	if idx.indexed["a"] != 1 || idx.deleted["b"] != 1 || idx.deleted["c"] != 1 {
		t.Fatalf("unexpected projection: indexed=%v deleted=%v", idx.indexed, idx.deleted)
	}

	// A listing published later (even within the overlap window) is picked up.
	src.rows = append(src.rows, ListingChange{ID: "c", Status: domain.ListingStatusPublished, UpdatedAt: t0.Add(time.Second)})
	src.rows = src.rows[1:] // old "a" unchanged; replace paused "c" row
	for i := range src.rows {
		if src.rows[i].ID == "c" && src.rows[i].Status == domain.ListingStatusPaused {
			src.rows = append(src.rows[:i], src.rows[i+1:]...)
			break
		}
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if idx.indexed["c"] != 1 {
		t.Fatalf("expected republished listing c to be indexed, got %v", idx.indexed)
	}
}
