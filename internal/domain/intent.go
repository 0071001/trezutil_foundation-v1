package domain

// CommerceActionType represents the type of a commerce action.
type CommerceActionType string

const (
	ActionSearchListings  CommerceActionType = "SEARCH_LISTINGS"
	ActionGetListing      CommerceActionType = "GET_LISTING"
	ActionCompareListings CommerceActionType = "COMPARE_LISTINGS"
	ActionSuggest         CommerceActionType = "SUGGEST"

	ActionCreateListing  CommerceActionType = "CREATE_LISTING"
	ActionUpdateListing  CommerceActionType = "UPDATE_LISTING"
	ActionPublishListing CommerceActionType = "PUBLISH_LISTING"
	ActionPauseListing   CommerceActionType = "PAUSE_LISTING"
	ActionResumeListing  CommerceActionType = "RESUME_LISTING"

	ActionCreateOrder CommerceActionType = "CREATE_ORDER"
	ActionGetOrder    CommerceActionType = "GET_ORDER"
	ActionCancelOrder CommerceActionType = "CANCEL_ORDER"

	ActionCheckAvailability CommerceActionType = "CHECK_AVAILABILITY"
	ActionCreateReservation CommerceActionType = "CREATE_RESERVATION"
	ActionGetReservation    CommerceActionType = "GET_RESERVATION"
	ActionCancelReservation CommerceActionType = "CANCEL_RESERVATION"
	ActionModifyReservation CommerceActionType = "MODIFY_RESERVATION"

	ActionCheckInventory  CommerceActionType = "CHECK_INVENTORY"
	ActionAdjustInventory CommerceActionType = "ADJUST_INVENTORY"

	ActionGetFulfillmentStatus      CommerceActionType = "GET_FULFILLMENT_STATUS"
	ActionTransitionFulfillment     CommerceActionType = "TRANSITION_FULFILLMENT"
	ActionAssignFulfillment         CommerceActionType = "ASSIGN_FULFILLMENT"
	ActionUpdateFulfillmentTracking CommerceActionType = "UPDATE_FULFILLMENT_TRACKING"

	ActionInitiatePayment  CommerceActionType = "INITIATE_PAYMENT"
	ActionGetPaymentStatus CommerceActionType = "GET_PAYMENT_STATUS"

	ActionGetDeliveryStatus CommerceActionType = "GET_DELIVERY_STATUS"

	ActionCreateCampaign  CommerceActionType = "CREATE_MARKETING_CAMPAIGN"
	ActionGetSellerReport CommerceActionType = "GET_SELLER_REPORT"
)

// ActionCategory represents the risk category of an action.
type ActionCategory string

const (
	ActionCategoryRead      ActionCategory = "READ"
	ActionCategoryWrite     ActionCategory = "WRITE"
	ActionCategoryFinancial ActionCategory = "FINANCIAL"
)

// GetCategory returns the risk category for an action type.
func (t CommerceActionType) GetCategory() ActionCategory {
	switch t {
	case ActionCreateListing, ActionUpdateListing, ActionPublishListing, ActionPauseListing,
		ActionCreateOrder, ActionCancelOrder, ActionCreateCampaign,
		ActionCreateReservation, ActionCancelReservation, ActionModifyReservation,
		ActionAdjustInventory, ActionAssignFulfillment, ActionTransitionFulfillment,
		ActionUpdateFulfillmentTracking:
		return ActionCategoryWrite
	case ActionCheckAvailability, ActionGetOrder, ActionGetListing, ActionCompareListings,
		ActionGetReservation, ActionCheckInventory, ActionGetPaymentStatus,
		ActionGetDeliveryStatus, ActionGetSellerReport, ActionSuggest,
		ActionGetFulfillmentStatus:
		return ActionCategoryRead
	case ActionInitiatePayment:
		return ActionCategoryFinancial
	default:
		return ActionCategoryRead
	}
}

// ConversationIntent represents an extracted intent from a conversation.
type ConversationIntent struct {
	Name       string
	Confidence float64
	Source     string // USER, SYSTEM, AI, APPLICATION
	Entities   []ConversationEntity
	ActionType CommerceActionType
}

// ConversationEntity represents an extracted entity from a conversation.
type ConversationEntity struct {
	Name            string
	Value           string
	NormalizedValue string
	Confidence      float64
	Source          string // USER, SYSTEM, AI, APPLICATION
}

// NewConversationIntent creates a new intent.
func NewConversationIntent(name string, actionType CommerceActionType) ConversationIntent {
	return ConversationIntent{
		Name:       name,
		ActionType: actionType,
		Confidence: 1.0,
		Source:     "USER",
	}
}

// AddEntity adds an entity to the intent.
func (i *ConversationIntent) AddEntity(entity ConversationEntity) {
	i.Entities = append(i.Entities, entity)
}

// GetEntity returns the first entity with the given name.
func (i ConversationIntent) GetEntity(name string) (*ConversationEntity, bool) {
	for idx := range i.Entities {
		if i.Entities[idx].Name == name {
			return &i.Entities[idx], true
		}
	}
	return nil, false
}
