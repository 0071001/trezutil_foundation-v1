package negotiation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// Store keeps negotiations (infrastructure/postgres/negotiations.go).
type Store interface {
	// Create writes a new negotiation with its mandate, participants, events and messages, filling their ids.
	Create(ctx context.Context, s *Snapshot) error
	Load(ctx context.Context, id string) (*Snapshot, error)
	// Apply writes a change atomically, or returns ErrConflict when the negotiation is no longer at ExpectedVersion.
	Apply(ctx context.Context, c Change) error
	ListForBuyer(ctx context.Context, buyerID string, limit int) ([]Snapshot, error)
	// ListForOrganizations returns the negotiations where these organizations take part, holding only their own
	// participants and offers.
	ListForOrganizations(ctx context.Context, orgIDs []string, limit int) ([]Snapshot, error)
	ParticipantNegotiation(ctx context.Context, participantID string) (negotiationID, organizationID string, err error)
	DueDeadlines(ctx context.Context, now time.Time, limit int) ([]string, error)
	DueReminders(ctx context.Context, now time.Time, limit int) ([]string, error)
}

// Listings reads the offers asked.
type Listings interface {
	FindByID(ctx context.Context, id string) (*domain.Listing, error)
}

// Organizations reads suppliers and who manages them.
type Organizations interface {
	FindByID(ctx context.Context, id string) (*domain.Organization, error)
	GetMemberRole(ctx context.Context, organizationID, userID string) (domain.OrganizationRole, error)
}

// Memberships lists the businesses a person belongs to.
type Memberships interface {
	FindMembershipsByUserID(ctx context.Context, userID string) ([]domain.OrganizationMember, error)
}

// Orders creates the order of an accepted offer, at the agreed price.
type Orders interface {
	CreateAgreedOrder(ctx context.Context, actorID, listingID string, quantity int, unitMinor int64, currency, idempotencyKey string) (*domain.Order, error)
}

// Config tunes the engine.
type Config struct {
	// PortalURL is where a supplier answers (the link in every notification), e.g. https://trezutil.com/supplier/requests.
	PortalURL string
	// WhatsApp: also tell suppliers on WhatsApp (sent by the outbox, see Deliverer).
	WhatsApp bool
}

// Service runs negotiations.
type Service struct {
	store    Store
	listings Listings
	orgs     Organizations
	members  Memberships
	orders   Orders
	audit    domain.AuditLogger
	cfg      Config
	now      func() time.Time
}

// NewService builds the engine; audit may be nil.
func NewService(store Store, listings Listings, orgs Organizations, members Memberships, orders Orders, audit domain.AuditLogger, cfg Config) *Service {
	return &Service{store: store, listings: listings, orgs: orgs, members: members, orders: orders, audit: audit, cfg: cfg, now: time.Now}
}

// WithClock replaces the clock (tests).
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

// ---------------------------------------------------------------- buyer

// StartRequest is what a buyer asks.
type StartRequest struct {
	ListingIDs     []string `json:"listing_ids"`
	Quantity       int      `json:"quantity"`
	TargetPrice    float64  `json:"target_price"`
	MaxPrice       float64  `json:"max_price"`
	DeadlineHours  int      `json:"deadline_hours"`
	MaxRounds      int      `json:"max_rounds"`
	AutoCounter    *bool    `json:"auto_counter"`
	ConversationID string   `json:"conversation_id"`
}

func toMinor(v float64) int64 { return int64(math.Round(v * 100)) }

