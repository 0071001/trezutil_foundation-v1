package domain

import (
	"testing"
)

func TestNewVerticalPack(t *testing.T) {
	pack := NewVerticalPack(
		PackECommerce,
		"E-Commerce Pack",
		[]PackCapability{CapabilityCatalog, CapabilitySearch},
		AISpecialistCommerce,
	)

	if pack.ID != "ECOMMERCE" {
		t.Errorf("expected ID ECOMMERCE, got %s", pack.ID)
	}
	if pack.Code != PackECommerce {
		t.Errorf("expected Code ECOMMERCE, got %s", pack.Code)
	}
	if pack.Name != "E-Commerce Pack" {
		t.Errorf("expected Name 'E-Commerce Pack', got %s", pack.Name)
	}
	if pack.Status != PackStatusActive {
		t.Errorf("expected Status ACTIVE, got %s", pack.Status)
	}
	if len(pack.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(pack.Capabilities))
	}
	if pack.AIProfile != AISpecialistCommerce {
		t.Errorf("expected AIProfile COMMERCE_AI, got %s", pack.AIProfile)
	}
	if pack.Version.Major != 1 {
		t.Errorf("expected version major 1, got %d", pack.Version.Major)
	}
}

func TestVerticalPackSupportsAction(t *testing.T) {
	pack := NewVerticalPack(PackECommerce, "E-Commerce", nil, AISpecialistCommerce)
	pack.SupportedActions = []CommerceActionType{
		ActionSearchListings,
		ActionGetListing,
		ActionCreateOrder,
	}

	tests := []struct {
		action    CommerceActionType
		supported bool
	}{
		{ActionSearchListings, true},
		{ActionGetListing, true},
		{ActionCreateOrder, true},
		{ActionCancelOrder, false},
		{ActionCreateReservation, false},
	}

	for _, tt := range tests {
		if got := pack.SupportsAction(tt.action); got != tt.supported {
			t.Errorf("SupportsAction(%s) = %v, want %v", tt.action, got, tt.supported)
		}
	}
}

func TestVerticalPackHasCapability(t *testing.T) {
	pack := NewVerticalPack(PackECommerce, "E-Commerce", nil, AISpecialistCommerce)
	pack.Capabilities = []PackCapability{CapabilityCatalog, CapabilitySearch, CapabilityOrdering}

	tests := []struct {
		cap    PackCapability
		exists bool
	}{
		{CapabilityCatalog, true},
		{CapabilitySearch, true},
		{CapabilityOrdering, true},
		{CapabilityReservation, false},
		{CapabilityInventory, false},
	}

	for _, tt := range tests {
		if got := pack.HasCapability(tt.cap); got != tt.exists {
			t.Errorf("HasCapability(%s) = %v, want %v", tt.cap, got, tt.exists)
		}
	}
}

func TestVerticalPackIsActive(t *testing.T) {
	tests := []struct {
		status PackStatus
		active bool
	}{
		{PackStatusActive, true},
		{PackStatusInactive, false},
		{PackStatusDeprecated, false},
	}

	for _, tt := range tests {
		pack := NewVerticalPack(PackECommerce, "E-Commerce", nil, AISpecialistCommerce)
		pack.Status = tt.status
		if got := pack.IsActive(); got != tt.active {
			t.Errorf("IsActive() for status %s = %v, want %v", tt.status, got, tt.active)
		}
	}
}

func TestVerticalPackCanTransition(t *testing.T) {
	pack := NewVerticalPack(PackECommerce, "E-Commerce", nil, AISpecialistCommerce)
	pack.Workflow = nil

	if pack.CanTransition() {
		t.Error("expected CanTransition() to be false when Workflow is nil")
	}

	pack.Workflow = &PackWorkflow{Name: "test"}
	if !pack.CanTransition() {
		t.Error("expected CanTransition() to be true when Workflow is set")
	}
}

