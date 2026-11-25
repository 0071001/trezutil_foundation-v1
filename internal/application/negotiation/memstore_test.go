package negotiation

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// memStore is an in-memory Store and Outbox for the tests.
type memStore struct {
	mu   sync.Mutex
	seq  int
	snap map[string]*Snapshot
	keys map[string]bool
}

func newMemStore() *memStore { return &memStore{snap: map[string]*Snapshot{}, keys: map[string]bool{}} }

func (m *memStore) id(prefix string) string { m.seq++; return fmt.Sprintf("%s-%d", prefix, m.seq) }

func clone(s *Snapshot) *Snapshot {
	c := *s
	c.Participants = append([]Participant(nil), s.Participants...)
	c.Offers = append([]Offer(nil), s.Offers...)
	c.Events = append([]Event(nil), s.Events...)
	c.Messages = append([]Message(nil), s.Messages...)
	return &c
}

func (m *memStore) Create(ctx context.Context, s *Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.Negotiation.ID = m.id("neg")
	for i := range s.Participants {
		s.Participants[i].ID = m.id("part")
		s.Participants[i].NegotiationID = s.Negotiation.ID
	}
	for i := range s.Events {
		s.Events[i].NegotiationID = s.Negotiation.ID
	}
	m.snap[s.Negotiation.ID] = clone(s)
	return nil
}

func (m *memStore) Load(ctx context.Context, id string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.snap[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(s), nil
}

func (m *memStore) Apply(ctx context.Context, c Change) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.snap[c.NegotiationID]
	if !ok {
		return ErrNotFound
	}
	if s.Negotiation.Version != c.ExpectedVersion {
		return ErrConflict
	}
	s.Negotiation.Version++
	if c.Status != "" {
		s.Negotiation.Status = c.Status
	}
	if c.AcceptedOfferID != "" {
		s.Negotiation.AcceptedOfferID = c.AcceptedOfferID
	}
	if c.ClearAccepted {
		s.Negotiation.AcceptedOfferID = ""
	}
	if c.OrderID != "" {
		s.Negotiation.OrderID = c.OrderID
	}
	for _, u := range c.Participants {
		for i := range s.Participants {
			if s.Participants[i].ID == u.ID {
				s.Participants[i].Status, s.Participants[i].Round = u.Status, u.Round
				if u.RemindedAt != nil {
					s.Participants[i].RemindedAt = u.RemindedAt
				}
			}
		}
	}
	for id, st := range c.OfferStatuses {
		for i := range s.Offers {
			if s.Offers[i].ID == id {
				s.Offers[i].Status = st
			}
		}
	}
	for _, o := range c.NewOffers {
		o.ID = m.id("offer")
		s.Offers = append(s.Offers, o)
	}
	for _, e := range c.Events {
		e.NegotiationID = c.NegotiationID
		s.Events = append(s.Events, e)
	}
	for _, msg := range c.Messages {
		if m.keys[msg.IdempotencyKey] {
			continue
		}
		m.keys[msg.IdempotencyKey] = true
		msg.ID = m.id("msg")
		s.Messages = append(s.Messages, msg)
	}
	return nil
}

func (m *memStore) ListForBuyer(ctx context.Context, buyerID string, limit int) ([]Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Snapshot{}
	for _, s := range m.snap {
		if s.Negotiation.BuyerID == buyerID {
			out = append(out, *clone(s))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Negotiation.ID > out[j].Negotiation.ID })
	return out, nil
}

func (m *memStore) ListForOrganizations(ctx context.Context, orgIDs []string, limit int) ([]Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	in := map[string]bool{}
	for _, o := range orgIDs {
		in[o] = true
	}
	out := []Snapshot{}
	for _, s := range m.snap {
		for _, p := range s.Participants {
			if in[p.OrganizationID] {
				out = append(out, *clone(s))
				break
			}
		}
	}
	return out, nil
}

func (m *memStore) ParticipantNegotiation(ctx context.Context, participantID string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.snap {
		for _, p := range s.Participants {
			if p.ID == participantID {
				return s.Negotiation.ID, p.OrganizationID, nil
			}
		}
	}
	return "", "", ErrNotFound
}

func (m *memStore) DueDeadlines(ctx context.Context, now time.Time, limit int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for id, s := range m.snap {
		n := s.Negotiation
		if (n.Status == StatusSoliciting || n.Status == StatusNegotiating) && !now.Before(n.Deadline) ||
			n.Status == StatusAwaitingDecision && !now.Before(n.Deadline.Add(7*24*time.Hour)) {
			out = append(out, id)
		}
	}
	return out, nil
}

func (m *memStore) DueReminders(ctx context.Context, now time.Time, limit int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for id, s := range m.snap {
		if !s.Negotiation.Status.Open() || !now.Before(s.Negotiation.Deadline) {
			continue
		}
		for _, p := range s.Participants {
			if (p.Status == PInvited || p.Status == PCountered) && p.RemindedAt == nil {
				out = append(out, id)
				break
			}
		}
	}
	return out, nil
}

func (m *memStore) PendingMessages(ctx context.Context, limit int) ([]Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []Message{}
	for _, s := range m.snap {
		for _, msg := range s.Messages {
			if msg.State == "pending" {
				out = append(out, msg)
			}
		}
	}
	return out, nil
}

func (m *memStore) MarkMessage(ctx context.Context, id, state, lastError, externalID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.snap {
		for i := range s.Messages {
			if s.Messages[i].ID == id {
				s.Messages[i].State, s.Messages[i].LastError, s.Messages[i].ExternalID = state, lastError, externalID
				s.Messages[i].Attempts++
			}
		}
	}
	return nil
}

// ---- collaborators

type fakeListings map[string]*domain.Listing

func (f fakeListings) FindByID(ctx context.Context, id string) (*domain.Listing, error) {
	if l, ok := f[id]; ok {
		return l, nil
	}
	return nil, domain.ErrListingNotFound
}

type fakeOrgs struct {
	orgs    map[string]*domain.Organization
	members map[string]domain.OrganizationRole // org|user → role
}

func (f fakeOrgs) FindByID(ctx context.Context, id string) (*domain.Organization, error) {
	if o, ok := f.orgs[id]; ok {
		return o, nil
	}
	return nil, domain.ErrOrganizationNotFound
}

func (f fakeOrgs) GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrganizationRole, error) {
	if r, ok := f.members[orgID+"|"+userID]; ok {
		return r, nil
	}
	return "", domain.ErrNotMember
}

func (f fakeOrgs) FindMembershipsByUserID(ctx context.Context, userID string) ([]domain.OrganizationMember, error) {
	out := []domain.OrganizationMember{}
	for k, r := range f.members {
		var org, user string
		for i := 0; i < len(k); i++ {
			if k[i] == '|' {
				org, user = k[:i], k[i+1:]
			}
		}
		if user == userID {
			out = append(out, domain.OrganizationMember{OrganizationID: org, UserID: user, Role: r, Status: domain.MembershipStatusActive})
		}
	}
	return out, nil
}

type agreedOrder struct {
	actor, listing, currency, key string
	quantity                      int
	unit                          int64
}

type fakeOrders struct {
	made []agreedOrder
	hook func() error // runs while the order is being written
}

func (f *fakeOrders) CreateAgreedOrder(ctx context.Context, actorID, listingID string, quantity int, unitMinor int64, currency, key string) (*domain.Order, error) {
	if f.hook != nil {
		if err := f.hook(); err != nil {
			return nil, err
		}
	}
	f.made = append(f.made, agreedOrder{actorID, listingID, currency, key, quantity, unitMinor})
	return &domain.Order{ID: "order-1"}, nil
}
