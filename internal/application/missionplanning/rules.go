// Package missionplanning holds the planners that turn a Mission into a plan (domain.MissionPlanner).
//
// RuleBased is the first one: a template of steps per mission type, no model. It is deliberately plain so that
// another planner (for example one backed by a language model) can later sit behind the same interface without
// any change to Mission, MissionStep or the Commerce Action Runtime. A planner only proposes steps; it never
// searches, reserves, orders, pays, uses a credit or runs a commerce action.
package missionplanning

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// Step types. Each names one thing to find or check, stable enough for a later phase to connect it to a commerce
// action and its parameters. Titles and descriptions are in English: clients show them in the person's language
// from the step type.
const (
	FindVenue          = "FIND_VENUE"
	FindCatering       = "FIND_CATERING"
	FindDecoration     = "FIND_DECORATION"
	FindEntertainment  = "FIND_ENTERTAINMENT"
	FindPhotography    = "FIND_PHOTOGRAPHY"
	FindTransport      = "FIND_TRANSPORT"
	FindAccommodation  = "FIND_ACCOMMODATION"
	FindLocalTransport = "FIND_LOCAL_TRANSPORT"
	FindActivities     = "FIND_ACTIVITIES"
	FindMovingVehicle  = "FIND_MOVING_TRANSPORT"
	FindMovers         = "FIND_MOVERS"
	FindStorage        = "FIND_STORAGE"
	FindTransportOpt   = "FIND_TRANSPORT_OPTION"
	CheckAvailability  = "CHECK_AVAILABILITY"
)

// PlannerName and TemplatesVersion identify this planner and its templates; changing the templates means a new
// TemplatesVersion, which makes the next planning a different plan rather than a silent edit of the old one.
const (
	PlannerName = "rules"
	// Version 2 adds the dependencies between steps: an availability check waits for the search that found what it checks.
	TemplatesVersion = "2"
)

type templateStep struct {
	typ, title, description string
	action                  domain.CommerceActionType
	// after are the types of earlier steps of the template this one waits for.
	after []string
}

// find is a step that searches the catalogue; check looks at availability. Both are actions the runtime has.
func find(typ, title, description string) templateStep {
	return templateStep{typ, title, description, domain.ActionSearchListings, nil}
}
func check(typ, title, description string, after ...string) templateStep {
	return templateStep{typ, title, description, domain.ActionCheckAvailability, after}
}

// templates is the plan of each mission type. Types without a template (OTHER, BUSINESS_TRIP, PROJECT) are not
// planned: the planner asks for more information instead of making steps up. The steps are the same for every
// mission of a type: nothing here depends on, or invents, a place, a date, a head count or a budget.
var templates = map[domain.MissionType][]templateStep{
	domain.MissionTypeEvent: {
		find(FindVenue, "Find a venue", "A place for the event."),
		find(FindCatering, "Find catering", "Food and drink for the guests."),
		find(FindDecoration, "Find decoration", "Decoration for the place."),
		find(FindEntertainment, "Find entertainment", "Music or entertainment for the event."),
		find(FindPhotography, "Find a photographer", "Photos or video of the event."),
		find(FindTransport, "Find transport", "Getting people to and from the event."),
	},
	domain.MissionTypeTravel: {
		find(FindTransport, "Find transport", "Getting to the destination."),
		find(FindAccommodation, "Find accommodation", "Somewhere to stay."),
		find(FindLocalTransport, "Find local transport", "Getting around once there."),
		find(FindActivities, "Find activities", "Things to do during the trip."),
	},
	domain.MissionTypeMove: {
		find(FindAccommodation, "Find accommodation", "The place to move to."),
		find(FindMovingVehicle, "Find moving transport", "A vehicle to carry the belongings."),
		find(FindMovers, "Find movers", "People to help with the move."),
		find(FindStorage, "Find storage", "Somewhere to keep belongings in between."),
	},
	domain.MissionTypeTransport: {
		find(FindTransportOpt, "Find a transport option", "The way of getting there."),
		check(CheckAvailability, "Check availability", "Whether the option is free when it is needed.", FindTransportOpt),
	},
	domain.MissionTypeStay: {
		find(FindAccommodation, "Find accommodation", "Somewhere to stay."),
		check(CheckAvailability, "Check availability", "Whether it is free when it is needed.", FindAccommodation),
	},
}

// RuleBased plans from the templates above.
type RuleBased struct {
	supports func(domain.CommerceActionType) bool
}

// NewRuleBased creates the planner. supports answers whether the Commerce OS can run an action today (the
// runtime's handler registry); it is only asked, never used to run anything. With nil, no action counts as
// supported.
func NewRuleBased(supports func(domain.CommerceActionType) bool) *RuleBased {
	return &RuleBased{supports: supports}
}

var _ domain.MissionPlanner = (*RuleBased)(nil)

// Name identifies the planner.
func (p *RuleBased) Name() string { return PlannerName }

// Version identifies its templates.
func (p *RuleBased) Version() string { return TemplatesVersion }

// Plannable reports whether there is a template for the type.
func (p *RuleBased) Plannable(t domain.MissionType) bool { return len(templates[t]) > 0 }

// Plan proposes the steps of the mission's type. A type without a template gets no steps and a request for more
// information; the mission is never changed.
func (p *RuleBased) Plan(_ context.Context, m *domain.Mission) (*domain.PlanProposal, error) {
	proposal := &domain.PlanProposal{Planner: PlannerName, PlannerVersion: TemplatesVersion}
	tpl := templates[m.Type]
	if len(tpl) == 0 {
		proposal.NeedsInformation, proposal.Reason = true, domain.ReasonNoTemplate
		return proposal, nil
	}
	for _, t := range tpl {
		support := domain.ActionNotYetSupported
		if p.supports != nil && p.supports(t.action) {
			support = domain.ActionSupported
		}
		proposal.Steps = append(proposal.Steps, domain.PlannedStep{
			Type: t.typ, Title: t.title, Description: t.description, ActionType: t.action, ActionSupport: support, DependsOn: t.after,
		})
	}
	return proposal, nil
}

// TemplateTypes lists the mission types that have a template, in display order (tests, documentation).
func TemplateTypes() []domain.MissionType {
	var out []domain.MissionType
	for _, t := range domain.MissionTypes() {
		if len(templates[t]) > 0 {
			out = append(out, t)
		}
	}
	return out
}