func TestVerticalPackValidate(t *testing.T) {
	tests := []struct {
		name    string
		pack    *VerticalPack
		wantErr bool
	}{
		{
			name: "valid pack",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: false,
		},
		{
			name: "missing code",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.Code = ""
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: true,
		},
		{
			name: "missing name",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.Name = ""
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: true,
		},
		{
			name: "no capabilities",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", nil, AISpecialistCommerce)
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: true,
		},
		{
			name: "no supported actions",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.SupportedActions = nil
				return p
			}(),
			wantErr: true,
		},
		{
			name: "missing AI profile",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.AIProfile = ""
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: true,
		},
		{
			name: "invalid status",
			pack: func() *VerticalPack {
				p := NewVerticalPack(PackECommerce, "E-Commerce", []PackCapability{CapabilityCatalog}, AISpecialistCommerce)
				p.Status = "UNKNOWN"
				p.SupportedActions = []CommerceActionType{ActionSearchListings}
				return p
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pack.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPackWorkflowValidate(t *testing.T) {
	tests := []struct {
		name    string
		wf      *PackWorkflow
		wantErr bool
	}{
		{
			name: "valid workflow",
			wf: &PackWorkflow{
				Name:      "test_workflow",
				EntryStep: "start",
				Steps: []PackWorkflowStep{
					{Name: "start", Action: ActionSearchListings, NextSteps: []string{"end"}},
					{Name: "end", NextSteps: []string{}},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			wf: &PackWorkflow{
				EntryStep: "start",
				Steps:     []PackWorkflowStep{{Name: "start"}},
			},
			wantErr: true,
		},
		{
			name: "missing entry step",
			wf: &PackWorkflow{
				Name:  "test",
				Steps: []PackWorkflowStep{{Name: "start"}},
			},
			wantErr: true,
		},
		{
			name: "entry step not in steps",
			wf: &PackWorkflow{
				Name:      "test",
				EntryStep: "nonexistent",
				Steps:     []PackWorkflowStep{{Name: "start"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.wf.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPackWorkflowValidTransitions(t *testing.T) {
	wf := &PackWorkflow{
		Name:      "test",
		EntryStep: "start",
		Steps: []PackWorkflowStep{
			{Name: "start", NextSteps: []string{"middle", "end"}},
			{Name: "middle", NextSteps: []string{"end"}},
			{Name: "end", NextSteps: []string{}},
		},
	}

	tests := []struct {
		from  string
		to    string
		valid bool
	}{
		{"start", "middle", true},
		{"start", "end", true},
		{"start", "nonexistent", false},
		{"middle", "end", true},
		{"middle", "start", false},
		{"end", "start", false},
		{"nonexistent", "start", false},
	}

	for _, tt := range tests {
		got := wf.ValidTransitions(tt.from, tt.to)
		if got != tt.valid {
			t.Errorf("ValidTransitions(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.valid)
		}
	}
}

func TestPackWorkflowStepsByAction(t *testing.T) {
	wf := &PackWorkflow{
		Name:      "test",
		EntryStep: "listing",
		Steps: []PackWorkflowStep{
			{Name: "listing", Action: ActionSearchListings, NextSteps: []string{"details"}},
			{Name: "details", Action: ActionGetListing, NextSteps: []string{"checkout"}},
			{Name: "checkout", NextSteps: []string{}},
		},
	}

	result := wf.StepsByAction()

	if len(result[ActionSearchListings]) != 1 {
		t.Errorf("expected 1 step for ActionSearchListings, got %d", len(result[ActionSearchListings]))
	}
	if len(result[ActionGetListing]) != 1 {
		t.Errorf("expected 1 step for ActionGetListing, got %d", len(result[ActionGetListing]))
	}
	if len(result[ActionCreateOrder]) != 0 {
		t.Errorf("expected 0 steps for ActionCreateOrder, got %d", len(result[ActionCreateOrder]))
	}
}
