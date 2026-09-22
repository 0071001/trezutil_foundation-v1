package ai

import (
	"fmt"
	"sync"

	"github.com/trezutil/trezutil/internal/domain"
)

type ActionCapability struct {
	Action          domain.CommerceActionType
	Category        domain.ActionCategory
	RequiresConfirm bool
	Specialists     []domain.AISpecialistCode
}

type CapabilityMatrix struct {
	mu           sync.RWMutex
	capabilities map[domain.CommerceActionType]ActionCapability
}

func NewCapabilityMatrix() *CapabilityMatrix {
	cm := &CapabilityMatrix{
		capabilities: make(map[domain.CommerceActionType]ActionCapability),
	}

	for _, spec := range domain.DefaultSpecialists {
		for _, action := range spec.OwnerAction {
			cm.addCapability(ActionCapability{
				Action:          action,
				Category:        action.GetCategory(),
				RequiresConfirm: domain.AIActionRequiresConfirmation(action),
				Specialists:     []domain.AISpecialistCode{spec.Code},
			})
		}
	}

	return cm
}

func (cm *CapabilityMatrix) addCapability(cap ActionCapability) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if existing, ok := cm.capabilities[cap.Action]; ok {
		existing.Specialists = appendUnique(existing.Specialists, cap.Specialists...)
		cm.capabilities[cap.Action] = existing
	} else {
		cm.capabilities[cap.Action] = cap
	}
}

func appendUnique(existing []domain.AISpecialistCode, items ...domain.AISpecialistCode) []domain.AISpecialistCode {
	seen := make(map[domain.AISpecialistCode]bool)
	for _, e := range existing {
		seen[e] = true
	}
	for _, item := range items {
		if !seen[item] {
			existing = append(existing, item)
			seen[item] = true
		}
	}
	return existing
}

func (cm *CapabilityMatrix) GetCapability(action domain.CommerceActionType) (ActionCapability, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cap, ok := cm.capabilities[action]
	return cap, ok
}

func (cm *CapabilityMatrix) OwnerForAction(action domain.CommerceActionType) (domain.AISpecialistCode, bool) {
	cap, ok := cm.GetCapability(action)
	if !ok {
		return domain.AISpecialistCommerce, true
	}
	if len(cap.Specialists) == 0 {
		return domain.AISpecialistCommerce, true
	}
	return cap.Specialists[0], true
}

func (cm *CapabilityMatrix) AllActionsForSpecialist(code domain.AISpecialistCode) []domain.CommerceActionType {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	actions := []domain.CommerceActionType{}
	for action, cap := range cm.capabilities {
		for _, s := range cap.Specialists {
			if s == code {
				actions = append(actions, action)
				break
			}
		}
	}
	return actions
}

func (cm *CapabilityMatrix) CanSpecialistHandle(code domain.AISpecialistCode, action domain.CommerceActionType) bool {
	cap, ok := cm.GetCapability(action)
	if !ok {
		return false
	}
	for _, s := range cap.Specialists {
		if s == code {
			return true
		}
	}
	return false
}

func (cm *CapabilityMatrix) RequiresConfirmation(action domain.CommerceActionType) bool {
	cap, ok := cm.GetCapability(action)
	if !ok {
		return domain.AIActionRequiresConfirmation(action)
	}
	return cap.RequiresConfirm
}

func (cm *CapabilityMatrix) Count() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.capabilities)
}

func (cm *CapabilityMatrix) String() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return fmt.Sprintf("CapabilityMatrix with %d actions", len(cm.capabilities))
}
