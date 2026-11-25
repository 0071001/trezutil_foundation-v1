// Package negotiation is the negotiation engine, phase 1 (docs/architecture/NEGOTIATION_ENGINE.md).
//
// A buyer asks the suppliers of one to five published offers for their best price, within a mandate: a target unit
// price, a ceiling that never leaves the buyer's side, a number of rounds and a deadline. Suppliers answer in their
// portal. Trezutil may counter on the buyer's behalf, inside the mandate only (Strategy proposes, Guard decides). The
// buyer alone accepts an offer, which creates an order at the agreed price: no supplier answer ever creates an order
// or a payment. Everything that happens is written in the negotiation's journal.
package negotiation

import (
	"errors"
	"time"
)

// Status of a negotiation.
type Status string

const (
	StatusSoliciting       Status = "soliciting"        // requests sent, no answer yet
	StatusNegotiating      Status = "negotiating"       // answers and counter-offers are being exchanged
	StatusAwaitingDecision Status = "awaiting_decision" // an offer meets the target, or nothing more can be gained
	StatusAccepted         Status = "accepted"          // the buyer accepted an offer; an order exists
	StatusRejected         Status = "rejected"          // the buyer turned every offer down
	StatusExpired          Status = "expired"           // the deadline passed with no offer
	StatusCancelled        Status = "cancelled"         // the buyer withdrew the request
)

// Open reports whether the negotiation can still change.
func (s Status) Open() bool {
	return s == StatusSoliciting || s == StatusNegotiating || s == StatusAwaitingDecision
}

// ParticipantStatus is where one supplier stands.
type ParticipantStatus string

const (
	PInvited   ParticipantStatus = "invited"
	PResponded ParticipantStatus = "responded"
	PCountered ParticipantStatus = "countered"
	PDeclined  ParticipantStatus = "declined"
	PAccepted  ParticipantStatus = "accepted"
	PRejected  ParticipantStatus = "rejected"
	PExpired   ParticipantStatus = "expired"
)

// Active reports whether the supplier can still answer.
func (s ParticipantStatus) Active() bool { return s == PInvited || s == PResponded || s == PCountered }

// Offer authors and sources.
const (
	AuthorSupplier = "supplier"
	AuthorBuyer    = "buyer"

	SourcePortal   = "portal"
	SourceBuyer    = "buyer"
	SourceStrategy = "strategy"

	OfferOpen       = "open"
	OfferSuperseded = "superseded"
	OfferAccepted   = "accepted"
	OfferDeclined   = "declined"
)

// Negotiation is the request as a whole.
type Negotiation struct {
	ID              string
	BuyerID         string
	ConversationID  string
	Title           string
	Quantity        int
	Currency        string
	Status          Status
	Deadline        time.Time
	AcceptedOfferID string
	OrderID         string
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Mandate is what the buyer allows Trezutil to do. MaxUnitMinor is the buyer's secret.
type Mandate struct {
	Version         int
	TargetUnitMinor int64
	MaxUnitMinor    int64
	MaxRounds       int
	AutoCounter     bool
	GrantedBy       string
	CreatedAt       time.Time
}

// Participant is one supplier offer asked.
type Participant struct {
	ID             string
	NegotiationID  string
	OrganizationID string
	SupplierName   string
	ListingID      string
	ListingTitle   string
	ListUnitMinor  int64
	Status         ParticipantStatus
	Round          int
	RemindedAt     *time.Time
	LastActivityAt time.Time
}

// Offer is an offer or a counter-offer.
type Offer struct {
	ID            string
	NegotiationID string
	ParticipantID string
	Round         int
	Author        string
	Source        string
	UnitMinor     int64
	Quantity      int
	DeliveryDays  *int
	ValidUntil    *time.Time
	Note          string
	Status        string
	CreatedBy     string
	CreatedAt     time.Time
}

// Message is a notification to a supplier (outbox) or, later, an answer received.
type Message struct {
	ID             string
	NegotiationID  string
	ParticipantID  string
	Channel        string // portal | whatsapp
	Direction      string // out | in
	Body           string
	Recipient      string
	ExternalID     string
	IdempotencyKey string
	State          string // pending | sent | delivered | failed | skipped
	Attempts       int
	LastError      string
	CreatedAt      time.Time
}

// Event is one line of the journal.
type Event struct {
	ID            int64
	NegotiationID string
	ParticipantID string
	Actor         string // buyer | supplier | system
	ActorID       string
	Kind          string
	Data          map[string]interface{}
	CreatedAt     time.Time
}

// Snapshot is a negotiation with everything it holds.
type Snapshot struct {
	Negotiation  Negotiation
	Mandate      Mandate
	Participants []Participant
	Offers       []Offer
	Events       []Event
	Messages     []Message
}

// Participant returns the participant with this id.
func (s *Snapshot) Participant(id string) *Participant {
	for i := range s.Participants {
		if s.Participants[i].ID == id {
			return &s.Participants[i]
		}
	}
	return nil
}

// OpenOffer returns the open offer of an author for a participant.
func (s *Snapshot) OpenOffer(participantID, author string) *Offer {
	for i := len(s.Offers) - 1; i >= 0; i-- {
		o := &s.Offers[i]
		if o.ParticipantID == participantID && o.Author == author && o.Status == OfferOpen {
			return o
		}
	}
	return nil
}

// ParticipantUpdate changes one participant.
type ParticipantUpdate struct {
	ID         string
	Status     ParticipantStatus
	Round      int
	RemindedAt *time.Time
}

// Change is what one use case writes, atomically, provided the negotiation is still at ExpectedVersion.
type Change struct {
	NegotiationID   string
	ExpectedVersion int
	Status          Status
	AcceptedOfferID string
	// ClearAccepted forgets the accepted offer (its order could not be created).
	ClearAccepted bool
	OrderID       string
	Participants  []ParticipantUpdate
	NewOffers     []Offer
	OfferStatuses map[string]string
	Events        []Event
	Messages      []Message
}

// Errors.
var (
	ErrNotFound      = errors.New("negotiation not found")
	ErrForbidden     = errors.New("not allowed")
	ErrConflict      = errors.New("the negotiation changed meanwhile")
	ErrClosed        = errors.New("the negotiation is closed")
	ErrDemoSupplier  = errors.New("demonstration suppliers do not negotiate")
	ErrOwnOffer      = errors.New("you cannot negotiate with your own business")
	ErrMixedCurrency = errors.New("every offer of a negotiation must be in the same currency")
	ErrUnavailable   = errors.New("the offer is not published")
	ErrOfferExpired  = errors.New("this offer is no longer valid")
)

// InvalidError is a field the caller must correct.
type InvalidError struct {
	Field   string
	Message string
}

func (e *InvalidError) Error() string { return e.Field + ": " + e.Message }

func invalid(field, msg string) error { return &InvalidError{Field: field, Message: msg} }
