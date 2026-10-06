package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

/*
 * Editing a business, a seller profile or a store from the seller workspace.
 *
 * The update use cases take a loose map; what reaches them is decided here: only the fields a seller may
 * edit (never the status, the verification or the owner), trimmed, bounded, an email that parses and a phone
 * number that looks like one. Optional text fields arrive as *string, the form the use cases apply. Who may
 * edit is still decided by the use cases (owner and admin for the business; the profile's owner, or owner,
 * admin and manager, for a seller profile and a store).
 */

type profileField struct {
	max      int
	required bool // cannot be emptied
	optional bool // stored as a nullable column: passed as *string
	kind     string
}

var (
	organizationFields = map[string]profileField{
		"display_name": {max: 120, required: true},
		"legal_name":   {max: 200, required: true},
		"description":  {max: 2000, optional: true},
		"phone":        {max: 32, kind: "phone"},
		"email":        {max: 254, optional: true, kind: "email"},
		"province":     {max: 100, optional: true},
		"city":         {max: 100, optional: true},
		"address":      {max: 300, optional: true},
	}
	sellerFields = map[string]profileField{
		"display_name": {max: 120, required: true},
		"bio":          {max: 2000, optional: true},
		"phone":        {max: 32, kind: "phone"},
		"email":        {max: 254, optional: true, kind: "email"},
	}
	storeFields = map[string]profileField{
		"name":        {max: 120, required: true},
		"description": {max: 2000, optional: true},
		"phone":       {max: 32, kind: "phone"},
		"email":       {max: 254, optional: true, kind: "email"},
		"province":    {max: 100, optional: true},
		"city":        {max: 100, optional: true},
		"address":     {max: 300, optional: true},
	}
	phonePattern = regexp.MustCompile(`^\+?[0-9 ()-]{6,31}$`)
)

// profileUpdateError is a field the request may not set as sent.
type profileUpdateError struct{ field, message string }

// sanitizeProfileUpdates keeps the fields the request may set, checked and in the types the use cases read.
func sanitizeProfileUpdates(raw map[string]interface{}, allowed map[string]profileField) (map[string]interface{}, *profileUpdateError) {
	out := map[string]interface{}{}
	for key, value := range raw {
		f, ok := allowed[key]
		if !ok {
			return nil, &profileUpdateError{key, "This field cannot be changed here"}
		}
		s, ok := value.(string)
		if !ok && value != nil {
			return nil, &profileUpdateError{key, "Must be text"}
		}
		s = strings.TrimSpace(s)
		if utf8.RuneCountInString(s) > f.max {
			return nil, &profileUpdateError{key, "Too long"}
		}
		if s == "" && f.required {
			return nil, &profileUpdateError{key, "Cannot be empty"}
		}
		if s != "" {
			switch f.kind {
			case "email":
				if addr, err := mail.ParseAddress(s); err != nil || addr.Address != s {
					return nil, &profileUpdateError{key, "Not a valid email address"}
				}
			case "phone":
				if !phonePattern.MatchString(s) {
					return nil, &profileUpdateError{key, "Not a valid phone number"}
				}
			}
		}
		if f.optional {
			v := s
			out[key] = &v
		} else {
			out[key] = s
		}
	}
	if len(out) == 0 {
		return nil, &profileUpdateError{"", "Nothing to change"}
	}
	return out, nil
}

// readProfileUpdates decodes and checks the body of an edit; it answers the 400 itself.
func readProfileUpdates(w http.ResponseWriter, r *http.Request, allowed map[string]profileField) (map[string]interface{}, bool) {
	var raw map[string]interface{}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&raw); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return nil, false
	}
	clean, bad := sanitizeProfileUpdates(raw, allowed)
	if bad != nil {
		writeJSON(w, map[string]interface{}{"error": map[string]string{"code": "INVALID_FIELD", "field": bad.field, "message": bad.message}}, http.StatusBadRequest)
		return nil, false
	}
	return clean, true
}

// profileEdit is the shared shape of the three edit routes.
func profileEdit(param string, allowed map[string]profileField, code string, update func(r *http.Request, actorID, id string, updates map[string]interface{}) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		id := r.URL.Query().Get(param)
		if !orgIDPattern.MatchString(id) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: param + " is required"}}, http.StatusBadRequest)
			return
		}
		updates, ok := readProfileUpdates(w, r, allowed)
		if !ok {
			return
		}
		if err := update(r, actorID, id, updates); err != nil {
			if errors.Is(err, domain.ErrInsufficientPermissions) {
				writeFailure(w, code, err)
				return
			}
			if isNotFound(err) {
				writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Not found"}}, http.StatusNotFound)
				return
			}
			writeFailure(w, code, err)
			return
		}
		writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
	}
}

// PatchOrganizationH handles PATCH /api/v1/organizations?organization_id= (owner and admin of the business).
func (h *OrganizationHandler) PatchOrganizationH() http.HandlerFunc {
	return profileEdit("organization_id", organizationFields, "ORGANIZATION_UPDATE_FAILED", func(r *http.Request, actorID, id string, u map[string]interface{}) error {
		return h.orgUseCases.UpdateOrganization(r.Context(), actorID, id, u)
	})
}

// PatchSellerH handles PATCH /api/v1/sellers?seller_id= (the profile's owner; owner, admin and manager of its business).
func (h *SellerHandler) PatchSellerH() http.HandlerFunc {
	return profileEdit("seller_id", sellerFields, "SELLER_UPDATE_FAILED", func(r *http.Request, actorID, id string, u map[string]interface{}) error {
		return h.sellerUseCases.UpdateSellerProfile(r.Context(), actorID, id, u)
	})
}

// PatchStoreH handles PATCH /api/v1/stores?store_id= (owner, admin and manager of its business).
func (h *StoreHandler) PatchStoreH() http.HandlerFunc {
	return profileEdit("store_id", storeFields, "STORE_UPDATE_FAILED", func(r *http.Request, actorID, id string, u map[string]interface{}) error {
		return h.storeUseCases.UpdateStore(r.Context(), actorID, id, u)
	})
}

func isNotFound(err error) bool {
	return errors.Is(err, domain.ErrOrganizationNotFound) || errors.Is(err, domain.ErrSellerNotFound) || errors.Is(err, domain.ErrStoreNotFound)
}
