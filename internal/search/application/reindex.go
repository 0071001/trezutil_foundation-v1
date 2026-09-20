package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

// ReindexJob represents a reindexing job.
type ReindexJob struct {
	projectionSource projection.ListingProjectionSource
	mapper           *projection.SearchListingDocumentMapper
	indexPort        domain.SearchIndexPort
	batchSize        int
	metrics          *Metrics
}

// ReindexJobProgress tracks reindexing progress.
type ReindexJobProgress struct {
	Processed int
	Total     int
	Errors    []string
}

// ReindexJobOptions configures the ReindexJob.
type ReindexJobOptions func(*ReindexJob)

// WithReindexMetrics sets the metrics collector for reindexing.
func WithReindexMetrics(metrics *Metrics) ReindexJobOptions {
	return func(job *ReindexJob) {
		job.metrics = metrics
	}
}

// NewReindexJob creates a new ReindexJob.
func NewReindexJob(
	projectionSource projection.ListingProjectionSource,
	mapper *projection.SearchListingDocumentMapper,
	indexPort domain.SearchIndexPort,
	batchSize int,
	opts ...ReindexJobOptions,
) *ReindexJob {
	if batchSize <= 0 {
		batchSize = 1000
	}

	job := &ReindexJob{
		projectionSource: projectionSource,
		mapper:           mapper,
		indexPort:        indexPort,
		batchSize:        batchSize,
	}

	for _, opt := range opts {
		opt(job)
	}

	return job
}

// Start begins the reindexing process.
func (j *ReindexJob) Start(ctx context.Context) (*ReindexJobProgress, error) {
	progress := &ReindexJobProgress{}

	listingIDs, err := j.projectionSource.GetPublishedListingIDs(ctx, "", j.batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get published listing IDs: %w", err)
	}

	progress.Total = len(listingIDs)

	for i := 0; i < len(listingIDs); i += j.batchSize {
		end := i + j.batchSize
		if end > len(listingIDs) {
			end = len(listingIDs)
		}

		batch := listingIDs[i:end]
		for _, listingID := range batch {
			if err := j.ReindexListing(ctx, listingID); err != nil {
				progress.Errors = append(progress.Errors, fmt.Sprintf("listing %s: %v", listingID, err))
			} else {
				progress.Processed++
			}
		}
	}

	if j.metrics != nil {
		j.metrics.RecordReindex(0, progress.Processed, len(progress.Errors))
	}

	if len(progress.Errors) > 0 {
		return progress, fmt.Errorf("reindex completed with %d errors", len(progress.Errors))
	}

	return progress, nil
}

// StartFromListingID begins reindexing from a specific listing ID.
func (j *ReindexJob) StartFromListingID(ctx context.Context, listingID string) (*ReindexJobProgress, error) {
	if listingID == "" {
		return nil, fmt.Errorf("listing_id required")
	}

	progress := &ReindexJobProgress{}

	if err := j.ReindexListing(ctx, listingID); err != nil {
		progress.Errors = append(progress.Errors, err.Error())
		return progress, nil
	}

	progress.Processed = 1
	progress.Total = 1

	return progress, nil
}

// ReindexListing rebuilds a single listing document.
func (j *ReindexJob) ReindexListing(ctx context.Context, listingID string) error {
	start := time.Now()

	proj, err := j.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	doc, err := j.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if j.indexPort != nil {
		if err := j.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	if j.metrics != nil {
		duration := time.Since(start).Nanoseconds()
		j.metrics.RecordIndexRequest(true, duration)
	}

	return nil
}
