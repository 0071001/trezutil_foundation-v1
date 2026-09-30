package vertical

import (
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type PackDefinition func() *domain.VerticalPack

var PackDefinitions = map[domain.PackCode]PackDefinition{
	domain.PackECommerce: func() *domain.VerticalPack {
		pack := domain.NewVerticalPack(
			domain.PackECommerce,
			"E-Commerce Pack",
			[]domain.PackCapability{
				domain.CapabilityCatalog,
				domain.CapabilitySearch,
				domain.CapabilityOrdering,
				domain.CapabilityInventory,
				domain.CapabilityFulfillment,
				domain.CapabilityCustomerSupport,
				domain.CapabilityMerchantOps,
			},
			domain.AISpecialistCommerce,
		)
		pack.Description = "Vertical pack for e-commerce marketplace operations"
		pack.SupportedActions = []domain.CommerceActionType{
			domain.ActionSearchListings,
			domain.ActionGetListing,
			domain.ActionCompareListings,
			domain.ActionSuggest,
			domain.ActionCreateListing,
			domain.ActionPublishListing,
			domain.ActionPauseListing,
			domain.ActionResumeListing,
			domain.ActionCreateOrder,
			domain.ActionGetOrder,
			domain.ActionCancelOrder,
			domain.ActionGetFulfillmentStatus,
			domain.ActionInitiatePayment,
			domain.ActionGetPaymentStatus,
			domain.ActionGetDeliveryStatus,
			domain.ActionAssignFulfillment,
			domain.ActionTransitionFulfillment,
			domain.ActionUpdateFulfillmentTracking,
		}
		pack.Policies = []domain.PackPolicy{
			{Key: "require_payment_before_fulfillment", Value: "true"},
			{Key: "allow_partial_reservations", Value: "false"},
			{Key: "inventory_decrement_on", Value: "order_created"},
		}
		pack.Workflow = &domain.PackWorkflow{
			Name:      "ecommerce_checkout",
			EntryStep: "listing",
			Steps: []domain.PackWorkflowStep{
				{Name: "listing", Description: "Search and browse listings", Action: domain.ActionSearchListings, NextSteps: []string{"details", "order"}},
				{Name: "details", Description: "View listing details", Action: domain.ActionGetListing, NextSteps: []string{"checkout"}},
				{Name: "checkout", Description: "Create order", Action: domain.ActionCreateOrder, NextSteps: []string{"payment"}},
				{Name: "payment", Description: "Initiate payment", Action: domain.ActionInitiatePayment, NextSteps: []string{"fulfillment"}},
				{Name: "fulfillment", Description: "Assign fulfillment", Action: domain.ActionAssignFulfillment, NextSteps: []string{"tracking"}},
				{Name: "tracking", Description: "Update tracking info", Action: domain.ActionUpdateFulfillmentTracking, NextSteps: []string{}},
			},
		}
		pack.UIMetadata = domain.PackUIMetadata{
			RoutePrefix: "/commerce",
			Icon:        "shopping-cart",
			Color:       "#2563eb",
			Order:       1,
		}
		return pack
	},
	domain.PackBooking: func() *domain.VerticalPack {
		pack := domain.NewVerticalPack(
			domain.PackBooking,
			"Booking & Hospitality Pack",
			[]domain.PackCapability{
				domain.CapabilityCatalog,
				domain.CapabilitySearch,
				domain.CapabilityAvailability,
				domain.CapabilityReservation,
				domain.CapabilityCapacity,
				domain.CapabilityScheduling,
				domain.CapabilityFulfillment,
				domain.CapabilityCustomerSupport,
			},
			domain.AISpecialistBooking,
		)
		pack.Description = "Vertical pack for booking and reservation operations"
		pack.SupportedActions = []domain.CommerceActionType{
			domain.ActionSearchListings,
			domain.ActionGetListing,
			domain.ActionSuggest,
			domain.ActionCreateReservation,
			domain.ActionCheckAvailability,
			domain.ActionGetReservation,
			domain.ActionCancelReservation,
			domain.ActionModifyReservation,
			domain.ActionGetPaymentStatus,
			domain.ActionGetDeliveryStatus,
		}
		pack.Policies = []domain.PackPolicy{
			{Key: "require_payment_before_reservation", Value: "true"},
			{Key: "allow_overbooking", Value: "false"},
			{Key: "min_advance_booking_hours", Value: "1"},
		}
		pack.Workflow = &domain.PackWorkflow{
			Name:      "booking_flow",
			EntryStep: "search",
			Steps: []domain.PackWorkflowStep{
				{Name: "search", Description: "Search available listings", Action: domain.ActionSearchListings, NextSteps: []string{"details"}},
				{Name: "details", Description: "View listing details", Action: domain.ActionGetListing, NextSteps: []string{"availability"}},
				{Name: "availability", Description: "Check availability", Action: domain.ActionCheckAvailability, NextSteps: []string{"reservation"}},
				{Name: "reservation", Description: "Create reservation", Action: domain.ActionCreateReservation, NextSteps: []string{"confirmation"}},
				{Name: "confirmation", Description: "Confirm reservation", Action: domain.ActionGetReservation, NextSteps: []string{"reminder"}},
				{Name: "reminder", Description: "Send reminder", NextSteps: []string{}},
			},
		}
		pack.UIMetadata = domain.PackUIMetadata{
			RoutePrefix: "/booking",
			Icon:        "calendar-check",
			Color:       "#059669",
			Order:       2,
		}
		return pack
	},
	domain.PackServices: func() *domain.VerticalPack {
		pack := domain.NewVerticalPack(
			domain.PackServices,
			"Services Pack",
			[]domain.PackCapability{
				domain.CapabilityCatalog,
				domain.CapabilitySearch,
				domain.CapabilityAvailability,
				domain.CapabilityReservation,
				domain.CapabilityScheduling,
				domain.CapabilityCapacity,
				domain.CapabilityCustomerSupport,
				domain.CapabilityMerchantOps,
			},
			domain.AISpecialistCommerce,
		)
		pack.Description = "Vertical pack for service provider operations"
		pack.SupportedActions = []domain.CommerceActionType{
			domain.ActionSearchListings,
			domain.ActionGetListing,
			domain.ActionSuggest,
			domain.ActionCreateReservation,
			domain.ActionCheckAvailability,
			domain.ActionGetReservation,
			domain.ActionCancelReservation,
			domain.ActionModifyReservation,
			domain.ActionCheckInventory,
			domain.ActionAdjustInventory,
			domain.ActionGetFulfillmentStatus,
			domain.ActionGetPaymentStatus,
		}
		pack.Policies = []domain.PackPolicy{
			{Key: "require_payment_before_reservation", Value: "true"},
			{Key: "allow_scheduling_into_future_days", Value: "90"},
			{Key: "require_provider_assignment", Value: "true"},
		}
		pack.Workflow = &domain.PackWorkflow{
			Name:      "service_flow",
			EntryStep: "inquiry",
			Steps: []domain.PackWorkflowStep{
				{Name: "inquiry", Description: "Search service listings", Action: domain.ActionSearchListings, NextSteps: []string{"availability"}},
				{Name: "availability", Description: "Check capacity and availability", Action: domain.ActionCheckAvailability, NextSteps: []string{"booking"}},
				{Name: "booking", Description: "Create reservation", Action: domain.ActionCreateReservation, NextSteps: []string{"confirmation"}},
				{Name: "confirmation", Description: "Confirm reservation", Action: domain.ActionGetReservation, NextSteps: []string{"fulfillment"}},
				{Name: "fulfillment", Description: "Assign fulfillment", Action: domain.ActionAssignFulfillment, NextSteps: []string{}},
			},
		}
		pack.UIMetadata = domain.PackUIMetadata{
			RoutePrefix: "/services",
			Icon:        "wrench",
			Color:       "#7c3aed",
			Order:       3,
		}
		return pack
	},
}

func LoadAllPacks(registry *PackRegistry) error {
	for code, def := range PackDefinitions {
		pack := def()
		if err := registry.Register(pack); err != nil {
			return fmt.Errorf("failed to register pack %s: %w", code, err)
		}
	}
	return nil
}