// Start opens a negotiation and asks every supplier for its best price.
func (s *Service) Start(ctx context.Context, buyerID string, req StartRequest) (*Snapshot, error) {
	if buyerID == "" {
		return nil, ErrForbidden
	}
	ids := dedupe(req.ListingIDs)
	switch {
	case len(ids) == 0:
		return nil, invalid("listing_ids", "choose at least one offer")
	case len(ids) > 5:
		return nil, invalid("listing_ids", "at most 5 offers per negotiation")
	case req.Quantity < 1 || req.Quantity > 100000:
		return nil, invalid("quantity", "between 1 and 100000")
	case math.IsNaN(req.TargetPrice) || req.TargetPrice <= 0:
		return nil, invalid("target_price", "a positive price")
	case math.IsNaN(req.MaxPrice) || req.MaxPrice < req.TargetPrice:
		return nil, invalid("max_price", "at least the target price")
	case req.MaxPrice > 1e9:
		return nil, invalid("max_price", "too large")
	}
	hours := req.DeadlineHours
	if hours == 0 {
		hours = 48
	}
	if hours < 2 || hours > 14*24 {
		return nil, invalid("deadline_hours", "between 2 hours and 14 days")
	}
	rounds := req.MaxRounds
	if rounds == 0 {
		rounds = 2
	}
	if rounds < 1 || rounds > 5 {
		return nil, invalid("max_rounds", "between 1 and 5")
	}
	auto := req.AutoCounter == nil || *req.AutoCounter

	now := s.now()
	snap := &Snapshot{
		Negotiation: Negotiation{BuyerID: buyerID, ConversationID: strings.TrimSpace(req.ConversationID), Quantity: req.Quantity,
			Status: StatusSoliciting, Deadline: now.Add(time.Duration(hours) * time.Hour), Version: 1, CreatedAt: now, UpdatedAt: now},
		Mandate: Mandate{Version: 1, TargetUnitMinor: toMinor(req.TargetPrice), MaxUnitMinor: toMinor(req.MaxPrice), MaxRounds: rounds, AutoCounter: auto, GrantedBy: buyerID, CreatedAt: now},
	}
	orgs := map[string]*domain.Organization{}
	for _, id := range ids {
		l, err := s.listings.FindByID(ctx, id)
		if err != nil || l == nil {
			return nil, invalid("listing_ids", "an offer was not found")
		}
		if !l.IsPublished() {
			return nil, ErrUnavailable
		}
		if demo, _ := l.Metadata["demo"].(bool); demo {
			return nil, ErrDemoSupplier
		}
		if role, err := s.orgs.GetMemberRole(ctx, l.OrganizationID, buyerID); err == nil && role != "" {
			return nil, ErrOwnOffer
		}
		if snap.Negotiation.Currency == "" {
			snap.Negotiation.Currency = l.PriceCurrency
			snap.Negotiation.Title = l.Title
		} else if l.PriceCurrency != snap.Negotiation.Currency {
			return nil, ErrMixedCurrency
		}
		org, ok := orgs[l.OrganizationID]
		if !ok {
			org, _ = s.orgs.FindByID(ctx, l.OrganizationID)
			orgs[l.OrganizationID] = org
		}
		p := Participant{OrganizationID: l.OrganizationID, ListingID: l.ID, ListingTitle: l.Title, ListUnitMinor: toMinor(l.PriceAmount),
			Status: PInvited, Round: 1, LastActivityAt: now}
		if org != nil {
			p.SupplierName = org.DisplayName
		}
		// Checked before anything is written: a refused message leaves no half-started negotiation.
		if err := CheckOutgoing(s.requestText(snap.Negotiation, p), snap.Mandate, p.ListUnitMinor); err != nil {
			return nil, err
		}
		snap.Participants = append(snap.Participants, p)
	}
	if len(snap.Participants) > 1 {
		snap.Negotiation.Title = fmt.Sprintf("%s (+%d)", snap.Negotiation.Title, len(snap.Participants)-1)
	}
	snap.Events = append(snap.Events, Event{Actor: "buyer", ActorID: buyerID, Kind: "negotiation.started", CreatedAt: now, Data: map[string]interface{}{
		"suppliers": len(snap.Participants), "quantity": req.Quantity, "max_rounds": rounds, "auto_counter": auto, "deadline": snap.Negotiation.Deadline.UTC().Format(time.RFC3339),
	}})
	if err := s.store.Create(ctx, snap); err != nil {
		return nil, err
	}
	// The requests go out once the participants have their ids (the outbox keys use them).
	var msgs []Message
	var events []Event
	for _, p := range snap.Participants {
		text := s.requestText(snap.Negotiation, p)
		msgs = append(msgs, s.notify(ctx, snap.Negotiation, p, "request", text, orgs[p.OrganizationID])...)
		events = append(events, Event{ParticipantID: p.ID, Actor: "system", Kind: "request.sent", CreatedAt: now, Data: map[string]interface{}{"supplier": p.SupplierName}})
	}
	if err := s.store.Apply(ctx, Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: 1, Status: StatusSoliciting, Events: events, Messages: msgs}); err != nil {
		return nil, err
	}
	s.record(ctx, buyerID, "negotiation.mandate_granted", snap.Negotiation.ID, map[string]interface{}{
		"suppliers": len(snap.Participants), "max_rounds": rounds, "auto_counter": auto, "currency": snap.Negotiation.Currency,
	})
	return s.store.Load(ctx, snap.Negotiation.ID)
}

// ListForBuyer lists a buyer's negotiations, most recent first.
func (s *Service) ListForBuyer(ctx context.Context, buyerID string) ([]Snapshot, error) {
	if buyerID == "" {
		return nil, ErrForbidden
	}
	return s.store.ListForBuyer(ctx, buyerID, 50)
}

