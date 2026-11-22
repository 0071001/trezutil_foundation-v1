// Package failover composes AI providers into one: a request goes to the first
// provider and, if that call fails (no credit, rate limit, outage, rejected
// request), to the next. If every provider fails the last error is returned
// and the conversation falls back to keyword routing.
package failover

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
)

// ID is the router identifier of a composed provider.
const ID = "failover"

// Provider is a domain.AIProvider backed by an ordered list of providers.
type Provider struct {
	providers []domain.AIProvider
	logger    zerolog.Logger
}

// New composes the providers in priority order. With a single provider it
// still works, so callers need not special-case it.
func New(logger zerolog.Logger, providers ...domain.AIProvider) (*Provider, error) {
	if len(providers) == 0 {
		return nil, errors.New("failover: no provider configured")
	}
	return &Provider{providers: providers, logger: logger}, nil
}

func (p *Provider) ID() string { return ID }

func (p *Provider) Name() string {
	names := make([]string, 0, len(p.providers))
	for _, pr := range p.providers {
		names = append(names, pr.Name())
	}
	return strings.Join(names, " → ")
}

func (p *Provider) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }

func (p *Provider) IsUsable() bool {
	for _, pr := range p.providers {
		if pr.IsUsable() {
			return true
		}
	}
	return false
}

// Models advertises the first provider's model so the router sees a capable
// model; each provider is addressed with its own model in Generate.
func (p *Provider) Models() []domain.AIModel {
	models := p.providers[0].Models()
	for i := range models {
		models[i].ProviderID = ID
	}
	return models
}

// Generate tries each usable provider in order.
func (p *Provider) Generate(ctx context.Context, req domain.AIRequest) (domain.AIResponse, error) {
	var last error
	for i, pr := range p.providers {
		if !pr.IsUsable() {
			continue
		}
		models := pr.Models()
		if len(models) == 0 {
			continue
		}
		attempt := req
		attempt.ModelID = models[0].ModelID
		resp, err := pr.Generate(ctx, attempt)
		if err == nil {
			if i > 0 {
				p.logger.Warn().Str("provider", pr.ID()).Msg("AI request answered by a fallback provider")
			}
			return resp, nil
		}
		last = err
		p.logger.Warn().Err(err).Str("provider", pr.ID()).Msg("AI provider failed; trying the next one")
		if ctx.Err() != nil {
			break
		}
	}
	if last == nil {
		last = domain.NewAIError(domain.AIErrProviderUnavailable, "no AI provider is usable")
	}
	return domain.AIResponse{}, last
}

// GenerateStream tries each usable streaming provider in order, like Generate, but only until one has sent text:
// text already shown cannot be taken back, so a failure after the first piece is returned, never retried elsewhere.
func (p *Provider) GenerateStream(ctx context.Context, req domain.AIRequest, onText func(string)) (domain.AIResponse, error) {
	var last error
	for i, pr := range p.providers {
		streamer, ok := pr.(domain.AITextStreamer)
		if !ok || !pr.IsUsable() {
			continue
		}
		models := pr.Models()
		if len(models) == 0 {
			continue
		}
		attempt := req
		attempt.ModelID = models[0].ModelID
		sent := false
		resp, err := streamer.GenerateStream(ctx, attempt, func(piece string) {
			sent = true
			if onText != nil {
				onText(piece)
			}
		})
		if err == nil {
			if i > 0 {
				p.logger.Warn().Str("provider", pr.ID()).Msg("AI stream answered by a fallback provider")
			}
			return resp, nil
		}
		last = err
		if sent || ctx.Err() != nil {
			break
		}
		p.logger.Warn().Err(err).Str("provider", pr.ID()).Msg("AI provider failed to stream; trying the next one")
	}
	if last == nil {
		last = domain.NewAIError(domain.AIErrProviderUnavailable, "no AI provider can stream")
	}
	return domain.AIResponse{}, last
}

var (
	_ domain.AIProvider     = (*Provider)(nil)
	_ domain.AITextStreamer = (*Provider)(nil)
)
