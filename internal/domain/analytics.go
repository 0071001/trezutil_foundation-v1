package domain

import (
	"context"
	"sort"
)

// Analytics are figures computed from the platform's own tables: orders,
// reservations, listings, accounts and conversations, by day, with the period
// before for comparison. Nothing is estimated or stored; a day with no activity
// is a real zero.
//
// Definitions (kept here so every screen and document uses the same ones):
//   - An order is *placed* when it is not a draft.
//   - Revenue is the total of orders in an EarningOrderStatuses status, per
//     currency (currencies are never added together).
//   - Days are Africa/Kinshasa calendar days (UTC+1, no daylight saving).

// AnalyticsTimeZone is the zone days are counted in.
const AnalyticsTimeZone = "Africa/Kinshasa"

// AnalyticsZoneOffsetSeconds is that zone's fixed offset from UTC (no daylight saving).
const AnalyticsZoneOffsetSeconds = 3600

// Analytics windows the API accepts, in days.
const (
	MinAnalyticsDays = 1
	MaxAnalyticsDays = 180
)

// EarningOrderStatuses are the order statuses whose total counts as revenue.
var EarningOrderStatuses = []string{"paid", "confirmed", "processing", "ready_for_fulfillment", "fulfilled", "completed"}

// DayCount is a number of events on one day ("2026-10-04").
type DayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// DayRevenue is the revenue of one day, per currency in minor units.
type DayRevenue struct {
	Date    string           `json:"date"`
	Amounts map[string]int64 `json:"amounts"`
}

// PeriodTotals are the headline numbers of one window.
type PeriodTotals struct {
	Orders        int              `json:"orders"`
	Reservations  int              `json:"reservations"`
	Conversations int              `json:"conversations"`
	NewUsers      int              `json:"new_users"`
	NewListings   int              `json:"new_listings"`
	Customers     int              `json:"customers"`
	Returning     int              `json:"returning_customers"`
	Revenue       map[string]int64 `json:"revenue"`
}

// RankedEntry is one row of a "top" list.
type RankedEntry struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Orders  int              `json:"orders"`
	Units   int              `json:"units"`
	Revenue map[string]int64 `json:"revenue"`
}

// Analytics is the answer to one analytics request.
type Analytics struct {
	GeneratedAt int64 `json:"-"`
	Days        int   `json:"days"`
	// From and To are the first and last day of the current window, inclusive.
	From string `json:"from"`
	To   string `json:"to"`

	Current  PeriodTotals `json:"current"`
	Previous PeriodTotals `json:"previous"`

	Orders        []DayCount   `json:"orders"`
	Revenue       []DayRevenue `json:"revenue"`
	Reservations  []DayCount   `json:"reservations"`
	Listings      []DayCount   `json:"new_listings"`
	Users         []DayCount   `json:"new_users,omitempty"`
	Conversations []DayCount   `json:"conversations,omitempty"`

	OrdersByStatus       map[string]int `json:"orders_by_status"`
	ReservationsByStatus map[string]int `json:"reservations_by_status"`
	ListingsByStatus     map[string]int `json:"listings_by_status"`
	ListingsByVertical   map[string]int `json:"published_listings_by_vertical"`
	UsersByRole          map[string]int `json:"users_by_role,omitempty"`

	TopOrganizations []RankedEntry `json:"top_organizations,omitempty"`
	TopListings      []RankedEntry `json:"top_listings,omitempty"`
}

// RevenueCurrencies lists the currencies with revenue in the window: USD first
// (the platform's main currency), then the others alphabetically. Amounts in
// different currencies, and in different minor units, are never compared.
func (a *Analytics) RevenueCurrencies() []string {
	out := make([]string, 0, len(a.Current.Revenue))
	for c := range a.Current.Revenue {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == "USD") != (out[j] == "USD") {
			return out[i] == "USD"
		}
		return out[i] < out[j]
	})
	return out
}

// AnalyticsRepository computes analytics for the whole platform or one organization.
type AnalyticsRepository interface {
	// Platform covers every organization (administration console).
	Platform(ctx context.Context, now int64, days int) (*Analytics, error)
	// Organization covers one organization's orders, bookings and listings.
	Organization(ctx context.Context, organizationID string, now int64, days int) (*Analytics, error)
}
