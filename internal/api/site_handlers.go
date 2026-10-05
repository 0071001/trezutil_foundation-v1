package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg" // registers the decoder used to check uploaded logos
	_ "image/png"  // registers the decoder used to check uploaded logos
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// SiteHandler serves the managed content of the public site:
//
//	GET  /api/v1/site/content         public: the trust strip with live figures, the partners and the section order
//	GET  /api/v1/site/assets/{id}     public: an uploaded partner logo
//	GET  /api/v1/admin/site/content   administrators: the editable document and the live figure behind each metric
//	PUT  /api/v1/admin/site/content   administrators: save it (optimistic concurrency on `expected_version`)
//	POST /api/v1/admin/site/assets    administrators: upload a partner logo
//
// No figure is ever accepted from a client: the numbers of the trust strip are
// computed from the platform's tables on every read (and cached for a minute).
type SiteHandler struct {
	repo  domain.SiteContentRepository
	users domain.UserRepository
	log   zerolog.Logger
	now   func() time.Time

	mu      sync.Mutex
	stats   domain.SiteStats
	statsAt time.Time
}

// statsTTL is how long the live figures are reused: the public home page must
// not turn every visit into a dozen table counts.
const statsTTL = 60 * time.Second

// NewSiteHandler creates a SiteHandler.
func NewSiteHandler(repo domain.SiteContentRepository, users domain.UserRepository, logger zerolog.Logger) *SiteHandler {
	return &SiteHandler{repo: repo, users: users, log: logger, now: time.Now}
}

// liveStats returns the figures, computed at most once per statsTTL. If a
// refresh fails the last good figures are kept for the next call to retry.
func (h *SiteHandler) liveStats(r *http.Request) (domain.SiteStats, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stats != nil && h.now().Sub(h.statsAt) < statsTTL {
		return h.stats, nil
	}
	fresh, err := h.repo.Stats(r.Context())
	if err != nil {
		if h.stats != nil {
			return h.stats, nil
		}
		return nil, err
	}
	h.stats, h.statsAt = fresh, h.now()
	return fresh, nil
}

type publicTrustItem struct {
	Metric  string `json:"metric"`
	Value   int    `json:"value"`
	LabelFR string `json:"label_fr,omitempty"`
	LabelEN string `json:"label_en,omitempty"`
}

type publicPartner struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	LogoURL string `json:"logo_url"`
	LinkURL string `json:"link_url,omitempty"`
}

// PublicContent is what the public site shows: only enabled metrics that have
// reached their minimum, only visible partners.
func PublicContent(c *domain.SiteContent, stats domain.SiteStats) (trust []publicTrustItem, partners []publicPartner) {
	trust, partners = []publicTrustItem{}, []publicPartner{}
	stats = c.WithPartnerCount(stats)
	for _, t := range c.Trust {
		value, known := stats[t.Metric]
		if !t.Enabled || !known || value < t.MinValue {
			continue
		}
		trust = append(trust, publicTrustItem{Metric: string(t.Metric), Value: value, LabelFR: t.LabelFR, LabelEN: t.LabelEN})
	}
	for _, p := range c.Partners {
		if p.Visible {
			partners = append(partners, publicPartner{ID: p.ID, Name: p.Name, Kind: string(p.Kind), LogoURL: p.LogoURL, LinkURL: p.LinkURL})
		}
	}
	return trust, partners
}

// PublicContentH handles GET /api/v1/site/content.
func (h *SiteHandler) PublicContentH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := h.repo.Get(r.Context())
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_FAILED", Message: "The site content could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		stats, err := h.liveStats(r)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_FAILED", Message: "The site content could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		trust, partners := PublicContent(content, stats)
		w.Header().Set("Cache-Control", "public, max-age=60")
		writeJSON(w, map[string]interface{}{
			"generated_at": isoTime(h.now().Unix()),
			"trust":        trust,
			"partners":     partners,
			"sections":     content.Sections,
		}, http.StatusOK)
	}
}

// adminView is the editable document plus what an editor needs to edit it well.
func (h *SiteHandler) adminView(r *http.Request, c *domain.SiteContent, stats domain.SiteStats) map[string]interface{} {
	stats = c.WithPartnerCount(stats)
	view := map[string]interface{}{
		"content":    map[string]interface{}{"trust": c.Trust, "partners": c.Partners, "sections": c.Sections},
		"version":    c.Version,
		"stored":     c.Stored,
		"updated_at": isoTime(c.UpdatedAt),
		"stats":      stats,
		"limits": map[string]interface{}{
			"max_trust_items": domain.MaxTrustItems, "max_partners": domain.MaxPartners, "max_partner_name": domain.MaxPartnerName,
			"max_trust_label": domain.MaxTrustLabel, "max_asset_bytes": domain.SiteAssetMaxBytes,
		},
	}
	if c.UpdatedBy != "" {
		view["updated_by"] = c.UpdatedBy
		if u, err := h.users.FindByID(r.Context(), c.UpdatedBy); err == nil {
			name := u.DisplayName
			if name == "" && u.Email != nil {
				name = *u.Email
			}
			view["updated_by_name"] = name
		}
	}
	return view
}

// AdminContentH handles GET /api/v1/admin/site/content (administrators).
func (h *SiteHandler) AdminContentH() http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		content, err := h.repo.Get(r.Context())
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_FAILED", Message: "The site content could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		stats, err := h.liveStats(r)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_FAILED", Message: "The site content could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, h.adminView(r, content, stats), http.StatusOK)
	}, domain.RoleAdmin)
}

