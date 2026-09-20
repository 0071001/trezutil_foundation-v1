package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
	"github.com/trezutil/trezutil/packages/events"
)

// EventProcessorOptions configures the EventProcessor.
type EventProcessorOptions func(*EventProcessor)

// WithMetrics sets the metrics collector.
func WithMetrics(metrics *Metrics) EventProcessorOptions {
	return func(ep *EventProcessor) {
		ep.metrics = metrics
	}
}

// WithDeadLetter sets the dead-letter handler.
func WithDeadLetter(dl *DeadLetter) EventProcessorOptions {
	return func(ep *EventProcessor) {
		ep.deadLetter = dl
	}
}

// SearchEventHandler handles search-relevant domain events.
type SearchEventHandler interface {
	HandleListingCreated(ctx context.Context, event events.Event) error
	HandleListingUpdated(ctx context.Context, event events.Event) error
	HandleListingPublished(ctx context.Context, event events.Event) error
	HandleListingPaused(ctx context.Context, event events.Event) error
	HandleListingSuspended(ctx context.Context, event events.Event) error
	HandleListingSold(ctx context.Context, event events.Event) error
	HandleListingClosed(ctx context.Context, event events.Event) error
	HandleListingExpired(ctx context.Context, event events.Event) error
	HandleListingAvailabilityChanged(ctx context.Context, event events.Event) error
	HandleListingLocationUpdated(ctx context.Context, event events.Event) error
	HandleCategoryUpdated(ctx context.Context, event events.Event) error
	HandleAttributeDefinitionUpdated(ctx context.Context, event events.Event) error
}

// EventProcessor processes search-relevant events.
type EventProcessor struct {
	projectionSource projection.ListingProjectionSource
	mapper           *projection.SearchListingDocumentMapper
	indexPort        domain.SearchIndexPort
	orderTracker     *EventOrderTracker
	processedEvents  map[string]bool
	deadLetter       *DeadLetter
	metrics          *Metrics
	maxRetries       int
}

// NewEventProcessor creates a new EventProcessor.
func NewEventProcessor(
	projectionSource projection.ListingProjectionSource,
	mapper *projection.SearchListingDocumentMapper,
	indexPort domain.SearchIndexPort,
	opts ...EventProcessorOptions,
) *EventProcessor {
	ep := &EventProcessor{
		projectionSource: projectionSource,
		mapper:           mapper,
		indexPort:        indexPort,
		orderTracker:     NewEventOrderTracker(),
		processedEvents:  make(map[string]bool),
		maxRetries:       3,
	}

	for _, opt := range opts {
		opt(ep)
	}

	return ep
}

// ProcessEvent processes a single search-relevant event.
func (p *EventProcessor) ProcessEvent(ctx context.Context, event events.Event) error {
	return p.processEventWithRetry(ctx, event, 0)
}

func (p *EventProcessor) processEventWithRetry(ctx context.Context, event events.Event, attempt int) error {
	start := time.Now()

	defer func() {
		latency := time.Since(start).Nanoseconds()
		if p.metrics != nil {
			p.metrics.RecordIndexRequest(true, latency)
		}
	}()

	if p.processedEvents[event.EventID] {
		return nil
	}

	if attempt == 0 && !p.orderTracker.CheckAndUpdate(event.ListingID, event.Version, event.EventID) {
		if p.metrics != nil {
			p.metrics.RecordProjectionEvent(true)
		}
		return nil
	}

	var err error
	switch event.EventName {
	case "ListingCreated":
		err = p.HandleListingCreated(ctx, event)
	case "ListingUpdated":
		err = p.HandleListingUpdated(ctx, event)
	case "ListingPublished":
		err = p.HandleListingPublished(ctx, event)
	case "ListingPaused":
		err = p.HandleListingPaused(ctx, event)
	case "ListingSuspended":
		err = p.HandleListingSuspended(ctx, event)
	case "ListingSold":
		err = p.HandleListingSold(ctx, event)
	case "ListingClosed":
		err = p.HandleListingClosed(ctx, event)
	case "ListingExpired":
		err = p.HandleListingExpired(ctx, event)
	case "ListingAvailabilityChanged":
		err = p.HandleListingAvailabilityChanged(ctx, event)
	case "ListingLocationUpdated":
		err = p.HandleListingLocationUpdated(ctx, event)
	case "CategoryUpdated":
		err = p.HandleCategoryUpdated(ctx, event)
	case "AttributeDefinitionUpdated":
		err = p.HandleAttributeDefinitionUpdated(ctx, event)
	default:
		return nil
	}

	if err != nil {
		if p.metrics != nil {
			p.metrics.RecordProjectionEvent(false)
		}

		if attempt < p.maxRetries {
			backoff := time.Duration(attempt+1) * 500 * time.Millisecond
			timer := time.NewTimer(backoff)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return p.processEventWithRetry(ctx, event, attempt+1)
			}
		}

		if p.deadLetter != nil {
			p.deadLetter.Add(ctx, event, err, attempt)
		}
		return fmt.Errorf("event %s failed after %d retries: %w", event.EventID, attempt, err)
	}

	p.processedEvents[event.EventID] = true
	if p.metrics != nil {
		p.metrics.RecordProjectionEvent(true)
	}

	return nil
}