// Get returns one of the buyer's negotiations.
func (s *Service) Get(ctx context.Context, buyerID, id string) (*Snapshot, error) {
	snap, err := s.store.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if snap.Negotiation.BuyerID != buyerID {
		return nil, ErrNotFound
	}
	return snap, nil
}

// CounterRequest is a counter-offer the buyer types.
type CounterRequest struct {
	ParticipantID string  `json:"participant_id"`
	UnitPrice     float64 `json:"unit_price"`
	Note          string  `json:"note"`
}

// Counter sends the buyer's own counter-offer to one supplier.
func (s *Service) Counter(ctx context.Context, buyerID, id string, req CounterRequest) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, err := s.Get(ctx, buyerID, id)
		if err != nil {
			return nil, err
		}
		p := snap.Participant(req.ParticipantID)
		if p == nil {
			return nil, ErrNotFound
		}
		offer := snap.OpenOffer(p.ID, AuthorSupplier)
		if offer == nil {
			return nil, invalid("participant_id", "this supplier has not made an offer yet")
		}
		price := toMinor(req.UnitPrice)
		now := s.now()
		if err := CheckCounter(snap.Negotiation, snap.Mandate, *p, offer.UnitMinor, price, false, now); err != nil {
			return nil, err
		}
		c := s.counterChange(ctx, snap, p, offer, price, SourceBuyer, buyerID, strings.TrimSpace(req.Note), now)
		c.Status = recompute(snap, c, now)
		if err := s.store.Apply(ctx, c); err != nil {
			return nil, err
		}
		return s.store.Load(ctx, id)
	})
}

// Accept accepts a supplier's offer: the order is created at the agreed price, the other suppliers are told.
//
// The choice is recorded first, which closes the negotiation to every other change (a new offer, a withdrawal, the
// deadline), and only then is the order created: an order never exists for an offer that could still change. If the
// order cannot be created the negotiation is reopened; if the process stops in between, accepting the same offer
// again finishes the acceptance.
func (s *Service) Accept(ctx context.Context, buyerID, id, offerID string) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, err := s.Get(ctx, buyerID, id)
		if err != nil {
			return nil, err
		}
		n := snap.Negotiation
		resuming := n.Status == StatusAccepted && n.OrderID == "" && n.AcceptedOfferID == offerID
		if !n.Status.Open() && !resuming {
			return nil, ErrClosed
		}
		var offer *Offer
		for i := range snap.Offers {
			if snap.Offers[i].ID == offerID {
				offer = &snap.Offers[i]
			}
		}
		if offer == nil || offer.Author != AuthorSupplier || (offer.Status != OfferOpen && !resuming) {
			return nil, invalid("offer_id", "this offer cannot be accepted")
		}
		now := s.now()
		if !resuming && offer.ValidUntil != nil && now.After(*offer.ValidUntil) {
			return nil, ErrOfferExpired
		}
		p := snap.Participant(offer.ParticipantID)
		if p == nil || p.Status == PDeclined || p.Status == PExpired || p.Status == PRejected {
			return nil, invalid("offer_id", "this supplier is no longer in the negotiation")
		}
		version := n.Version
		if !resuming {
			if err := s.store.Apply(ctx, Change{NegotiationID: id, ExpectedVersion: version, Status: StatusAccepted,
				AcceptedOfferID: offer.ID, OfferStatuses: map[string]string{offer.ID: OfferAccepted}}); err != nil {
				return nil, err
			}
			version++
		}
		// The only way to an order: the runtime's order use case, at the agreed price, idempotent per accepted offer.
		order, err := s.orders.CreateAgreedOrder(ctx, buyerID, p.ListingID, offer.Quantity, offer.UnitMinor, n.Currency, "negotiation:"+n.ID+":"+offer.ID)
		if err != nil {
			if !resuming {
				// Reopen even if the caller has gone away meanwhile.
				undo, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
				defer cancel()
				if uerr := s.store.Apply(undo, Change{NegotiationID: id, ExpectedVersion: version, Status: n.Status, ClearAccepted: true,
					OfferStatuses: map[string]string{offer.ID: OfferOpen}}); uerr != nil {
					return nil, fmt.Errorf("order not created (%w) and negotiation not reopened: %v", err, uerr)
				}
			}
			return nil, err
		}
		c := Change{NegotiationID: id, ExpectedVersion: version, Status: StatusAccepted, AcceptedOfferID: offer.ID, OrderID: order.ID,
			OfferStatuses: map[string]string{offer.ID: OfferAccepted}}
		for _, other := range snap.Participants {
			switch {
			case other.ID == p.ID:
				c.Participants = append(c.Participants, ParticipantUpdate{ID: other.ID, Status: PAccepted, Round: other.Round})
				c.Messages = append(c.Messages, s.notify(ctx, n, other, "accepted", s.acceptedText(n, other, offer.UnitMinor), nil)...)
			case other.Status.Active():
				c.Participants = append(c.Participants, ParticipantUpdate{ID: other.ID, Status: PRejected, Round: other.Round})
				c.Messages = append(c.Messages, s.notify(ctx, n, other, "closed", s.closedText(n, other, true), nil)...)
			}
			if o := snap.OpenOffer(other.ID, AuthorSupplier); o != nil && o.ID != offer.ID {
				c.OfferStatuses[o.ID] = OfferDeclined
			}
			if o := snap.OpenOffer(other.ID, AuthorBuyer); o != nil {
				c.OfferStatuses[o.ID] = OfferSuperseded
			}
		}
		c.Events = append(c.Events, Event{ParticipantID: p.ID, Actor: "buyer", ActorID: buyerID, Kind: "offer.accepted", CreatedAt: now,
			Data: map[string]interface{}{"unit_minor": offer.UnitMinor, "quantity": offer.Quantity, "order_id": order.ID, "supplier": p.SupplierName}})
		if err := s.store.Apply(ctx, c); err != nil {
			return nil, err
		}
		s.record(ctx, buyerID, "negotiation.offer_accepted", id, map[string]interface{}{"offer_id": offer.ID, "order_id": order.ID, "unit_minor": offer.UnitMinor, "currency": n.Currency})
		return s.store.Load(ctx, id)
	})
}