// maxSiteBody bounds a save: forty partners and sixteen metrics fit in a few kilobytes.
const maxSiteBody = 128 << 10

// SaveContentH handles PUT /api/v1/admin/site/content (administrators).
func (h *SiteHandler) SaveContentH() http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ExpectedVersion int                     `json:"expected_version"`
			Trust           []domain.TrustItem      `json:"trust"`
			Partners        []domain.Partner        `json:"partners"`
			Sections        []domain.SectionSetting `json:"sections"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxSiteBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || dec.Decode(&struct{}{}) != io.EOF { // exactly one JSON document
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "The request body is not valid"}}, http.StatusBadRequest)
			return
		}
		if req.ExpectedVersion < 0 {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "expected_version is invalid"}}, http.StatusBadRequest)
			return
		}
		content := &domain.SiteContent{Trust: req.Trust, Partners: req.Partners, Sections: req.Sections}
		if content.Trust == nil {
			content.Trust = []domain.TrustItem{}
		}
		if content.Partners == nil {
			content.Partners = []domain.Partner{}
		}
		content.Normalize()
		if err := content.Validate(); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_INVALID", Message: err.Error()}}, http.StatusUnprocessableEntity)
			return
		}
		actor := auth.UserID(r.Context())
		saved, err := h.repo.Save(r.Context(), content, req.ExpectedVersion, actor)
		switch {
		case errors.Is(err, domain.ErrSiteContentConflict):
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_CONFLICT", Message: "Someone else saved changes since you opened this page. Reload to see them."}}, http.StatusConflict)
			return
		case err != nil:
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_CONTENT_FAILED", Message: "The changes could not be saved"}}, http.StatusInternalServerError)
			return
		}
		h.log.Info().Str("event", "site_content_saved").Str("actor", actor).Int("version", saved.Version).
			Int("trust_items", len(saved.Trust)).Int("partners", len(saved.Partners)).Msg("site content saved")
		stats, _ := h.liveStats(r)
		writeJSON(w, h.adminView(r, saved, stats), http.StatusOK)
	}, domain.RoleAdmin)
}

var errLogoSize = errors.New("logo size out of range")

// Allowed logo formats, by sniffed content type.
var siteAssetTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true}

// A logo is small by nature; the ceiling also keeps the decoded image (width x height x 4 bytes) to 16 MB.
const (
	minLogoPixels = 16
	maxLogoPixels = 2048
)

// UploadAssetH handles POST /api/v1/admin/site/assets (administrators): one
// PNG, JPEG or WebP logo of at most 512 KiB, sent as the multipart field "file".
// The type is decided from the bytes, never from the file name or the header.
func (h *SiteHandler) UploadAssetH() http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, domain.SiteAssetMaxBytes+64<<10)
		if err := r.ParseMultipartForm(domain.SiteAssetMaxBytes + 64<<10); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ASSET_TOO_LARGE", Message: "The logo must be 512 KB or smaller"}}, http.StatusRequestEntityTooLarge)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Send the logo as the form field \"file\""}}, http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, domain.SiteAssetMaxBytes+1))
		if err != nil || len(data) == 0 {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "The logo could not be read"}}, http.StatusBadRequest)
			return
		}
		if len(data) > domain.SiteAssetMaxBytes {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ASSET_TOO_LARGE", Message: "The logo must be 512 KB or smaller"}}, http.StatusRequestEntityTooLarge)
			return
		}
		contentType := http.DetectContentType(data)
		if !siteAssetTypes[contentType] {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ASSET_TYPE_NOT_ALLOWED", Message: "Use a PNG, JPEG or WebP image"}}, http.StatusUnsupportedMediaType)
			return
		}
		if contentType != "image/webp" { // WebP is checked by its signature; PNG and JPEG are decoded in full
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			if err == nil && (cfg.Width < minLogoPixels || cfg.Height < minLogoPixels || cfg.Width > maxLogoPixels || cfg.Height > maxLogoPixels) {
				err = errLogoSize
			}
			if err == nil { // the size is known and bounded, so a full decode is affordable
				_, _, err = image.Decode(bytes.NewReader(data))
			}
			if err != nil {
				writeJSON(w, ErrorResponse{Error: Error{Code: "ASSET_INVALID", Message: "The image is damaged or its size is not between 16 and 2048 pixels"}}, http.StatusUnprocessableEntity)
				return
			}
		}
		actor := auth.UserID(r.Context())
		id, err := h.repo.SaveAsset(r.Context(), contentType, data, actor)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_ASSET_FAILED", Message: "The logo could not be saved"}}, http.StatusInternalServerError)
			return
		}
		h.log.Info().Str("event", "site_asset_uploaded").Str("actor", actor).Str("asset", id).Int("bytes", len(data)).Msg("site logo uploaded")
		writeJSON(w, map[string]interface{}{"id": id, "url": "/api/v1/site/assets/" + id, "content_type": contentType, "size": len(data)}, http.StatusCreated)
	}, domain.RoleAdmin)
}

// AssetH handles GET /api/v1/site/assets/{id}: a logo uploaded through the console.
func (h *SiteHandler) AssetH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType, data, err := h.repo.Asset(r.Context(), r.PathValue("id"))
		if errors.Is(err, domain.ErrSiteAssetNotFound) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Logo not found"}}, http.StatusNotFound)
			return
		}
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "SITE_ASSET_FAILED", Message: "The logo could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		// The bytes behind an id never change, so they can be cached for good.
		// The headers keep an uploaded file from ever being treated as a page.
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		_, _ = w.Write(data)
	}
}
