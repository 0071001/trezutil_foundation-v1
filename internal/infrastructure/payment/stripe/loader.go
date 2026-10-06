package stripe

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
)

// ProviderLister lists configured providers by code across organizations.
type ProviderLister interface {
	FindAllByCode(ctx context.Context, code string) ([]*domain.IntegrationProvider, error)
}

// ResolveSecret resolves a provider secret reference. Only environment
// references ("env:NAME") are supported, so secrets never live in the database.
func ResolveSecret(ref string) string {
	if name, ok := strings.CutPrefix(ref, "env:"); ok && name != "" {
		return os.Getenv(name)
	}
	return ""
}

// Loader registers a Stripe adapter for every organization provider row with
// code "stripe" whose secret reference resolves.
//
// Provider row contract (providers table):
//
//	code          = 'stripe'
//	status        = 'ACTIVE', capabilities include 'PAYMENT'
//	secret_ref    = 'env:<VAR>'   -> secret API key (sk_…)
//	configuration = {"settings": {
//	    "publishable_key": "pk_…",                 (public)
//	    "capture_method": "automatic" | "manual",
//	    "webhook_secret_ref": "env:<VAR>",         -> endpoint secret (whsec_…)
//	    "base_url": "https://api.stripe.com"       (optional; tests)
//	}}
type Loader struct {
	Providers ProviderLister
	Registry  *provider.ProviderRegistry
	Logger    zerolog.Logger

	mu             sync.RWMutex
	webhookSecrets map[string]string // orgID -> whsec
}

// Load registers adapters for providers not yet registered and returns how
// many are active.
func (l *Loader) Load(ctx context.Context) (int, error) {
	rows, err := l.Providers.FindAllByCode(ctx, ProviderCode)
	if err != nil {
		return 0, err
	}
	secrets := map[string]string{}
	active := 0
	for _, p := range rows {
		settings := p.Configuration.Settings
		if ws := ResolveSecret(settings["webhook_secret_ref"]); ws != "" {
			secrets[p.OrganizationID] = ws
		}
		key := ResolveSecret(p.SecretRef)
		if key == "" {
			l.Logger.Warn().Str("provider_id", p.ID).Str("organization_id", p.OrganizationID).
				Msg("stripe provider configured but its secret_ref does not resolve; skipping")
			continue
		}
		adapter, err := NewAdapter(Config{
			ProviderID:     p.ID,
			SecretKey:      key,
			PublishableKey: settings["publishable_key"],
			CaptureMethod:  settings["capture_method"],
			BaseURL:        settings["base_url"],
		})
		if err != nil {
			return active, fmt.Errorf("stripe provider %s: %w", p.ID, err)
		}
		// A provider already registered keeps its adapter but takes the current record, so a status change
		// (enabled or disabled from the console) applies at this reload.
		if !l.Registry.UpdateProvider(p) {
			if err := l.Registry.Register(p, adapter); err != nil && !strings.Contains(err.Error(), "already registered") {
				return active, err
			}
		}
		if p.IsUsable() {
			active++
		}
	}
	l.mu.Lock()
	l.webhookSecrets = secrets
	l.mu.Unlock()
	return active, nil
}

// Run reloads providers periodically so newly configured organizations are
// picked up without a restart.
func (l *Loader) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := l.Load(ctx); err != nil {
				l.Logger.Warn().Err(err).Msg("stripe provider reload failed")
			}
		}
	}
}

// WebhookSecret returns the webhook signing secret for an organization,
// falling back to STRIPE_WEBHOOK_SECRET for single-account deployments.
func (l *Loader) WebhookSecret(orgID string) string {
	l.mu.RLock()
	s := l.webhookSecrets[orgID]
	l.mu.RUnlock()
	if s != "" {
		return s
	}
	return os.Getenv("STRIPE_WEBHOOK_SECRET")
}