// Reject turns one supplier's offer down; the others go on.
func (s *Service) Reject(ctx context.Context, buyerID, id, participantID string) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, err := s.Get(ctx, buyerID, id)
		if err != nil {
			return nil, err
		}
		if !snap.Negotiation.Status.Open() {
			return nil, ErrClosed
		}
		p := snap.Participant(participantID)
		if p == nil || !p.Status.Active() {
			return nil, invalid("participant_id", "this supplier is no longer in the negotiation")
		}
		now := s.now()
		c := Change{NegotiationID: id, ExpectedVersion: snap.Negotiation.Version, OfferStatuses: map[string]string{}}
		c.Participants = append(c.Participants, ParticipantUpdate{ID: p.ID, Status: PRejected, Round: p.Round})
		if o := snap.OpenOffer(p.ID, AuthorSupplier); o != nil {
			c.OfferStatuses[o.ID] = OfferDeclined
		}
		if o := snap.OpenOffer(p.ID, AuthorBuyer); o != nil {
			c.OfferStatuses[o.ID] = OfferSuperseded
		}
		c.Messages = s.notify(ctx, snap.Negotiation, *p, "closed", s.closedText(snap.Negotiation, *p, false), nil)
		c.Events = []Event{{ParticipantID: p.ID, Actor: "buyer", ActorID: buyerID, Kind: "offer.rejected", CreatedAt: now, Data: map[string]interface{}{"supplier": p.SupplierName}}}
		c.Status = recompute(snap, c, now)
		if c.Status == StatusExpired {
			c.Status = StatusRejected
		}
		if err := s.store.Apply(ctx, c); err != nil {
			return nil, err
		}
		return s.store.Load(ctx, id)
	})
}

// Cancel withdraws the request: every supplier still in it is told.
func (s *Service) Cancel(ctx context.Context, buyerID, id string) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, err := s.Get(ctx, buyerID, id)
		if err != nil {
			return nil, err
		}
		if !snap.Negotiation.Status.Open() {
			return nil, ErrClosed
		}
		now := s.now()
		c := Change{NegotiationID: id, ExpectedVersion: snap.Negotiation.Version, Status: StatusCancelled, OfferStatuses: map[string]string{}}
		for _, p := range snap.Participants {
			if p.Status.Active() {
				c.Participants = append(c.Participants, ParticipantUpdate{ID: p.ID, Status: PRejected, Round: p.Round})
				c.Messages = append(c.Messages, s.notify(ctx, snap.Negotiation, p, "closed", s.closedText(snap.Negotiation, p, false), nil)...)
			}
			for _, author := range []string{AuthorSupplier, AuthorBuyer} {
				if o := snap.OpenOffer(p.ID, author); o != nil {
					c.OfferStatuses[o.ID] = OfferSuperseded
				}
			}
		}
		c.Events = []Event{{Actor: "buyer", ActorID: buyerID, Kind: "negotiation.cancelled", CreatedAt: now}}
		if err := s.store.Apply(ctx, c); err != nil {
			return nil, err
		}
		s.record(ctx, buyerID, "negotiation.cancelled", id, nil)
		return s.store.Load(ctx, id)
	})
}

// ---------------------------------------------------------------- supplier

