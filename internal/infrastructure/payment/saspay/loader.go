package saspay

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

// ProviderLister lists configured providers by code across organisations.
type ProviderLister interface {
	FindAllByCode(ctx context.Context, code string) ([]*domain.IntegrationProvider, error)
}

func resolveSecret(ref string) string {
	if name, ok := strings.CutPrefix(ref, "env:"); ok && name != "" {
		return os.Getenv(name)
	}
	return ""
}

// Loader registers a SasPay adapter for every providers row with code "saspay" whose secret resolves.
//
// Provider row contract (providers table; scripts/ops/set-saspay-key.sh creates Trezutil's):
//
//	code          = 'saspay'
//	status        = 'ACTIVE', capabilities include 'PAYMENT'
//	secret_ref    = 'env:SASPAY_SECRET_KEY'
//	configuration = {"settings": {
//	    "scope": "platform",                         (serves every organisation without its own provider)
//	    "webhook_secret_ref": "env:SASPAY_WEBHOOK_SECRET",
//	    "return_url": "https://trezutil.com/",       (card payments come back here)
//	    "base_url": "https://api.saspay.me/api/v1",  (optional; sandbox or tests)
//	    "country": "CD"
//	}}
type Loader struct {
	Providers ProviderLister
	Registry  *provider.ProviderRegistry
	Logger    zerolog.Logger

	mu             sync.RWMutex
	webhookSecrets map[string]string // orgID -> signing secret
}

// Load registers adapters for providers not yet registered and returns how many are usable.
func (l *Loader) Load(ctx context.Context) (int, error) {
	rows, err := l.Providers.FindAllByCode(ctx, ProviderCode)
	if err != nil {
		return 0, err
	}
	secrets := map[string]string{}
	active := 0
	for _, p := range rows {
		settings := p.Configuration.Settings
		if ws := resolveSecret(settings["webhook_secret_ref"]); ws != "" {
			secrets[p.OrganizationID] = ws
		}
		key := resolveSecret(p.SecretRef)
		if key == "" {
			l.Logger.Warn().Str("provider_id", p.ID).Msg("saspay provider configured but its secret_ref does not resolve; skipping")
			continue
		}
		adapter, err := NewAdapter(Config{
			ProviderID: p.ID,
			SecretKey:  key,
			BaseURL:    settings["base_url"],
			Country:    settings["country"],
			ReturnURL:  settings["return_url"],
		})
		if err != nil {
			return active, fmt.Errorf("saspay provider %s: %w", p.ID, err)
		}
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

// Run reloads providers periodically.
func (l *Loader) Run(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := l.Load(ctx); err != nil {
				l.Logger.Warn().Err(err).Msg("saspay provider reload failed")
			}
		}
	}
}

// WebhookSecret returns the signing secret of an organisation's SasPay webhook, falling back to SASPAY_WEBHOOK_SECRET.
func (l *Loader) WebhookSecret(orgID string) string {
	l.mu.RLock()
	s := l.webhookSecrets[orgID]
	l.mu.RUnlock()
	if s != "" {
		return s
	}
	return os.Getenv("SASPAY_WEBHOOK_SECRET")
}
