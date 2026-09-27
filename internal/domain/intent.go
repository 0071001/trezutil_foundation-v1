package domain

import "fmt"

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
	ActionCancelPayment    CommerceActionType = "CANCEL_PAYMENT"
	ActionRefundPayment    CommerceActionType = "REFUND_PAYMENT"
	ActionAuthorizePayment CommerceActionType = "AUTHORIZE_PAYMENT"
	ActionCapturePayment   CommerceActionType = "CAPTURE_PAYMENT"
	ActionGetRefundStatus  CommerceActionType = "GET_REFUND_STATUS"

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
	case ActionInitiatePayment, ActionCancelPayment, ActionRefundPayment,
		ActionAuthorizePayment, ActionCapturePayment:
		return ActionCategoryFinancial
	default:
		return ActionCategoryRead
	}
}

// ConversationIntent represents an extracted intent from a conversation.
type ConversationIntent struct {
	Name                 string
	Confidence           float64
	Source               string // USER, SYSTEM, AI, APPLICATION
	Entities             []ConversationEntity
	ActionType           CommerceActionType
	Specialist           AISpecialistCode
	VerticalPack         PackCode
	RequiresConfirmation bool
	MissingParameters    []MissingParameter
	Explanation          string
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

// ToAction converts a ConversationIntent into a CommerceAction.
func (i ConversationIntent) ToAction(conversationID string, actor CommerceActor, tenantID string) *CommerceAction {
	action := NewCommerceAction(i.ActionType, actor, conversationID)
	action.WithTenant(tenantID)
	action.WithSource(i.Source)

	for _, entity := range i.Entities {
		action.Parameters[entity.Name] = entity.NormalizedValue
		if entity.NormalizedValue == "" {
			action.Parameters[entity.Name] = entity.Value
		}
	}

	if i.RequiresConfirmation {
		action.ConfirmationState = ConfirmationPending
	}

	return action
}

// EntitiesFromParams converts a parameter map to conversation entities.
func EntitiesFromParams(params map[string]interface{}) []ConversationEntity {
	entities := make([]ConversationEntity, 0, len(params))
	for k, v := range params {
		entity := ConversationEntity{
			Name:            k,
			Value:           fmt.Sprintf("%v", v),
			NormalizedValue: fmt.Sprintf("%v", v),
			Source:          "AI",
		}
		entities = append(entities, entity)
	}
	return entities
}

// MissingRequiredParams checks the intent against known required parameters and returns any that are missing.
func (i ConversationIntent) MissingRequiredParams() []MissingParameter {
	required := RequiredParamsForAction[i.ActionType]
	var missing []MissingParameter
	for _, reqParam := range required {
		found := false
		for _, entity := range i.Entities {
			if entity.Name == reqParam {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, MissingParameter{
				Name:        reqParam,
				Description: reqParam + " is required",
			})
		}
	}
	return missing
}

// RequiredParamsForAction maps action types to their required parameter names.
var RequiredParamsForAction = map[CommerceActionType][]string{
	ActionCreateOrder:           {"listing_id"},
	ActionGetOrder:              {"order_id"},
	ActionCancelOrder:           {"order_id"},
	ActionCreateListing:         {"listing"},
	ActionPublishListing:        {"listing_id"},
	ActionPauseListing:          {"listing_id"},
	ActionResumeListing:         {"listing_id"},
	ActionCreateReservation:     {"listing_id", "start_at", "end_at"},
	ActionGetReservation:        {"reservation_id"},
	ActionCancelReservation:     {"reservation_id"},
	ActionModifyReservation:     {"reservation_id"},
	ActionCheckAvailability:     {"listing_id", "start_at", "end_at"},
	ActionInitiatePayment:       {"order_id"},
	ActionGetPaymentStatus:      {"order_id"},
	ActionCancelPayment:         {"payment_id"},
	ActionRefundPayment:         {"payment_id", "amount_minor"},
	ActionAuthorizePayment:      {"payment_id"},
	ActionCapturePayment:        {"payment_id"},
	ActionGetRefundStatus:       {"refund_id"},
	ActionGetFulfillmentStatus:  {"order_id"},
	ActionAssignFulfillment:     {"fulfillment_id"},
	ActionTransitionFulfillment: {"fulfillment_id", "target_status"},
}

// IsReadAction returns true if the action is a read operation.
func (i ConversationIntent) IsReadAction() bool {
	return i.ActionType.GetCategory() == ActionCategoryRead
}
