package vertical

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type PackRegistryConfig struct {
	Logger zerolog.Logger
}

type PackRegistry struct {
	mu     sync.RWMutex
	logger zerolog.Logger
	packs  map[domain.PackCode]*domain.VerticalPack
}

func NewPackRegistry(config PackRegistryConfig) *PackRegistry {
	if config.Logger.GetLevel() == 0 {
		config.Logger = zerolog.Nop()
	}
	return &PackRegistry{
		logger: config.Logger,
		packs:  make(map[domain.PackCode]*domain.VerticalPack),
	}
}

func (r *PackRegistry) Register(pack *domain.VerticalPack) error {
	if pack == nil {
		return fmt.Errorf("pack cannot be nil")
	}
	if err := pack.Validate(); err != nil {
		return fmt.Errorf("invalid pack: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packs[pack.Code]; exists {
		return fmt.Errorf("pack %s already registered", pack.Code)
	}

	r.packs[pack.Code] = pack
	r.logger.Info().
		Str("pack_code", string(pack.Code)).
		Str("pack_name", pack.Name).
		Msg("Registered vertical pack")
	return nil
}

func (r *PackRegistry) Get(code domain.PackCode) (*domain.VerticalPack, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pack, ok := r.packs[code]
	return pack, ok
}

func (r *PackRegistry) GetAll() []*domain.VerticalPack {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.VerticalPack, 0, len(r.packs))
	for _, pack := range r.packs {
		result = append(result, pack)
	}
	return result
}

func (r *PackRegistry) FindByAction(action domain.CommerceActionType) []domain.PackCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.PackCode
	for code, pack := range r.packs {
		if pack.SupportsAction(action) {
			result = append(result, code)
		}
	}
	return result
}

func (r *PackRegistry) FindByCapability(cap domain.PackCapability) []domain.PackCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.PackCode
	for code, pack := range r.packs {
		if pack.HasCapability(cap) {
			result = append(result, code)
		}
	}
	return result
}

func (r *PackRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.packs)
}