// HandleListingCreated handles ListingCreated event.
// Action: Do not index yet. Wait for ListingPublished.
func (p *EventProcessor) HandleListingCreated(ctx context.Context, event events.Event) error {
	return nil
}

// HandleListingUpdated handles ListingUpdated event.
// Action: Reindex if published.
func (p *EventProcessor) HandleListingUpdated(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingUpdated")
	}

	proj, err := p.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	if !projection.IsSearchable(proj.Status) {
		return nil
	}

	doc, err := p.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if p.indexPort != nil {
		if err := p.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	return nil
}

// HandleListingPublished handles ListingPublished event.
// Action: Index or make visible in search.
func (p *EventProcessor) HandleListingPublished(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingPublished")
	}

	proj, err := p.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	doc, err := p.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if p.indexPort != nil {
		if err := p.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	return nil
}

// HandleListingPaused handles ListingPaused event.
// Action: Remove from public search. Keep in internal index.
func (p *EventProcessor) HandleListingPaused(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingPaused")
	}

	proj, err := p.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	doc, err := p.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if p.indexPort != nil {
		if err := p.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	return nil
}

// HandleListingSuspended handles ListingSuspended event.
// Action: Remove from all search.
func (p *EventProcessor) HandleListingSuspended(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingSuspended")
	}

	if p.indexPort != nil {
		if err := p.indexPort.DeleteListingDocument(ctx, listingID); err != nil {
			return fmt.Errorf("failed to delete document: %w", err)
		}
	}

	return nil
}

// HandleListingSold handles ListingSold event.
// Action: Remove from public search.
func (p *EventProcessor) HandleListingSold(ctx context.Context, event events.Event) error {
	return p.handleRemoveFromPublicSearch(ctx, event)
}

// HandleListingClosed handles ListingClosed event.
// Action: Remove from public search.
func (p *EventProcessor) HandleListingClosed(ctx context.Context, event events.Event) error {
	return p.handleRemoveFromPublicSearch(ctx, event)
}

// HandleListingExpired handles ListingExpired event.
// Action: Remove from public search.
func (p *EventProcessor) HandleListingExpired(ctx context.Context, event events.Event) error {
	return p.handleRemoveFromPublicSearch(ctx, event)
}

// HandleListingAvailabilityChanged handles ListingAvailabilityChanged event.
// Action: Update availability fields.
func (p *EventProcessor) HandleListingAvailabilityChanged(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingAvailabilityChanged")
	}

	proj, err := p.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	doc, err := p.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if p.indexPort != nil {
		if err := p.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	return nil
}

// HandleListingLocationUpdated handles ListingLocationUpdated event.
// Action: Reindex geo fields.
func (p *EventProcessor) HandleListingLocationUpdated(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required for ListingLocationUpdated")
	}

	proj, err := p.projectionSource.GetListingProjection(ctx, listingID)
	if err != nil {
		return fmt.Errorf("failed to load listing projection: %w", err)
	}

	doc, err := p.mapper.MapToDocument(proj)
	if err != nil {
		return fmt.Errorf("failed to map document: %w", err)
	}

	if p.indexPort != nil {
		if err := p.indexPort.IndexListingDocument(ctx, doc); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
	}

	return nil
}

// HandleCategoryUpdated handles CategoryUpdated event.
// Action: Reindex all listings in this category and its children.
func (p *EventProcessor) HandleCategoryUpdated(ctx context.Context, event events.Event) error {
	_, ok := event.Payload["category_id"].(string)
	if !ok {
		return fmt.Errorf("category_id required for CategoryUpdated")
	}

	return nil
}

// HandleAttributeDefinitionUpdated handles AttributeDefinitionUpdated event.
// Action: Reindex all listings using this attribute definition.
func (p *EventProcessor) HandleAttributeDefinitionUpdated(ctx context.Context, event events.Event) error {
	_, ok := event.Payload["attribute_definition_id"].(string)
	if !ok {
		return fmt.Errorf("attribute_definition_id required for AttributeDefinitionUpdated")
	}

	return nil
}

func (p *EventProcessor) handleRemoveFromPublicSearch(ctx context.Context, event events.Event) error {
	listingID := event.ListingID
	if listingID == "" {
		return fmt.Errorf("listing_id required")
	}

	if p.indexPort != nil {
		if err := p.indexPort.DeleteListingDocument(ctx, listingID); err != nil {
			return fmt.Errorf("failed to delete document: %w", err)
		}
	}

	return nil
}
