package domain

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The public site's managed content: the home page's trust strip, partner band
// and section order. Staff edit it in the administration console; the public
// site reads it from GET /api/v1/site/content.
//
// Two rules shape this file:
//   - A figure is never stored. The trust strip stores *which* platform metrics
//     to show and how; the number behind each is computed from the platform's
//     own tables on every read (SiteStats), so it cannot be made up.
//   - Everything an editor can type is validated here, not in the database or
//     the browser: lengths, the set of metrics and sections, and where a logo
//     may come from.

// SiteMetric names a platform figure the trust strip can show.
type SiteMetric string

const (
	// What Trezutil enables, not how much it lists: the figures the home page's trust strip shows by default.
	SiteMetricConversations SiteMetric = "conversations" // conversations people started to say what they need
	SiteMetricConnections   SiteMetric = "connections"   // customer and business pairs that went through with an order or a booking
	SiteMetricTransactions  SiteMetric = "transactions"  // orders completed plus bookings confirmed
	SiteMetricMerchants     SiteMetric = "merchants"     // active businesses with at least one published listing

	// Also available to staff, off by default.
	SiteMetricRegisteredUsers SiteMetric = "registered_users" // active accounts that shop or sell (never the staff)
	SiteMetricPartners        SiteMetric = "partners"         // partners shown in the partner band (counted from the managed list)

	SiteMetricLiveListings      SiteMetric = "live_listings"
	SiteMetricVerifiedSellers   SiteMetric = "verified_sellers"
	SiteMetricDealsLive         SiteMetric = "deals_live"
	SiteMetricCitiesCovered     SiteMetric = "cities_covered"
	SiteMetricBookingsConfirmed SiteMetric = "bookings_confirmed"
	SiteMetricOrdersCompleted   SiteMetric = "orders_completed"
	SiteMetricVerticalsLive     SiteMetric = "verticals_live"
)

// SiteMetrics is the catalogue of metrics, in the order the editor lists them.
var SiteMetrics = []SiteMetric{
	SiteMetricConversations, SiteMetricConnections, SiteMetricTransactions, SiteMetricMerchants,
	SiteMetricRegisteredUsers, SiteMetricPartners,
	SiteMetricLiveListings, SiteMetricVerifiedSellers, SiteMetricDealsLive, SiteMetricCitiesCovered,
	SiteMetricBookingsConfirmed, SiteMetricOrdersCompleted, SiteMetricVerticalsLive,
}

// Valid reports whether m is a metric the platform can compute.
func (m SiteMetric) Valid() bool {
	for _, k := range SiteMetrics {
		if k == m {
			return true
		}
	}
	return false
}

// SiteStats holds the live value of every metric.
type SiteStats map[SiteMetric]int

// WithPartnerCount returns the figures plus the number of partners the public
// site is showing. That one figure is counted from the managed list itself (the
// partners are content, not platform activity), so it follows what staff publish
// and cannot say more than the band shows.
func (c *SiteContent) WithPartnerCount(stats SiteStats) SiteStats {
	out := make(SiteStats, len(stats)+1)
	for metric, value := range stats {
		out[metric] = value
	}
	shown := 0
	for _, p := range c.Partners {
		if p.Visible {
			shown++
		}
	}
	out[SiteMetricPartners] = shown
	return out
}

// TrustItem is one entry of the trust strip.
type TrustItem struct {
	Metric  SiteMetric `json:"metric"`
	Enabled bool       `json:"enabled"`
	// MinValue hides the item on the public site until its figure reaches it, so
	// a young platform never advertises a number that is still embarrassing.
	MinValue int `json:"min_value"`
	// LabelFR and LabelEN replace the default wording when set.
	LabelFR string `json:"label_fr,omitempty"`
	LabelEN string `json:"label_en,omitempty"`
}

// PartnerKind groups partners in the editor.
type PartnerKind string

const (
	PartnerBank        PartnerKind = "bank"
	PartnerMobileMoney PartnerKind = "mobile_money"
	PartnerShop        PartnerKind = "shop"
	PartnerHotel       PartnerKind = "hotel"
	PartnerHub         PartnerKind = "hub"
	PartnerTech        PartnerKind = "tech"
	PartnerOther       PartnerKind = "other"
)