// supplierRoles may answer for a business.
var supplierRoles = map[domain.OrganizationRole]bool{
	domain.OrganizationRoleOwner: true, domain.OrganizationRoleAdmin: true, domain.OrganizationRoleManager: true, domain.OrganizationRoleSales: true,
}

// Inbox lists the requests sent to the businesses the person answers for. The buyer's ceiling and identity are not in it.
func (s *Service) Inbox(ctx context.Context, userID string) ([]Snapshot, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	members, err := s.members.FindMembershipsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	var orgs []string
	for _, m := range members {
		if supplierRoles[m.Role] {
			orgs = append(orgs, m.OrganizationID)
		}
	}
	if len(orgs) == 0 {
		return []Snapshot{}, nil
	}
	list, err := s.store.ListForOrganizations(ctx, orgs, 100)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, o := range orgs {
		allowed[o] = true
	}
	out := make([]Snapshot, 0, len(list))
	for i := range list {
		for _, p := range list[i].Participants {
			if allowed[p.OrganizationID] {
				out = append(out, ForSupplier(&list[i], p.ID))
			}
		}
	}
	return out, nil
}

// ForSupplier is what one supplier may see of a negotiation: its own request, offers and journal. The buyer's mandate
// (target and ceiling), identity and conversation, the other suppliers and the messages' delivery are left out.
func ForSupplier(snap *Snapshot, participantID string) Snapshot {
	n := snap.Negotiation
	n.BuyerID, n.ConversationID, n.AcceptedOfferID = "", "", ""
	if n.OrderID != "" {
		// The supplier sees the order only when it is theirs.
		mine := false
		for _, o := range snap.Offers {
			if o.ParticipantID == participantID && o.Status == OfferAccepted {
				mine = true
			}
		}
		if !mine {
			n.OrderID = ""
		}
	}
	out := Snapshot{Negotiation: n, Mandate: Mandate{MaxRounds: snap.Mandate.MaxRounds}}
	for _, p := range snap.Participants {
		if p.ID == participantID {
			// The negotiation is titled after the first offer asked: a supplier reads its own.
			out.Negotiation.Title = p.ListingTitle
			out.Participants = append(out.Participants, p)
		}
	}
	for _, o := range snap.Offers {
		if o.ParticipantID == participantID {
			o.CreatedBy = ""
			out.Offers = append(out.Offers, o)
		}
	}
	for _, e := range snap.Events {
		if e.ParticipantID == participantID {
			e.ActorID = ""
			out.Events = append(out.Events, e)
		}
	}
	for _, m := range snap.Messages {
		if m.ParticipantID == participantID && m.Channel == "portal" {
			out.Messages = append(out.Messages, Message{ID: m.ID, Channel: m.Channel, Body: m.Body, CreatedAt: m.CreatedAt})
		}
	}
	return out
}

// supplierSnapshot loads the negotiation of a participant the person answers for.
func (s *Service) supplierSnapshot(ctx context.Context, userID, participantID string) (*Snapshot, *Participant, error) {
	negotiationID, orgID, err := s.store.ParticipantNegotiation(ctx, participantID)
	if err != nil {
		return nil, nil, err
	}
	role, err := s.orgs.GetMemberRole(ctx, orgID, userID)
	if err != nil || !supplierRoles[role] {
		return nil, nil, ErrNotFound
	}
	snap, err := s.store.Load(ctx, negotiationID)
	if err != nil {
		return nil, nil, err
	}
	p := snap.Participant(participantID)
	if p == nil {
		return nil, nil, ErrNotFound
	}
	return snap, p, nil
}

// OfferRequest is a supplier's answer.
type OfferRequest struct {
	UnitPrice    float64 `json:"unit_price"`
	DeliveryDays *int    `json:"delivery_days"`
	ValidDays    int     `json:"valid_days"`
	Note         string  `json:"note"`
}

// SupplierOffer records a supplier's offer; Trezutil may counter at once, within the mandate.
func (s *Service) SupplierOffer(ctx context.Context, userID, participantID string, req OfferRequest) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, p, err := s.supplierSnapshot(ctx, userID, participantID)
		if err != nil {
			return nil, err
		}
		price := toMinor(req.UnitPrice)
		switch {
		case math.IsNaN(req.UnitPrice) || price <= 0 || req.UnitPrice > 1e9:
			return nil, invalid("unit_price", "a positive price")
		case req.DeliveryDays != nil && (*req.DeliveryDays < 0 || *req.DeliveryDays > 365):
			return nil, invalid("delivery_days", "between 0 and 365")
		case req.ValidDays < 0 || req.ValidDays > 60:
			return nil, invalid("valid_days", "between 1 and 60")
		case len([]rune(req.Note)) > 1000:
			return nil, invalid("note", "1000 characters at most")
		}
		return s.supplierView(s.recordSupplierOffer(ctx, snap, p, userID, price, req.DeliveryDays, req.ValidDays, strings.TrimSpace(req.Note), false))(p.ID)
	})
}

