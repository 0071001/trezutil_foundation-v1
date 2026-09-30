package api

import (
	"net/http"

	"github.com/trezutil/trezutil/internal/domain"
)

// verticalCatalog lists the vertical catalogue.
type verticalCatalog interface {
	All() []domain.VerticalDefinition
}

// VerticalDTO is the public description of a commerce vertical: what the
// customer can do with its listings (flows) and which capabilities it runs on.
type VerticalDTO struct {
	Code         string   `json:"code"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Flows        []string `json:"flows"`
	Capabilities []string `json:"capabilities"`
	ListingTypes []string `json:"listing_types"`
}

// ListVerticalsH handles GET /v1/verticals (public): the vertical catalogue
// the web app uses to choose booking or purchase flows and to let sellers pick
// a vertical. It changes only with a release, so it may be cached briefly.
func ListVerticalsH(catalog verticalCatalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defs := catalog.All()
		items := make([]VerticalDTO, 0, len(defs))
		for _, d := range defs {
			dto := VerticalDTO{Code: string(d.Code), Name: d.Name, Description: d.Description,
				Flows: []string{}, Capabilities: []string{}, ListingTypes: []string{}}
			for _, f := range d.Flows {
				dto.Flows = append(dto.Flows, string(f))
			}
			for _, c := range d.Capabilities {
				dto.Capabilities = append(dto.Capabilities, string(c))
			}
			for _, t := range d.ListingTypes {
				dto.ListingTypes = append(dto.ListingTypes, string(t))
			}
			items = append(items, dto)
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
	}
}
