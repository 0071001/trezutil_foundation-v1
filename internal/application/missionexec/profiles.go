package missionexec

import (
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * What a step asks of the runtime. Two executable actions, both read-only and both already served by the runtime
 * (handler, parameter validation and tests exist; neither reserves, orders or pays):
 *
 *   - SEARCH_LISTINGS, the catalogue search. A step is executable when it maps unambiguously onto a vertical of the
 *     existing vertical catalogue: the request is then "search this vertical, in the mission's place if it has one".
 *   - CHECK_AVAILABILITY, whether a listing is free on the mission's dates. It needs two things the step cannot invent:
 *     the listing, taken BY REFERENCE from the search it depends on (the first listing that search found, which is
 *     deterministic: the runtime's own top result), and the mission's start date. A step missing either is not run with
 *     a guess: it is blocked (needs_input) until the person provides it.
 *
 * Nothing is invented. A step with no mapping is not executable yet, and says so.
 *
 * Not executable yet, for the reasons given: FIND_VENUE, FIND_DECORATION, FIND_ENTERTAINMENT, FIND_PHOTOGRAPHY,
 * FIND_ACTIVITIES, FIND_MOVERS and FIND_STORAGE have no vertical of their own (they would need search words or
 * categories, which is a step-by-step design of its own).
 */

// checkAvailabilityStep is the step type that checks availability.
const checkAvailabilityStep = "CHECK_AVAILABILITY"

// Missing inputs, as named in a step's Detail.
const (
	missingDates   = "dates"
	missingListing = "listing"
)

// secondsPerDay is the shortest availability window: a check needs an end after its start, and a mission with only a
// start date is a one-day window.
const secondsPerDay = 86400

// actionable reports whether a step maps onto an action at all, whatever it still lacks.
func actionable(m *domain.Mission, step *domain.MissionStep) bool {
	switch step.ActionType {
	case domain.ActionSearchListings:
		_, ok := searchRequest(m, step)
		return ok
	case domain.ActionCheckAvailability:
		return step.Type == checkAvailabilityStep
	}
	return false
}

// buildParams builds the runtime parameters of a step from the mission and from what the steps it depends on produced
// (their latest executions, by reference). missing names what is not known yet ("" when the step has everything); a
// step with no mapping returns nil and "" (it is not executable, which is another answer).
func buildParams(m *domain.Mission, step *domain.MissionStep, upstream []domain.UpstreamStep) (params map[string]interface{}, missing string) {
	switch step.ActionType {
	case domain.ActionSearchListings:
		params, _ = searchRequest(m, step)
		return params, ""
	case domain.ActionCheckAvailability:
		if step.Type != checkAvailabilityStep {
			return nil, ""
		}
		return availabilityRequest(m, upstream)
	}
	return nil, ""
}

// availabilityRequest builds the CHECK_AVAILABILITY parameters: the first listing an upstream search found, the
// mission's dates and, when it has one, its head count.
func availabilityRequest(m *domain.Mission, upstream []domain.UpstreamStep) (map[string]interface{}, string) {
	if m.StartDate <= 0 {
		return nil, missingDates // only what the person said: a date is never made up
	}
	listing := firstListing(upstream)
	if listing == "" {
		return nil, missingListing // the search found nothing, or its result is gone
	}
	end := m.EndDate
	if end < m.StartDate+secondsPerDay {
		end = m.StartDate + secondsPerDay
	}
	params := map[string]interface{}{"listing_id": listing, "start_at": m.StartDate, "end_at": end}
	if m.Participants > 0 {
		params["party_size"] = m.Participants
	}
	return params, ""
}

// firstListing returns the first listing id found by a completed upstream step, in the steps' order, or "".
func firstListing(upstream []domain.UpstreamStep) string {
	for i := range upstream {
		up := &upstream[i]
		if up.Step.Status != domain.StepCompleted || up.Execution == nil || up.Execution.Status != domain.ExecutionCompleted {
			continue
		}
		switch ids := up.Execution.ResultPayload["listing_ids"].(type) {
		case []string:
			for _, id := range ids {
				if id != "" {
					return id
				}
			}
		case []interface{}: // after a round trip through the database
			for _, raw := range ids {
				if id, ok := raw.(string); ok && id != "" {
					return id
				}
			}
		}
	}
	return ""
}

// summarize reduces the runtime's answer to what an execution keeps and the reference to what it concerns.
func summarize(step *domain.MissionStep, exec *domain.MissionStepExecution, data, params map[string]interface{}) (payload map[string]interface{}, reference string) {
	if step.ActionType == domain.ActionCheckAvailability {
		payload = summarizeAvailability(data, params)
		id, _ := payload["listing_id"].(string)
		return payload, id // the listing that was checked is an existing entity: that is the reference
	}
	// a search result set is not a commerce entity: the record itself is the reference
	return summarizeSearch(data, params), exec.ID
}

// maxReasonLength bounds the free-text reason an availability answer may carry into the record.
const maxReasonLength = 200

// summarizeAvailability keeps the answer to "is this listing free then?": the listing, the verdict and the counts.
func summarizeAvailability(data, params map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{"available": false}
	if id, ok := params["listing_id"].(string); ok {
		payload["listing_id"] = id
	}
	if v, ok := params["start_at"]; ok {
		payload["start_at"] = v
	}
	if v, ok := params["end_at"]; ok {
		payload["end_at"] = v
	}
	found, _ := data["data"].(map[string]interface{})
	if found == nil {
		return payload
	}
	if v, ok := found["available"].(bool); ok {
		payload["available"] = v
	}
	for _, k := range []string{"capacity", "remaining"} {
		if n, ok := wholeNumber(found[k]); ok {
			payload[k] = n
		}
	}
	if r, ok := found["reason"].(string); ok && r != "" {
		if runes := []rune(r); len(runes) > maxReasonLength {
			r = string(runes[:maxReasonLength])
		}
		payload["reason"] = r
	}
	return payload
}

func wholeNumber(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// searchVerticals maps a step type, for a mission type, to the vertical its search runs in.
var searchVerticals = map[string]map[domain.MissionType]domain.Vertical{
	"FIND_ACCOMMODATION": {
		domain.MissionTypeStay:   domain.VerticalHotel,
		domain.MissionTypeTravel: domain.VerticalHotel,
		domain.MissionTypeMove:   domain.VerticalRealEstate,
	},
	"FIND_CATERING": {
		domain.MissionTypeEvent: domain.VerticalRestaurant,
	},
	"FIND_TRANSPORT": {
		domain.MissionTypeTravel: domain.VerticalTransport,
		domain.MissionTypeEvent:  domain.VerticalTransport,
	},
	"FIND_TRANSPORT_OPTION": {
		domain.MissionTypeTransport: domain.VerticalTransport,
	},
	"FIND_LOCAL_TRANSPORT": {
		domain.MissionTypeTravel: domain.VerticalVehicleRental,
	},
	"FIND_MOVING_TRANSPORT": {
		domain.MissionTypeMove: domain.VerticalVehicleRental,
	},
}

// maxListed bounds how many listing ids an execution keeps: references, not the result set.
const maxListed = 10

// searchRequest builds the SEARCH_LISTINGS parameters of a step, or reports that the step has no mapping.
func searchRequest(m *domain.Mission, step *domain.MissionStep) (map[string]interface{}, bool) {
	if step.ActionType != domain.ActionSearchListings {
		return nil, false
	}
	v, ok := searchVerticals[step.Type][m.Type]
	if !ok {
		return nil, false
	}
	params := map[string]interface{}{"vertical": string(v), "page_size": maxListed}
	if m.Location != "" { // only what the person said: a place is never made up
		params["location"] = m.Location
	}
	return params, true
}

// summarizeSearch reduces a SEARCH_LISTINGS answer to what a mission keeps: how many were found and the ids of the
// first few listings, which are references to existing entities.
func summarizeSearch(data map[string]interface{}, params map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{"total": 0, "listing_ids": []string{}, "has_more": false}
	if v, ok := params["vertical"].(string); ok {
		payload["vertical"] = v
	}
	if l, ok := params["location"].(string); ok {
		payload["location"] = l
	}
	found, _ := data["data"].(map[string]interface{})
	if found == nil {
		return payload
	}
	ids := []string{}
	switch items := found["items"].(type) {
	case []domain.SearchResult:
		for _, it := range items {
			ids = append(ids, it.ListingID)
		}
	case []interface{}:
		for _, raw := range items {
			if it, ok := raw.(map[string]interface{}); ok {
				if id, ok := it["ListingID"].(string); ok {
					ids = append(ids, id)
				} else if id, ok := it["listing_id"].(string); ok {
					ids = append(ids, id)
				}
			}
		}
	}
	total := len(ids)
	switch n := found["total"].(type) {
	case int:
		total = n
	case int64:
		total = int(n)
	case float64:
		total = int(n)
	}
	if len(ids) > maxListed {
		ids = ids[:maxListed]
	}
	payload["total"], payload["listing_ids"] = total, ids
	if more, ok := found["has_next"].(bool); ok {
		payload["has_more"] = more
	}
	return payload
}
