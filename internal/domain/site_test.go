package domain

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSiteContent_IsValidAndHonest(t *testing.T) {
	c := DefaultSiteContent()
	if err := c.Validate(); err != nil {
		t.Fatalf("the built-in content must pass its own validation: %v", err)
	}
	if len(c.Sections) != len(HomeSections) {
		t.Fatalf("every section is listed once, got %d of %d", len(c.Sections), len(HomeSections))
	}
	for _, s := range c.Sections {
		if !s.Visible {
			t.Errorf("section %q should start visible", s.Key)
		}
	}
	// A metric is never shown at zero by default: a young platform must not advertise "0".
	for _, item := range c.Trust {
		if item.MinValue < 1 {
			t.Errorf("metric %q starts with min_value %d; defaults must hide a zero", item.Metric, item.MinValue)
		}
	}
	// Every default logo is one shipped with the web app, never a remote address.
	for _, p := range c.Partners {
		if !strings.HasPrefix(p.LogoURL, "/partners/") {
			t.Errorf("default logo of %q is not bundled: %s", p.Name, p.LogoURL)
		}
	}
}

func TestSiteContent_NormalizeAddsWhatANewerReleaseIntroduced(t *testing.T) {
	// A document saved before the "partners" section and most metrics existed.
	c := SiteContent{
		Trust:    []TrustItem{{Metric: SiteMetricLiveListings, Enabled: true, MinValue: 5, LabelFR: "  annonces  "}},
		Sections: []SectionSetting{{Key: SectionNearby, Visible: false}, {Key: SectionTrust, Visible: true}},
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("a normalized older document must be valid: %v", err)
	}
	if got := c.Trust[0].LabelFR; got != "annonces" {
		t.Errorf("label not trimmed: %q", got)
	}
	if len(c.Trust) != len(SiteMetrics) {
		t.Errorf("missing metrics should be added (disabled), got %d of %d", len(c.Trust), len(SiteMetrics))
	}
	for _, item := range c.Trust[1:] {
		if item.Enabled {
			t.Errorf("a metric added by Normalize must not appear on the site by itself: %q", item.Metric)
		}
	}
	// What the editor chose is kept: "nearby" stays hidden and first.
	if c.Sections[0].Key != SectionNearby || c.Sections[0].Visible {
		t.Errorf("the editor's order and visibility must survive: %+v", c.Sections[0])
	}
	if len(c.Sections) != len(HomeSections) {
		t.Errorf("missing sections should be appended, got %d of %d", len(c.Sections), len(HomeSections))
	}
	last := c.Sections[len(c.Sections)-1]
	if !last.Visible {
		t.Errorf("a section added by Normalize starts visible: %+v", last)
	}
}

func validContent() SiteContent {
	c := DefaultSiteContent()
	c.Partners = []Partner{{ID: "rawbank", Name: "Rawbank", Kind: PartnerBank, LogoURL: "/partners/rawbank.png", Visible: true}}
	return c
}