// Valid reports whether k is a known partner kind.
func (k PartnerKind) Valid() bool {
	switch k {
	case PartnerBank, PartnerMobileMoney, PartnerShop, PartnerHotel, PartnerHub, PartnerTech, PartnerOther:
		return true
	}
	return false
}

// Partner is one logo in the partner band.
type Partner struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Kind    PartnerKind `json:"kind"`
	LogoURL string      `json:"logo_url"`
	// LinkURL is an optional https page the logo opens.
	LinkURL string `json:"link_url,omitempty"`
	Visible bool   `json:"visible"`
}

// HomeSection names a block of the home page that staff can hide or reorder.
// The hero is always first and cannot be hidden.
type HomeSection string

const (
	SectionTrust        HomeSection = "trust"
	SectionNearby       HomeSection = "nearby"
	SectionCategories   HomeSection = "categories"
	SectionShowcase     HomeSection = "showcase"
	SectionServices     HomeSection = "services"
	SectionProperties   HomeSection = "properties"
	SectionReservations HomeSection = "reservations"
	SectionWhatsApp     HomeSection = "whatsapp"
	SectionHow          HomeSection = "how"
	SectionSeller       HomeSection = "seller"
	SectionPartners     HomeSection = "partners"
)

// HomeSections lists every section in its default order.
var HomeSections = []HomeSection{
	SectionTrust, SectionNearby, SectionCategories, SectionShowcase, SectionServices,
	SectionProperties, SectionReservations, SectionWhatsApp, SectionHow, SectionSeller, SectionPartners,
}

// Valid reports whether s is a known section.
func (s HomeSection) Valid() bool {
	for _, k := range HomeSections {
		if k == s {
			return true
		}
	}
	return false
}

// SectionSetting says whether a section shows; its position in the list is its place on the page.
type SectionSetting struct {
	Key     HomeSection `json:"key"`
	Visible bool        `json:"visible"`
}

// SiteContent is the managed content of the public site.
type SiteContent struct {
	Trust    []TrustItem      `json:"trust"`
	Partners []Partner        `json:"partners"`
	Sections []SectionSetting `json:"sections"`

	// Set by the store, never by the editor.
	Version   int    `json:"-"`
	UpdatedAt int64  `json:"-"`
	UpdatedBy string `json:"-"`
	// Stored is false for the built-in defaults that have never been saved.
	Stored bool `json:"-"`
}

// Limits on what an editor can enter.
const (
	MaxTrustItems     = 16
	MaxPartners       = 40
	MaxPartnerName    = 60
	MaxTrustLabel     = 40
	MaxLogoURL        = 300
	MaxLinkURL        = 300
	MaxTrustMinValue  = 1_000_000
	SiteAssetMaxBytes = 512 * 1024
)

var (
	// ErrSiteContentInvalid wraps every validation failure of an edit.
	ErrSiteContentInvalid = errors.New("site content is invalid")
	// ErrSiteContentConflict means the content changed since the editor read it.
	ErrSiteContentConflict = errors.New("site content was changed by someone else")
	// ErrSiteAssetNotFound means no uploaded logo has that id.
	ErrSiteAssetNotFound = errors.New("site asset not found")
)

// SiteValidationError names the field that failed and why, in words an editor can act on.
type SiteValidationError struct {
	Field   string
	Message string
}

func (e *SiteValidationError) Error() string { return e.Field + ": " + e.Message }
func (e *SiteValidationError) Unwrap() error { return ErrSiteContentInvalid }

