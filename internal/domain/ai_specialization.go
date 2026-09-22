package domain

import (
	"fmt"
	"sync"
)

type AISpecialistCode string

const (
	AISpecialistCommerce  AISpecialistCode = "COMMERCE_AI"
	AISpecialistBooking   AISpecialistCode = "BOOKING_AI"
	AISpecialistSupport   AISpecialistCode = "SUPPORT_AI"
	AISpecialistMarketing AISpecialistCode = "MARKETING_AI"
	AISpecialistLogistics AISpecialistCode = "LOGISTICS_AI"
)

type AISpecialistStatus string

const (
	AISpecialistStatusActive   AISpecialistStatus = "ACTIVE"
	AISpecialistStatusInactive AISpecialistStatus = "INACTIVE"
)

type AISpecialistEntity struct {
	ID          string
	Code        AISpecialistCode
	Name        string
	Description string
	Status      AISpecialistStatus
	OwnerAction []CommerceActionType
	CanHandoff  []AISpecialistCode
	Config      map[string]interface{}
	CreatedAt   int64
	UpdatedAt   int64
}

func NewAISpecialist(code AISpecialistCode, name string, ownerActions []CommerceActionType, handoffs []AISpecialistCode) *AISpecialistEntity {
	return &AISpecialistEntity{
		ID:          string(code),
		Code:        code,
		Name:        name,
		Description: fmt.Sprintf("%s specialist for trezutil", name),
		Status:      AISpecialistStatusActive,
		OwnerAction: ownerActions,
		CanHandoff:  handoffs,
		Config:      make(map[string]interface{}),
	}
}

func (s *AISpecialistEntity) IsActive() bool {
	return s.Status == AISpecialistStatusActive
}

func (s *AISpecialistEntity) CanHandle(action CommerceActionType) bool {
	for _, a := range s.OwnerAction {
		if a == action {
			return true
		}
	}
	return false
}

func (s *AISpecialistEntity) CanHandoffTo(target AISpecialistCode) bool {
	for _, h := range s.CanHandoff {
		if h == target {
			return true
		}
	}
	return false
}

func (s *AISpecialistEntity) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("specialist code is required")
	}
	if s.Name == "" {
		return fmt.Errorf("specialist name is required")
	}
	if s.Status != AISpecialistStatusActive && s.Status != AISpecialistStatusInactive {
		return fmt.Errorf("invalid specialist status: %s", s.Status)
	}
	if len(s.OwnerAction) == 0 {
		return fmt.Errorf("specialist must own at least one action")
	}
	return nil
}

type AISpecialistRepository interface {
	Create(ctx interface{}, specialist *AISpecialistEntity) error
	FindByCode(ctx interface{}, code AISpecialistCode) (*AISpecialistEntity, error)
	FindAll(ctx interface{}) ([]*AISpecialistEntity, error)
	FindActive(ctx interface{}) ([]*AISpecialistEntity, error)
	Update(ctx interface{}, specialist *AISpecialistEntity) error
	Delete(ctx interface{}, id string) error
}

var DefaultSpecialists = []*AISpecialistEntity{
	NewAISpecialist(
		AISpecialistCommerce,
		"Commerce AI Specialist",
		[]CommerceActionType{
			ActionSearchListings, ActionGetListing, ActionCompareListings,
			ActionSuggest, ActionCheckAvailability, ActionCheckInventory,
			ActionCreateOrder, ActionGetOrder, ActionCancelOrder,
			ActionCreateReservation, ActionGetReservation, ActionCancelReservation,
			ActionModifyReservation, ActionGetFulfillmentStatus,
			ActionGetDeliveryStatus, ActionGetPaymentStatus,
		},
		[]AISpecialistCode{AISpecialistBooking, AISpecialistSupport, AISpecialistMarketing},
	),
	NewAISpecialist(
		AISpecialistBooking,
		"Booking AI Specialist",
		[]CommerceActionType{
			ActionCheckAvailability, ActionCreateReservation, ActionGetReservation,
			ActionCancelReservation, ActionModifyReservation,
			ActionGetFulfillmentStatus, ActionGetDeliveryStatus,
		},
		[]AISpecialistCode{AISpecialistCommerce},
	),
	NewAISpecialist(
		AISpecialistSupport,
		"Support AI Specialist",
		[]CommerceActionType{
			ActionSearchListings, ActionGetListing, ActionSuggest,
			ActionGetOrder, ActionGetPaymentStatus,
		},
		[]AISpecialistCode{AISpecialistCommerce, AISpecialistLogistics},
	),
	NewAISpecialist(
		AISpecialistMarketing,
		"Marketing AI Specialist",
		[]CommerceActionType{
			ActionSuggest, ActionCreateCampaign, ActionGetSellerReport,
			ActionSearchListings, ActionCompareListings,
		},
		[]AISpecialistCode{AISpecialistCommerce},
	),
	NewAISpecialist(
		AISpecialistLogistics,
		"Logistics AI Specialist",
		[]CommerceActionType{
			ActionGetFulfillmentStatus, ActionGetDeliveryStatus,
			ActionCheckInventory, ActionGetOrder,
		},
		[]AISpecialistCode{AISpecialistSupport, AISpecialistCommerce},
	),
}

