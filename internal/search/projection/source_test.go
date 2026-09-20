package projection

import (
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestComputeFreshnessScore(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name        string
		publishedAt int64
		now         int64
		wantMin     float64
		wantMax     float64
	}{
		{
			name:        "just published",
			publishedAt: now,
			now:         now,
			wantMin:     0.99,
			wantMax:     1.0,
		},
		{
			name:        "one week old",
			publishedAt: now - 7*24*60*60,
			now:         now,
			wantMin:     0.3,
			wantMax:     0.7,
		},
		{
			name:        "never published",
			publishedAt: 0,
			now:         now,
			wantMin:     0.0,
			wantMax:     0.0,
		},
		{
			name:        "future published",
			publishedAt: now + 86400,
			now:         now,
			wantMin:     0.99,
			wantMax:     1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := computeFreshnessScore(tt.publishedAt, tt.now)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("computeFreshnessScore() = %v, want between %v and %v", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestBuildCategoryPath(t *testing.T) {
	category := &domain.Category{
		ID:       "child-cat",
		ParentID: strPtr("parent-cat"),
	}

	path := buildCategoryPath(category)
	if len(path) != 2 {
		t.Errorf("expected category path length 2, got %d", len(path))
	}

	if path[0] != "parent-cat" {
		t.Errorf("expected first path item 'parent-cat', got '%s'", path[0])
	}

	if path[1] != "child-cat" {
		t.Errorf("expected second path item 'child-cat', got '%s'", path[1])
	}
}

func TestBuildCategoryPath_NoParent(t *testing.T) {
	category := &domain.Category{
		ID:       "root-cat",
		ParentID: nil,
	}

	path := buildCategoryPath(category)
	if len(path) != 1 {
		t.Errorf("expected category path length 1, got %d", len(path))
	}

	if path[0] != "root-cat" {
		t.Errorf("expected path item 'root-cat', got '%s'", path[0])
	}
}

func TestBuildMediaData(t *testing.T) {
	tests := []struct {
		name        string
		media       []domain.ListingMedia
		wantCount   int
		wantPrimary *string
	}{
		{
			name:        "no media",
			media:       []domain.ListingMedia{},
			wantCount:   0,
			wantPrimary: nil,
		},
		{
			name: "with primary media",
			media: []domain.ListingMedia{
				{URL: "https://example.com/1.jpg", Primary: false},
				{URL: "https://example.com/2.jpg", Primary: true},
			},
			wantCount:   2,
			wantPrimary: strPtr("https://example.com/2.jpg"),
		},
		{
			name: "no primary media",
			media: []domain.ListingMedia{
				{URL: "https://example.com/1.jpg", Primary: false},
			},
			wantCount:   1,
			wantPrimary: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, primary := buildMediaData(tt.media)
			if count != tt.wantCount {
				t.Errorf("buildMediaData() count = %d, want %d", count, tt.wantCount)
			}
			if primary == nil && tt.wantPrimary != nil {
				t.Errorf("buildMediaData() primary = nil, want %v", *tt.wantPrimary)
			}
			if primary != nil && tt.wantPrimary != nil && *primary != *tt.wantPrimary {
				t.Errorf("buildMediaData() primary = %v, want %v", *primary, *tt.wantPrimary)
			}
		})
	}
}

func TestBuildAttributeProjections(t *testing.T) {
	defs := []domain.AttributeDefinition{
		{ID: "def-1", Key: "brand", Type: domain.AttributeTypeText, Searchable: true},
		{ID: "def-2", Key: "price", Type: domain.AttributeTypeNumber, Searchable: true},
		{ID: "def-3", Key: "internal", Type: domain.AttributeTypeText, Searchable: false},
	}

	values := []domain.AttributeValue{
		{AttributeDefinitionID: "def-1", ValueText: strPtr("Apple")},
		{AttributeDefinitionID: "def-2", ValueNumber: float64Ptr(100.0)},
		{AttributeDefinitionID: "def-3", ValueText: strPtr("secret")},
	}

	projections := buildAttributeProjections(values, defs)

	if len(projections) != 2 {
		t.Errorf("expected 2 searchable projections, got %d", len(projections))
	}

	foundBrand := false
	foundPrice := false
	for _, p := range projections {
		if p.Key == "brand" {
			foundBrand = true
			if p.ValueText == nil || *p.ValueText != "Apple" {
				t.Errorf("expected brand value 'Apple', got %v", p.ValueText)
			}
		}
		if p.Key == "price" {
			foundPrice = true
			if p.ValueNumber == nil || *p.ValueNumber != 100.0 {
				t.Errorf("expected price value 100.0, got %v", p.ValueNumber)
			}
		}
	}

	if !foundBrand {
		t.Error("expected brand projection to be found")
	}
	if !foundPrice {
		t.Error("expected price projection to be found")
	}
}

func TestEmptyStringIfNil(t *testing.T) {
	if emptyStringIfNil(nil) != "" {
		t.Error("expected empty string for nil input")
	}

	s := "hello"
	if emptyStringIfNil(&s) != "hello" {
		t.Error("expected 'hello' for non-nil input")
	}
}

func float64Ptr(f float64) *float64 {
	return &f
}
