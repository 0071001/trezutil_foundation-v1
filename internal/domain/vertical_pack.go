package domain

import (
	"fmt"
	"time"
)

type PackCode string

const (
	PackECommerce PackCode = "ECOMMERCE"
	PackBooking   PackCode = "BOOKING"
	PackServices  PackCode = "SERVICES"
)

type PackStatus string

const (
	PackStatusActive     PackStatus = "ACTIVE"
	PackStatusInactive   PackStatus = "INACTIVE"
	PackStatusDeprecated PackStatus = "DEPRECATED"
)

type PackCapability = Capability

const (
	CapabilitySearch          PackCapability = "SEARCH"
	CapabilityOrdering        PackCapability = "ORDERING"
	CapabilityInventory       PackCapability = "INVENTORY"
	CapabilityCapacity        PackCapability = "CAPACITY"
	CapabilityScheduling      PackCapability = "SCHEDULING"
	CapabilityCustomerSupport PackCapability = "CUSTOMER_SUPPORT"
	CapabilityMerchantOps     PackCapability = "MERCHANT_OPERATIONS"
)

type PackVersion struct {
	Major int
	Minor int
	Patch int
}

type VerticalPack struct {
	ID               string
	Code             PackCode
	Name             string
	Description      string
	Version          PackVersion
	Status           PackStatus
	Capabilities     []PackCapability
	SupportedActions []CommerceActionType
	Policies         []PackPolicy
	Workflow         *PackWorkflow
	UIMetadata       PackUIMetadata
	AIProfile        AISpecialistCode
	CreatedAt        int64
	UpdatedAt        int64
}

type PackPolicy struct {
	Key   string
	Value string
}

type PackWorkflowStep struct {
	Name        string
	Description string
	Action      CommerceActionType
	Required    bool
	NextSteps   []string
}

type PackWorkflow struct {
	Name      string
	Steps     []PackWorkflowStep
	EntryStep string
}

func (w *PackWorkflow) StepsByAction() map[CommerceActionType][]PackWorkflowStep {
	result := make(map[CommerceActionType][]PackWorkflowStep)
	for _, step := range w.Steps {
		if step.Action != "" {
			result[step.Action] = append(result[step.Action], step)
		}
	}
	return result
}

func (w *PackWorkflow) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if w.EntryStep == "" {
		return fmt.Errorf("workflow entry step is required")
	}
	stepNames := make(map[string]bool)
	for _, step := range w.Steps {
		stepNames[step.Name] = true
	}
	if !stepNames[w.EntryStep] {
		return fmt.Errorf("entry step %s not found in workflow steps", w.EntryStep)
	}
	return nil
}

func (w *PackWorkflow) ValidTransitions(from, to string) bool {
	for _, step := range w.Steps {
		if step.Name == from {
			for _, next := range step.NextSteps {
				if next == to {
					return true
				}
			}
			return false
		}
	}
	return false
}

type PackUIMetadata struct {
	RoutePrefix string
	Icon        string
	Color       string
	Order       int
}

type OrganizationPack struct {
	OrganizationID  string
	PackCode        PackCode
	Enabled         bool
	ActivatedAt     int64
	ConfigOverrides map[string]string
}

func NewVerticalPack(code PackCode, name string, capabilities []PackCapability, specialist AISpecialistCode) *VerticalPack {
	now := time.Now().Unix()
	return &VerticalPack{
		ID:           string(code),
		Code:         code,
		Name:         name,
		Description:  fmt.Sprintf("%s vertical pack for Trezutil", name),
		Version:      PackVersion{Major: 1, Minor: 0, Patch: 0},
		Status:       PackStatusActive,
		Capabilities: capabilities,
		Policies:     []PackPolicy{},
		Workflow:     nil,
		UIMetadata:   PackUIMetadata{},
		AIProfile:    specialist,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (p *VerticalPack) SupportsAction(action CommerceActionType) bool {
	for _, a := range p.SupportedActions {
		if a == action {
			return true
		}
	}
	return false
}

func (p *VerticalPack) HasCapability(cap PackCapability) bool {
	for _, c := range p.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (p *VerticalPack) CanTransition() bool {
	return p.Workflow != nil
}

func (p *VerticalPack) Validate() error {
	if p.Code == "" {
		return fmt.Errorf("pack code is required")
	}
	if p.Name == "" {
		return fmt.Errorf("pack name is required")
	}
	if p.Status != PackStatusActive && p.Status != PackStatusInactive && p.Status != PackStatusDeprecated {
		return fmt.Errorf("invalid pack status: %s", p.Status)
	}
	if len(p.Capabilities) == 0 {
		return fmt.Errorf("pack must have at least one capability")
	}
	if len(p.SupportedActions) == 0 {
		return fmt.Errorf("pack must support at least one action")
	}
	if p.AIProfile == "" {
		return fmt.Errorf("pack must specify an AI profile")
	}
	if p.Workflow != nil {
		if err := p.Workflow.Validate(); err != nil {
			return fmt.Errorf("invalid workflow: %w", err)
		}
	}
	return nil
}

func (p *VerticalPack) IsActive() bool {
	return p.Status == PackStatusActive
}

type VerticalPackRepository interface {
	Create(ctx interface{}, pack *VerticalPack) error
	FindByCode(ctx interface{}, code PackCode) (*VerticalPack, error)
	FindAll(ctx interface{}) ([]*VerticalPack, error)
	FindByCapability(ctx interface{}, cap PackCapability) ([]*VerticalPack, error)
	Update(ctx interface{}, pack *VerticalPack) error
	Delete(ctx interface{}, id string) error
}

type OrganizationPackRepository interface {
	Create(ctx interface{}, orgPack *OrganizationPack) error
	FindByOrgAndPack(ctx interface{}, orgID string, code PackCode) (*OrganizationPack, error)
	ListByOrg(ctx interface{}, orgID string) ([]OrganizationPack, error)
	Enable(ctx interface{}, orgID string, packCode PackCode) error
	Disable(ctx interface{}, orgID string, packCode PackCode) error
	IsPackEnabled(ctx interface{}, orgID string, packCode PackCode) (bool, error)
}

type PackResolver interface {
	ResolvePackForAction(orgID string, action CommerceActionType) (PackCode, bool)
	ResolveSpecialistForAction(action CommerceActionType) (AISpecialistCode, bool)
	GetAvailablePacks(orgID string) []PackCode
	IsActionSupported(orgID string, action CommerceActionType) bool
}