var specialistRegistry = struct {
	mu           sync.RWMutex
	specialties  map[AISpecialistCode][]CommerceActionType
	actionToSpec map[CommerceActionType][]AISpecialistCode
}{
	specialties:  make(map[AISpecialistCode][]CommerceActionType),
	actionToSpec: make(map[CommerceActionType][]AISpecialistCode),
}

func init() {
	for _, s := range DefaultSpecialists {
		for _, action := range s.OwnerAction {
			specialistRegistry.actionToSpec[action] = append(
				specialistRegistry.actionToSpec[action], s.Code)
		}
		specialistRegistry.specialties[s.Code] = s.OwnerAction
	}
}

func GetSpecialistsForAction(action CommerceActionType) []AISpecialistCode {
	specialistRegistry.mu.RLock()
	defer specialistRegistry.mu.RUnlock()
	return specialistRegistry.actionToSpec[action]
}

func GetActionsForSpecialist(code AISpecialistCode) []CommerceActionType {
	specialistRegistry.mu.RLock()
	defer specialistRegistry.mu.RUnlock()
	return specialistRegistry.specialties[code]
}

type SpecializationDecision struct {
	SpecialistCode     AISpecialistCode
	Confidence         float64
	OwnerAction        CommerceActionType
	RequiresHandoff    bool
	HandoffTarget      AISpecialistCode
	HandoffReason      string
	HandoffContextKeys []string
}

type HandoffContext struct {
	FromSpecialist    AISpecialistCode
	ToSpecialist      AISpecialistCode
	Reason            string
	SharedContextKeys []string
	Confidential      bool
	OriginalRequestID string
}

type SystemInstructions struct {
	SpecialistCode      AISpecialistCode
	SystemPrompt        string
	Examples            []string
	Constraints         []string
	TenantAware         bool
	SensitiveFieldToken string
}

func DefaultSystemInstructions(code AISpecialistCode) *SystemInstructions {
	var domain string
	var constraints []string
	var examples []string

	switch code {
	case AISpecialistCommerce:
		domain = "e-commerce marketplace"
		constraints = []string{
			"Only classify intents within the commerce action allowlist",
			"Extract listing_id, order_id, reservation_id as parameters",
			"Reject any request containing tenant_id, role, or permission fields",
		}
		examples = []string{
			`User: "Find iPhone 15 in Kinshasa"` + "\n" + `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "iPhone 15", "location": "Kinshasa"}}`,
			`User: "What's my order ORD-123 status?"` + "\n" + `{"intent": "GET_ORDER", "confidence": 0.92, "parameters": {"order_id": "ORD-123"}}`,
		}
	case AISpecialistBooking:
		domain = "booking and reservation management"
		constraints = []string{
			"Focus on availability, reservation creation, and modification",
			"Extract check-in dates, guest count, and resource IDs",
		}
		examples = []string{
			`User: "Book a room for 2 guests on June 15"` + "\n" + `{"intent": "CREATE_RESERVATION", "confidence": 0.9, "parameters": {"guest_count": "2", "date": "2026-06-15"}}`,
		}
	case AISpecialistSupport:
		domain = "customer support and order assistance"
		constraints = []string{
			"Focus on order status, payment status, and listing inquiries",
			"Always verify actor identity before returning sensitive information",
		}
		examples = []string{
			`User: "Where is my order?"` + "\n" + `{"intent": "GET_ORDER", "confidence": 0.88, "parameters": {"order_id": "latest"}}`,
		}
	case AISpecialistMarketing:
		domain = "marketing and campaign management"
		constraints = []string{
			"Focus on campaigns, reports, and promotional suggestions",
		}
		examples = []string{
			`User: "Show me last month's sales report"` + "\n" + `{"intent": "GET_SELLER_REPORT", "confidence": 0.9, "parameters": {"period": "last_month"}}`,
		}
	case AISpecialistLogistics:
		domain = "logistics and fulfillment tracking"
		constraints = []string{
			"Focus on delivery status, inventory levels, and fulfillment",
		}
		examples = []string{
			`User: "Track my package"` + "\n" + `{"intent": "GET_DELIVERY_STATUS", "confidence": 0.85, "parameters": {"tracking_number": "unknown"}}`,
		}
	default:
		domain = "commerce"
	}

	base := fmt.Sprintf(
		"You are a specialized AI assistant for %s. You classify user intent into structured commerce actions. NEVER process tenant_id, user_id, actor_id, role, permissions, confirmation_state, secrets, or any authorization metadata — these are handled server-side. Return ONLY a JSON object with: intent, confidence, parameters, explanation.",
		domain,
	)

	return &SystemInstructions{
		SpecialistCode:      code,
		SystemPrompt:        base,
		Examples:            examples,
		Constraints:         constraints,
		TenantAware:         true,
		SensitiveFieldToken: "token",
	}
}

type AISpecializationError struct {
	Code       AIErrorCode
	Message    string
	Specialist AISpecialistCode
}

func (e AISpecializationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Specialist, e.Code, e.Message)
}

func NewAISpecializationError(code AIErrorCode, specialist AISpecialistCode, msg string) AISpecializationError {
	return AISpecializationError{
		Code:       code,
		Message:    msg,
		Specialist: specialist,
	}
}