func TestSiteContent_Validate(t *testing.T) {
	uploaded := "/api/v1/site/assets/5e11e700-0000-4000-8000-0000000000a1"
	tests := []struct {
		name   string
		mutate func(c *SiteContent)
		field  string // "" means valid
	}{
		{"valid as is", func(c *SiteContent) {}, ""},
		{"uploaded logo", func(c *SiteContent) { c.Partners[0].LogoURL = uploaded }, ""},
		{"https link", func(c *SiteContent) { c.Partners[0].LinkURL = "https://www.rawbank.cd/fr" }, ""},
		{"no partners at all", func(c *SiteContent) { c.Partners = []Partner{} }, ""},

		{"too many trust items", func(c *SiteContent) {
			for i := 0; i < MaxTrustItems; i++ {
				c.Trust = append(c.Trust, TrustItem{Metric: SiteMetricLiveListings})
			}
		}, "trust"},
		{"unknown metric", func(c *SiteContent) { c.Trust[0].Metric = "members_online" }, "trust[0].metric"},
		{"metric twice", func(c *SiteContent) { c.Trust[1].Metric = c.Trust[0].Metric }, "trust[1].metric"},
		{"negative minimum", func(c *SiteContent) { c.Trust[0].MinValue = -1 }, "trust[0].min_value"},
		{"minimum too large", func(c *SiteContent) { c.Trust[0].MinValue = MaxTrustMinValue + 1 }, "trust[0].min_value"},
		{"label with markup", func(c *SiteContent) { c.Trust[0].LabelFR = "<b>annonces</b>" }, "trust[0].label_fr"},
		{"label with control character", func(c *SiteContent) { c.Trust[0].LabelEN = "ads\u0007" }, "trust[0].label_en"},
		{"label too long", func(c *SiteContent) { c.Trust[0].LabelEN = strings.Repeat("é", MaxTrustLabel+1) }, "trust[0].label_en"},
		{"label at the limit", func(c *SiteContent) { c.Trust[0].LabelEN = strings.Repeat("é", MaxTrustLabel) }, ""},

		{"too many partners", func(c *SiteContent) {
			c.Partners = nil
			for i := 0; i <= MaxPartners; i++ {
				c.Partners = append(c.Partners, Partner{ID: "p" + strings.Repeat("a", i%30) + string(rune('a'+i%26)), Name: "P", Kind: PartnerOther, LogoURL: "/partners/a.png"})
			}
		}, "partners"},
		{"empty id", func(c *SiteContent) { c.Partners[0].ID = "" }, "partners[0].id"},
		{"id with capitals", func(c *SiteContent) { c.Partners[0].ID = "Rawbank" }, "partners[0].id"},
		{"id with a slash", func(c *SiteContent) { c.Partners[0].ID = "a/b" }, "partners[0].id"},
		{"duplicate id", func(c *SiteContent) { c.Partners = append(c.Partners, c.Partners[0]) }, "partners[1].id"},
		{"empty name", func(c *SiteContent) { c.Partners[0].Name = "" }, "partners[0].name"},
		{"name with markup", func(c *SiteContent) { c.Partners[0].Name = "Raw<script>" }, "partners[0].name"},
		{"name too long", func(c *SiteContent) { c.Partners[0].Name = strings.Repeat("a", MaxPartnerName+1) }, "partners[0].name"},
		{"unknown kind", func(c *SiteContent) { c.Partners[0].Kind = "casino" }, "partners[0].kind"},

		// A logo may only come from this site: no remote tracking pixels, no script URLs.
		{"empty logo", func(c *SiteContent) { c.Partners[0].LogoURL = "" }, "partners[0].logo_url"},
		{"remote https logo", func(c *SiteContent) { c.Partners[0].LogoURL = "https://cdn.example.com/logo.png" }, "partners[0].logo_url"},
		{"http logo", func(c *SiteContent) { c.Partners[0].LogoURL = "http://localhost/partners/a.png" }, "partners[0].logo_url"},
		{"javascript logo", func(c *SiteContent) { c.Partners[0].LogoURL = "javascript:alert(1)" }, "partners[0].logo_url"},
		{"data uri logo", func(c *SiteContent) { c.Partners[0].LogoURL = "data:image/png;base64,AAAA" }, "partners[0].logo_url"},
		{"path traversal", func(c *SiteContent) { c.Partners[0].LogoURL = "/partners/../secret.png" }, "partners[0].logo_url"},
		{"nested bundled path", func(c *SiteContent) { c.Partners[0].LogoURL = "/partners/a/b.png" }, "partners[0].logo_url"},
		{"other site path", func(c *SiteContent) { c.Partners[0].LogoURL = "/api/v1/admin/users" }, "partners[0].logo_url"},
		{"uploaded id malformed", func(c *SiteContent) { c.Partners[0].LogoURL = "/api/v1/site/assets/../../admin/users" }, "partners[0].logo_url"},

		{"http link", func(c *SiteContent) { c.Partners[0].LinkURL = "http://rawbank.cd" }, "partners[0].link_url"},
		{"link without host", func(c *SiteContent) { c.Partners[0].LinkURL = "https://" }, "partners[0].link_url"},
		{"link with credentials", func(c *SiteContent) { c.Partners[0].LinkURL = "https://user:pw@rawbank.cd" }, "partners[0].link_url"},
		{"javascript link", func(c *SiteContent) { c.Partners[0].LinkURL = "javascript:alert(1)" }, "partners[0].link_url"},

		{"unknown section", func(c *SiteContent) { c.Sections[0].Key = "casino" }, "sections[0].key"},
		{"section twice", func(c *SiteContent) { c.Sections[1].Key = c.Sections[0].Key }, "sections[1].key"},
		{"section missing", func(c *SiteContent) { c.Sections = c.Sections[1:] }, "sections"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validContent()
			tc.mutate(&c)
			err := c.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error on %s", tc.field)
			}
			if !errors.Is(err, ErrSiteContentInvalid) {
				t.Errorf("the error must wrap ErrSiteContentInvalid: %v", err)
			}
			var ve *SiteValidationError
			if !errors.As(err, &ve) || ve.Field != tc.field {
				t.Errorf("expected the error on %q, got %v", tc.field, err)
			}
		})
	}
}