func invalidSite(field, format string, args ...any) error {
	return &SiteValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// DefaultSiteContent is what the site shows until staff save something: every
// metric the platform can honestly compute, and the partner logos shipped with
// the web app. It is a starting point, not a claim: staff are expected to prune it.
func DefaultSiteContent() SiteContent {
	// The strip shows what Trezutil enables: people saying what they need, customers and businesses brought together,
	// what they did together, and the businesses that are here. How many listings there are is not shown.
	enabled := map[SiteMetric]bool{
		SiteMetricConversations: true, SiteMetricConnections: true, SiteMetricTransactions: true, SiteMetricMerchants: true,
	}
	trust := make([]TrustItem, 0, len(SiteMetrics))
	for _, m := range SiteMetrics {
		trust = append(trust, TrustItem{Metric: m, Enabled: enabled[m], MinValue: 1})
	}
	sections := make([]SectionSetting, 0, len(HomeSections))
	for _, s := range HomeSections {
		sections = append(sections, SectionSetting{Key: s, Visible: true})
	}
	p := func(id, name string, kind PartnerKind, file string) Partner {
		return Partner{ID: id, Name: name, Kind: kind, LogoURL: "/partners/" + file, Visible: true}
	}
	return SiteContent{
		Trust: trust,
		Partners: []Partner{
			p("rawbank", "Rawbank", PartnerBank, "rawbank.png"),
			p("mpesa", "M-Pesa", PartnerMobileMoney, "mpesa.svg"),
			p("hyper-psaro", "Hyper Psaro", PartnerShop, "hyper-psaro.svg"),
			p("memling", "Hôtel Memling", PartnerHotel, "memling.png"),
			p("tmb", "TMB", PartnerBank, "tmb.png"),
			p("orange-money", "Orange Money", PartnerMobileMoney, "orange-money.svg"),
			p("silikin-village", "Silikin Village", PartnerHub, "silikin-village.png"),
			p("ecobank", "Ecobank", PartnerBank, "ecobank.svg"),
			p("airtel-money", "Airtel Money", PartnerMobileMoney, "airtel.svg"),
			p("pullman", "Pullman Kinshasa Grand Hotel", PartnerHotel, "pullman.svg"),
			p("access-bank", "Access Bank", PartnerBank, "access-bank.png"),
			p("stripe", "Stripe", PartnerTech, "stripe.svg"),
			p("whatsapp", "WhatsApp", PartnerTech, "whatsapp.svg"),
		},
		Sections: sections,
	}
}

var (
	partnerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	// A logo is either one shipped with the web app or one uploaded through the console.
	bundledLogoPattern  = regexp.MustCompile(`^/partners/[a-z0-9][a-z0-9._-]{0,80}\.(png|jpe?g|webp|svg)$`)
	uploadedLogoPattern = regexp.MustCompile(`^/api/v1/site/assets/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// plainText reports whether s holds only printable text without markup characters.
func plainText(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || r == '<' || r == '>' {
			return false
		}
	}
	return true
}

// Normalize trims the editor's text, drops sections that no longer exist (a
// document saved before one was removed stays valid) and adds the sections and
// metrics a newer release introduced, so an older saved document never hides
// them by accident.
func (c *SiteContent) Normalize() {
	known := c.Sections[:0:0]
	for _, s := range c.Sections {
		if s.Key.Valid() {
			known = append(known, s)
		}
	}
	c.Sections = known
	for i := range c.Trust {
		c.Trust[i].LabelFR = strings.TrimSpace(c.Trust[i].LabelFR)
		c.Trust[i].LabelEN = strings.TrimSpace(c.Trust[i].LabelEN)
	}
	for i := range c.Partners {
		p := &c.Partners[i]
		p.ID, p.Name = strings.TrimSpace(p.ID), strings.TrimSpace(p.Name)
		p.LogoURL, p.LinkURL = strings.TrimSpace(p.LogoURL), strings.TrimSpace(p.LinkURL)
	}
	have := map[HomeSection]bool{}
	for _, s := range c.Sections {
		have[s.Key] = true
	}
	for _, s := range HomeSections {
		if !have[s] {
			c.Sections = append(c.Sections, SectionSetting{Key: s, Visible: true})
		}
	}
	haveMetric := map[SiteMetric]bool{}
	for _, t := range c.Trust {
		haveMetric[t.Metric] = true
	}
	for _, m := range SiteMetrics {
		if !haveMetric[m] && len(c.Trust) < MaxTrustItems {
			c.Trust = append(c.Trust, TrustItem{Metric: m, Enabled: false, MinValue: 1})
		}
	}
}

// Validate checks every field an editor can set.
func (c *SiteContent) Validate() error {
	if len(c.Trust) > MaxTrustItems {
		return invalidSite("trust", "at most %d items", MaxTrustItems)
	}
	seenMetric := map[SiteMetric]bool{}
	for i, t := range c.Trust {
		field := fmt.Sprintf("trust[%d]", i)
		if !t.Metric.Valid() {
			return invalidSite(field+".metric", "unknown metric %q", string(t.Metric))
		}
		if seenMetric[t.Metric] {
			return invalidSite(field+".metric", "%q is listed twice", string(t.Metric))
		}
		seenMetric[t.Metric] = true
		if t.MinValue < 0 || t.MinValue > MaxTrustMinValue {
			return invalidSite(field+".min_value", "must be between 0 and %d", MaxTrustMinValue)
		}
		if utf8.RuneCountInString(t.LabelFR) > MaxTrustLabel || !plainText(t.LabelFR) {
			return invalidSite(field+".label_fr", "at most %d characters of plain text", MaxTrustLabel)
		}
		if utf8.RuneCountInString(t.LabelEN) > MaxTrustLabel || !plainText(t.LabelEN) {
			return invalidSite(field+".label_en", "at most %d characters of plain text", MaxTrustLabel)
		}
	}

	if len(c.Partners) > MaxPartners {
		return invalidSite("partners", "at most %d partners", MaxPartners)
	}
	seenID := map[string]bool{}
	for i, p := range c.Partners {
		field := fmt.Sprintf("partners[%d]", i)
		if !partnerIDPattern.MatchString(p.ID) {
			return invalidSite(field+".id", "must be 1 to 40 lowercase letters, digits or dashes")
		}
		if seenID[p.ID] {
			return invalidSite(field+".id", "%q is used twice", p.ID)
		}
		seenID[p.ID] = true
		if n := utf8.RuneCountInString(p.Name); n < 1 || n > MaxPartnerName || !plainText(p.Name) {
			return invalidSite(field+".name", "1 to %d characters of plain text", MaxPartnerName)
		}
		if !p.Kind.Valid() {
			return invalidSite(field+".kind", "unknown kind %q", string(p.Kind))
		}
		if err := validateLogoURL(p.LogoURL); err != nil {
			return invalidSite(field+".logo_url", "%v", err)
		}
		if p.LinkURL != "" {
			if err := validateHTTPSURL(p.LinkURL, MaxLinkURL); err != nil {
				return invalidSite(field+".link_url", "%v", err)
			}
		}
	}

	if len(c.Sections) != len(HomeSections) {
		return invalidSite("sections", "every section must be listed exactly once")
	}
	seenSection := map[HomeSection]bool{}
	for i, s := range c.Sections {
		if !s.Key.Valid() {
			return invalidSite(fmt.Sprintf("sections[%d].key", i), "unknown section %q", string(s.Key))
		}
		if seenSection[s.Key] {
			return invalidSite(fmt.Sprintf("sections[%d].key", i), "%q is listed twice", string(s.Key))
		}
		seenSection[s.Key] = true
	}
	return nil
}

func validateLogoURL(raw string) error {
	if raw == "" || len(raw) > MaxLogoURL {
		return errors.New("a logo is required")
	}
	if bundledLogoPattern.MatchString(raw) || uploadedLogoPattern.MatchString(raw) {
		return nil
	}
	return errors.New("use a logo that is bundled with the site or uploaded here")
}

func validateHTTPSURL(raw string, max int) error {
	if len(raw) > max {
		return fmt.Errorf("at most %d characters", max)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("must be a full https:// address")
	}
	return nil
}

// SiteContentRepository stores the managed content, the uploaded logos and the live figures.
type SiteContentRepository interface {
	// Get returns the saved content, or the built-in defaults (Stored=false, Version=0) when nothing was saved.
	Get(ctx context.Context) (*SiteContent, error)
	// Save replaces the content if the stored version is expectedVersion (0 when nothing is stored),
	// and returns it with its new version. ErrSiteContentConflict otherwise.
	Save(ctx context.Context, content *SiteContent, expectedVersion int, actorID string) (*SiteContent, error)
	// Stats computes the live value of every metric.
	Stats(ctx context.Context) (SiteStats, error)
	// SaveAsset stores an uploaded logo (de-duplicated by content) and returns its id.
	SaveAsset(ctx context.Context, contentType string, data []byte, actorID string) (string, error)
	// Asset returns an uploaded logo.
	Asset(ctx context.Context, id string) (contentType string, data []byte, err error)
}
