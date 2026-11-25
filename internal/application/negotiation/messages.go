package negotiation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * What suppliers are told. Every message is written by this code from figures the strategy or the buyer fixed (no
 * model), passes the guard, and is kept: in the supplier's portal always, and on WhatsApp when an approved template is
 * configured (Meta delivers only templates to someone who has not written in the last 24 hours). Each message says it
 * is sent by Trezutil on behalf of a buyer, and never names the buyer.
 */

var kinshasa = func() *time.Location {
	if loc, err := time.LoadLocation("Africa/Kinshasa"); err == nil {
		return loc
	}
	return time.FixedZone("WAT", 3600)
}()

// Money writes an amount in minor units the way a supplier reads it: "45,00 USD", "42 000 FC".
func Money(minor int64, currency string) string {
	if currency == "CDF" {
		return groupThousands(minor/100, " ") + " FC"
	}
	return fmt.Sprintf("%s,%02d %s", groupThousands(minor/100, " "), minor%100, currency)
}

func when(t time.Time) string {
	t = t.In(kinshasa)
	return fmt.Sprintf("%02d/%02d à %02dh%02d", t.Day(), int(t.Month()), t.Hour(), t.Minute())
}

func (s *Service) link() string {
	if s.cfg.PortalURL == "" {
		return ""
	}
	return " " + s.cfg.PortalURL
}

func (s *Service) requestText(n Negotiation, p Participant) string {
	return fmt.Sprintf("Trezutil, pour le compte d'un acheteur, vous demande votre meilleur prix pour %d × « %s » (prix affiché : %s). Répondez avant le %s dans votre espace fournisseur :%s",
		n.Quantity, p.ListingTitle, Money(p.ListUnitMinor, n.Currency), when(n.Deadline), s.link())
}

func (s *Service) counterText(n Negotiation, p Participant, counter, offer int64) string {
	return fmt.Sprintf("Trezutil, pour le compte de l'acheteur : contre-proposition à %s l'unité pour %d × « %s » (votre offre : %s). Acceptez-la ou faites une nouvelle offre avant le %s :%s",
		Money(counter, n.Currency), n.Quantity, p.ListingTitle, Money(offer, n.Currency), when(n.Deadline), s.link())
}

func (s *Service) reminderText(n Negotiation, p Participant) string {
	return fmt.Sprintf("Rappel Trezutil : la demande de prix pour « %s » attend votre réponse avant le %s :%s", p.ListingTitle, when(n.Deadline), s.link())
}

func (s *Service) acceptedText(n Negotiation, p Participant, unit int64) string {
	return fmt.Sprintf("Bonne nouvelle : l'acheteur a retenu votre offre à %s l'unité pour %d × « %s ». La commande est dans votre espace fournisseur :%s",
		Money(unit, n.Currency), n.Quantity, p.ListingTitle, s.link())
}

func (s *Service) closedText(n Negotiation, p Participant, otherChosen bool) string {
	if otherChosen {
		return fmt.Sprintf("Trezutil : l'acheteur a retenu une autre offre pour « %s ». Merci pour votre réponse.", p.ListingTitle)
	}
	return fmt.Sprintf("Trezutil : la demande de prix pour « %s » est close. Merci pour votre réponse.", p.ListingTitle)
}

// notify keeps a message in the supplier's portal and, when WhatsApp is on and the business has a phone, queues it for
// WhatsApp. The idempotency key makes the same notification go out once.
func (s *Service) notify(ctx context.Context, n Negotiation, p Participant, purpose, text string, org *domain.Organization) []Message {
	key := fmt.Sprintf("neg:%s:%s:%s", n.ID, p.ID, purpose)
	now := s.now()
	out := []Message{{NegotiationID: n.ID, ParticipantID: p.ID, Channel: "portal", Direction: "out", Body: text, IdempotencyKey: key + ":portal", State: "delivered", CreatedAt: now}}
	if !s.cfg.WhatsApp {
		return out
	}
	if org == nil {
		org, _ = s.orgs.FindByID(ctx, p.OrganizationID)
	}
	phone := ""
	if org != nil {
		phone = normalizePhone(org.Phone)
	}
	m := Message{NegotiationID: n.ID, ParticipantID: p.ID, Channel: "whatsapp", Direction: "out", Body: text, Recipient: phone, IdempotencyKey: key + ":whatsapp", State: "pending", CreatedAt: now}
	if phone == "" {
		m.State, m.LastError = "skipped", "the business has no phone number"
	}
	return append(out, m)
}