// SupplierAcceptCounter answers the buyer's counter-offer with an offer at that price. The buyer still decides.
func (s *Service) SupplierAcceptCounter(ctx context.Context, userID, participantID string) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, p, err := s.supplierSnapshot(ctx, userID, participantID)
		if err != nil {
			return nil, err
		}
		counter := snap.OpenOffer(p.ID, AuthorBuyer)
		if counter == nil {
			return nil, invalid("participant_id", "there is no counter-offer to accept")
		}
		var days *int
		if last := snap.OpenOffer(p.ID, AuthorSupplier); last != nil {
			days = last.DeliveryDays
		}
		return s.supplierView(s.recordSupplierOffer(ctx, snap, p, userID, counter.UnitMinor, days, 0, "", true))(p.ID)
	})
}

func (s *Service) recordSupplierOffer(ctx context.Context, snap *Snapshot, p *Participant, userID string, price int64, days *int, validDays int, note string, acceptsCounter bool) (*Snapshot, error) {
	now := s.now()
	if !snap.Negotiation.Status.Open() {
		return nil, ErrClosed
	}
	if !now.Before(snap.Negotiation.Deadline) {
		return nil, ErrClosed
	}
	if !p.Status.Active() {
		return nil, ErrClosed
	}
	if validDays == 0 {
		validDays = 7
	}
	valid := now.Add(time.Duration(validDays) * 24 * time.Hour)
	offer := Offer{NegotiationID: snap.Negotiation.ID, ParticipantID: p.ID, Round: p.Round, Author: AuthorSupplier, Source: SourcePortal,
		UnitMinor: price, Quantity: snap.Negotiation.Quantity, DeliveryDays: days, ValidUntil: &valid, Note: note, Status: OfferOpen, CreatedBy: userID, CreatedAt: now}
	c := Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: snap.Negotiation.Version, NewOffers: []Offer{offer}, OfferStatuses: map[string]string{}}
	if o := snap.OpenOffer(p.ID, AuthorSupplier); o != nil {
		c.OfferStatuses[o.ID] = OfferSuperseded
	}
	if o := snap.OpenOffer(p.ID, AuthorBuyer); o != nil {
		c.OfferStatuses[o.ID] = OfferSuperseded
	}
	kind := "supplier.offer"
	if acceptsCounter {
		kind = "supplier.accepted_counter"
	}
	c.Events = append(c.Events, Event{ParticipantID: p.ID, Actor: "supplier", ActorID: userID, Kind: kind, CreatedAt: now,
		Data: map[string]interface{}{"unit_minor": price, "round": p.Round, "supplier": p.SupplierName}})
	c.Participants = []ParticipantUpdate{{ID: p.ID, Status: PResponded, Round: p.Round}}

	// Trezutil counters on the buyer's behalf when the mandate allows it (never on an accepted counter-offer).
	if !acceptsCounter {
		if counter, ok := Counter(snap.Mandate, price, p.Round); ok {
			if err := CheckCounter(snap.Negotiation, snap.Mandate, *p, price, counter, true, now); err == nil {
				pending := &Offer{UnitMinor: price}
				cc := s.counterChange(ctx, snap, p, pending, counter, SourceStrategy, "", "", now)
				c.NewOffers = append(c.NewOffers, cc.NewOffers...)
				c.Participants = cc.Participants
				c.Events = append(c.Events, cc.Events...)
				c.Messages = append(c.Messages, cc.Messages...)
			}
		}
	}
	c.Status = recompute(snap, c, now)
	if err := s.store.Apply(ctx, c); err != nil {
		return nil, err
	}
	return s.store.Load(ctx, snap.Negotiation.ID)
}