func TestSiteMetricCatalogue(t *testing.T) {
	seen := map[SiteMetric]bool{}
	for _, m := range SiteMetrics {
		if !m.Valid() || seen[m] {
			t.Errorf("metric %q is invalid or listed twice", m)
		}
		seen[m] = true
	}
	if SiteMetric("anything_else").Valid() {
		t.Error("an unknown metric must not be valid")
	}
	sections := map[HomeSection]bool{}
	for _, s := range HomeSections {
		if !s.Valid() || sections[s] {
			t.Errorf("section %q is invalid or listed twice", s)
		}
		sections[s] = true
	}
}

// The default partners point at files shipped with the web app. If a file is
// renamed or removed, the public band would show a broken image: fail here.
func TestDefaultSiteContent_BundledLogosExistInTheWebApp(t *testing.T) {
	for _, p := range DefaultSiteContent().Partners {
		file := filepath.Join("..", "..", "web", "frontend", "public", filepath.FromSlash(p.LogoURL))
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("%s: logo %s is missing: %v", p.Name, p.LogoURL, err)
			continue
		}
		if info.Size() < 500 {
			t.Errorf("%s: %s is only %d bytes, probably not an image", p.Name, p.LogoURL, info.Size())
		}
	}
}

// A section can be removed from the product (the home page's promotions block
// was). A document saved before that must keep working: the removed key is
// dropped, everything else the editor chose is kept, and the result validates.
func TestSiteContent_NormalizeDropsASectionThatNoLongerExists(t *testing.T) {
	c := DefaultSiteContent()
	c.Sections = append([]SectionSetting{{Key: "deals", Visible: true}}, c.Sections...)
	c.Sections[2].Visible = false // the editor had hidden something else
	hidden := c.Sections[2].Key
	if err := c.Validate(); err == nil {
		t.Fatal("a document with an unknown section is invalid until normalized")
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("normalized document must be valid: %v", err)
	}
	for _, s := range c.Sections {
		if s.Key == "deals" {
			t.Fatal("the removed section must be gone")
		}
		if s.Key == hidden && s.Visible {
			t.Errorf("the editor's choice for %q must survive", hidden)
		}
	}
	if len(c.Sections) != len(HomeSections) {
		t.Errorf("sections = %d, want %d", len(c.Sections), len(HomeSections))
	}
}

func TestDefaultSiteContent_TrustStripShowsPeopleBusinessesTransactionsAndPartners(t *testing.T) {
	c := DefaultSiteContent()
	var on []SiteMetric
	for _, item := range c.Trust {
		if item.Enabled {
			on = append(on, item.Metric)
			if item.MinValue < 1 {
				t.Errorf("%s: a figure at zero is hidden by default, so a young platform never advertises it", item.Metric)
			}
		}
	}
	want := []SiteMetric{SiteMetricConversations, SiteMetricConnections, SiteMetricTransactions, SiteMetricMerchants} // what Trezutil enables, never how many listings it holds
	if len(on) != len(want) {
		t.Fatalf("shown by default: %v, want %v", on, want)
	}
	for i := range want {
		if on[i] != want[i] {
			t.Errorf("position %d: %s, want %s", i, on[i], want[i])
		}
	}
	if MaxTrustItems < len(SiteMetrics) {
		t.Errorf("an editor must be able to list every metric: limit %d, metrics %d", MaxTrustItems, len(SiteMetrics))
	}
}

func TestSiteContent_WithPartnerCountCountsOnlyWhatIsShownAndLeavesTheFiguresAlone(t *testing.T) {
	c := SiteContent{Partners: []Partner{{ID: "a", Visible: true}, {ID: "b", Visible: false}, {ID: "c", Visible: true}}}
	cached := SiteStats{SiteMetricMerchants: 20}
	out := c.WithPartnerCount(cached)
	if out[SiteMetricPartners] != 2 {
		t.Errorf("two partners are shown, got %d", out[SiteMetricPartners])
	}
	if out[SiteMetricMerchants] != 20 {
		t.Errorf("the other figures are carried over, got %d", out[SiteMetricMerchants])
	}
	if _, touched := cached[SiteMetricPartners]; touched || len(cached) != 1 {
		t.Errorf("the cached figures are shared between requests and must not be modified: %v", cached)
	}
}