// normalizePhone writes a Congolese number in international form: 0821714747 → +243821714747.
func normalizePhone(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' || (r == '+' && b.Len() == 0) {
			b.WriteRune(r)
		}
	}
	p := b.String()
	switch {
	case strings.HasPrefix(p, "+"):
	case strings.HasPrefix(p, "00"):
		p = "+" + p[2:]
	case strings.HasPrefix(p, "243"):
		p = "+" + p
	case strings.HasPrefix(p, "0") && len(p) == 10:
		p = "+243" + p[1:]
	default:
		return ""
	}
	if len(p) < 9 || len(p) > 16 {
		return ""
	}
	return p
}

// ---------------------------------------------------------------- outbox

// Outbox is where the messages waiting to be sent are kept.
type Outbox interface {
	PendingMessages(ctx context.Context, limit int) ([]Message, error)
	// MarkMessage records an attempt: sent (with the provider's id), failed for good, or pending again (retry).
	MarkMessage(ctx context.Context, id, state, lastError, externalID string) error
}

// TemplateSender sends an approved WhatsApp template.
type TemplateSender interface {
	SendTemplate(ctx context.Context, to, name, language string, params []string) (string, error)
}

// Deliverer sends the WhatsApp messages of the outbox, each once.
type Deliverer struct {
	outbox   Outbox
	sender   TemplateSender
	template string
	language string
	logger   zerolog.Logger
}

// NewDeliverer builds the deliverer. Without a sender or an approved template, WhatsApp messages are marked skipped
// with the reason: nothing pretends they were sent.
func NewDeliverer(outbox Outbox, sender TemplateSender, template, language string, logger zerolog.Logger) *Deliverer {
	if language == "" {
		language = "fr"
	}
	return &Deliverer{outbox: outbox, sender: sender, template: template, language: language, logger: logger}
}

const maxAttempts = 5

// Run sends what is waiting; it returns how many messages were sent.
func (d *Deliverer) Run(ctx context.Context) (int, error) {
	msgs, err := d.outbox.PendingMessages(ctx, 20)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, m := range msgs {
		switch {
		case m.Channel != "whatsapp":
			_ = d.outbox.MarkMessage(ctx, m.ID, "delivered", "", "")
		case d.sender == nil || d.template == "":
			_ = d.outbox.MarkMessage(ctx, m.ID, "skipped", "WhatsApp is not configured for suppliers (no approved template)", "")
		case m.Recipient == "":
			_ = d.outbox.MarkMessage(ctx, m.ID, "skipped", "no phone number", "")
		default:
			body := m.Body
			if r := []rune(body); len(r) > 1000 {
				body = string(r[:1000])
			}
			id, err := d.sender.SendTemplate(ctx, m.Recipient, d.template, d.language, []string{body})
			if err != nil {
				state := "pending"
				if m.Attempts+1 >= maxAttempts {
					state = "failed"
				}
				d.logger.Warn().Err(err).Str("message_id", m.ID).Int("attempt", m.Attempts+1).Msg("negotiation WhatsApp message not sent")
				_ = d.outbox.MarkMessage(ctx, m.ID, state, truncate(err.Error(), 480), "")
				continue
			}
			_ = d.outbox.MarkMessage(ctx, m.ID, "sent", "", id)
			sent++
		}
	}
	return sent, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