// SupplierDecline: the supplier will not make an offer (or withdraws theirs).
func (s *Service) SupplierDecline(ctx context.Context, userID, participantID, reason string) (*Snapshot, error) {
	return s.retry(ctx, func() (*Snapshot, error) {
		snap, p, err := s.supplierSnapshot(ctx, userID, participantID)
		if err != nil {
			return nil, err
		}
		if !snap.Negotiation.Status.Open() || !p.Status.Active() {
			return nil, ErrClosed
		}
		now := s.now()
		c := Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: snap.Negotiation.Version, OfferStatuses: map[string]string{}}
		c.Participants = []ParticipantUpdate{{ID: p.ID, Status: PDeclined, Round: p.Round}}
		for _, author := range []string{AuthorSupplier, AuthorBuyer} {
			if o := snap.OpenOffer(p.ID, author); o != nil {
				c.OfferStatuses[o.ID] = OfferSuperseded
			}
		}
		reason = strings.TrimSpace(reason)
		if len([]rune(reason)) > 300 {
			reason = string([]rune(reason)[:300])
		}
		c.Events = []Event{{ParticipantID: p.ID, Actor: "supplier", ActorID: userID, Kind: "supplier.declined", CreatedAt: now, Data: map[string]interface{}{"reason": reason, "supplier": p.SupplierName}}}
		c.Status = recompute(snap, c, now)
		if err := s.store.Apply(ctx, c); err != nil {
			return nil, err
		}
		return s.supplierView(s.store.Load(ctx, snap.Negotiation.ID))(p.ID)
	})
}

// supplierView turns a result into what the supplier may see.
func (s *Service) supplierView(snap *Snapshot, err error) func(participantID string) (*Snapshot, error) {
	return func(participantID string) (*Snapshot, error) {
		if err != nil {
			return nil, err
		}
		view := ForSupplier(snap, participantID)
		return &view, nil
	}
}

// SupplierGet returns one request as the supplier sees it.
func (s *Service) SupplierGet(ctx context.Context, userID, participantID string) (*Snapshot, error) {
	snap, p, err := s.supplierSnapshot(ctx, userID, participantID)
	if err != nil {
		return nil, err
	}
	view := ForSupplier(snap, p.ID)
	return &view, nil
}

// ---------------------------------------------------------------- time

// Tick closes what reached its deadline and reminds suppliers who have not answered halfway through.
func (s *Service) Tick(ctx context.Context) (int, error) {
	now := s.now()
	done := 0
	ids, err := s.store.DueDeadlines(ctx, now, 50)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := s.retry(ctx, func() (*Snapshot, error) { return nil, s.expire(ctx, id, now) }); err == nil {
			done++
		}
	}
	ids, err = s.store.DueReminders(ctx, now, 50)
	if err != nil {
		return done, err
	}
	for _, id := range ids {
		if _, err := s.retry(ctx, func() (*Snapshot, error) { return nil, s.remind(ctx, id, now) }); err == nil {
			done++
		}
	}
	return done, nil
}

func (s *Service) expire(ctx context.Context, id string, now time.Time) error {
	snap, err := s.store.Load(ctx, id)
	if err != nil {
		return err
	}
	n := snap.Negotiation
	if !n.Status.Open() || now.Before(n.Deadline) {
		return nil
	}
	c := Change{NegotiationID: id, ExpectedVersion: n.Version, OfferStatuses: map[string]string{}}
	open := false
	for _, p := range snap.Participants {
		if p.Status == PInvited || p.Status == PCountered {
			c.Participants = append(c.Participants, ParticipantUpdate{ID: p.ID, Status: PExpired, Round: p.Round})
			if o := snap.OpenOffer(p.ID, AuthorBuyer); o != nil {
				c.OfferStatuses[o.ID] = OfferSuperseded
			}
		}
		if p.Status.Active() && snap.OpenOffer(p.ID, AuthorSupplier) != nil {
			open = true
		}
	}
	// With an offer in hand the buyer keeps a week to decide; after that, or with none, it ends.
	switch {
	case open && now.Before(n.Deadline.Add(7*24*time.Hour)):
		c.Status = StatusAwaitingDecision
	default:
		c.Status = StatusExpired
		for _, p := range snap.Participants {
			if p.Status.Active() {
				c.Participants = append(c.Participants, ParticipantUpdate{ID: p.ID, Status: PExpired, Round: p.Round})
			}
			if o := snap.OpenOffer(p.ID, AuthorSupplier); o != nil {
				c.OfferStatuses[o.ID] = OfferSuperseded
			}
		}
	}
	if c.Status == n.Status && len(c.Participants) == 0 {
		return nil
	}
	c.Events = []Event{{Actor: "system", Kind: "deadline.reached", CreatedAt: now, Data: map[string]interface{}{"status": string(c.Status)}}}
	return s.store.Apply(ctx, c)
}

func (s *Service) remind(ctx context.Context, id string, now time.Time) error {
	snap, err := s.store.Load(ctx, id)
	if err != nil {
		return err
	}
	n := snap.Negotiation
	if !n.Status.Open() || !now.Before(n.Deadline) {
		return nil
	}
	half := n.CreatedAt.Add(n.Deadline.Sub(n.CreatedAt) / 2)
	c := Change{NegotiationID: id, ExpectedVersion: n.Version, Status: n.Status}
	for _, p := range snap.Participants {
		if (p.Status == PInvited || p.Status == PCountered) && p.RemindedAt == nil && !now.Before(half) {
			at := now
			c.Participants = append(c.Participants, ParticipantUpdate{ID: p.ID, Status: p.Status, Round: p.Round, RemindedAt: &at})
			c.Messages = append(c.Messages, s.notify(ctx, n, p, fmt.Sprintf("reminder-%d", p.Round), s.reminderText(n, p), nil)...)
			c.Events = append(c.Events, Event{ParticipantID: p.ID, Actor: "system", Kind: "supplier.reminded", CreatedAt: now, Data: map[string]interface{}{"supplier": p.SupplierName}})
		}
	}
	if len(c.Participants) == 0 {
		return nil
	}
	return s.store.Apply(ctx, c)
}

// ---------------------------------------------------------------- shared

// counterChange builds a counter-offer to a supplier: the offer, the participant's next round, the message, the event.
func (s *Service) counterChange(ctx context.Context, snap *Snapshot, p *Participant, supplierOffer *Offer, price int64, source, actorID, note string, now time.Time) Change {
	c := Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: snap.Negotiation.Version, Status: StatusNegotiating, OfferStatuses: map[string]string{}}
	if o := snap.OpenOffer(p.ID, AuthorBuyer); o != nil {
		c.OfferStatuses[o.ID] = OfferSuperseded
	}
	c.NewOffers = []Offer{{NegotiationID: snap.Negotiation.ID, ParticipantID: p.ID, Round: p.Round, Author: AuthorBuyer, Source: source,
		UnitMinor: price, Quantity: snap.Negotiation.Quantity, Note: note, Status: OfferOpen, CreatedBy: actorID, CreatedAt: now}}
	c.Participants = []ParticipantUpdate{{ID: p.ID, Status: PCountered, Round: p.Round + 1}}
	actor := "system"
	if source == SourceBuyer {
		actor = "buyer"
	}
	c.Events = []Event{{ParticipantID: p.ID, Actor: actor, ActorID: actorID, Kind: "counter.sent", CreatedAt: now,
		Data: map[string]interface{}{"unit_minor": price, "round": p.Round, "by": source, "supplier": p.SupplierName}}}
	text := s.counterText(snap.Negotiation, *p, price, supplierOffer.UnitMinor)
	if CheckOutgoing(text, snap.Mandate, price, supplierOffer.UnitMinor, p.ListUnitMinor) == nil {
		c.Messages = s.notify(ctx, snap.Negotiation, *p, fmt.Sprintf("counter-%d", p.Round), text, nil)
	}
	return c
}

// recompute is the negotiation's status once a change is applied.
func recompute(snap *Snapshot, c Change, now time.Time) Status {
	status := map[string]ParticipantStatus{}
	for _, p := range snap.Participants {
		status[p.ID] = p.Status
	}
	for _, u := range c.Participants {
		status[u.ID] = u.Status
	}
	offerStatus := map[string]string{}
	for _, o := range snap.Offers {
		offerStatus[o.ID] = o.Status
	}
	for id, st := range c.OfferStatuses {
		offerStatus[id] = st
	}
	openSupplier := map[string]int64{}
	for _, o := range snap.Offers {
		if o.Author == AuthorSupplier && offerStatus[o.ID] == OfferOpen {
			openSupplier[o.ParticipantID] = o.UnitMinor
		}
	}
	for _, o := range c.NewOffers {
		if o.Author == AuthorSupplier {
			openSupplier[o.ParticipantID] = o.UnitMinor
		}
	}
	waiting, good, any := false, false, false
	for id, st := range status {
		if st == PInvited || st == PCountered {
			waiting = true
		}
		if price, ok := openSupplier[id]; ok && st.Active() {
			any = true
			if Satisfies(snap.Mandate, price) {
				good = true
			}
		}
	}
	switch {
	case good || (any && !waiting):
		return StatusAwaitingDecision
	case !waiting && !any:
		return StatusExpired
	case any || len(openSupplier) > 0:
		return StatusNegotiating
	}
	if snap.Negotiation.Status == StatusNegotiating {
		return StatusNegotiating
	}
	return StatusSoliciting
}

// retry runs a use case again once when the negotiation changed meanwhile (two suppliers answering at once).
func (s *Service) retry(ctx context.Context, fn func() (*Snapshot, error)) (*Snapshot, error) {
	snap, err := fn()
	if errors.Is(err, ErrConflict) {
		snap, err = fn()
	}
	return snap, err
}

func (s *Service) record(ctx context.Context, actorID, action, id string, meta map[string]interface{}) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: action, TargetType: "negotiation", TargetID: id, Metadata: meta, OccurredAt: s.now().Unix()})
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
