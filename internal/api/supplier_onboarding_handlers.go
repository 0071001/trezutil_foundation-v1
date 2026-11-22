package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/supplieronboarding"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// SupplierOnboardingHandler serves supplier self-registration (supplieronboarding.Service):
//
//	POST   /api/v1/supplier/applications                         register a business (the caller becomes its owner)
//	GET    /api/v1/supplier/applications                         the caller's applications
//	GET    /api/v1/supplier/applications/{org}                   one application (members, staff)
//	PATCH  /api/v1/supplier/applications/{org}                   update while draft / changes requested
//	POST   /api/v1/supplier/applications/{org}/documents         multipart: kind, file (PDF, JPEG, PNG, WebP ≤ 5 MB)
//	GET    /api/v1/supplier/applications/{org}/documents/{doc}   the document (members, staff)
//	DELETE /api/v1/supplier/applications/{org}/documents/{doc}
//	POST   /api/v1/supplier/applications/{org}/submit            send for verification (owner, admin)
//	GET    /api/v1/admin/supplier-applications?status=submitted  review queue (staff)
//	POST   /api/v1/admin/supplier-applications/{org}/review      {"decision":"verify|reject|request_changes","note":"…"}
type SupplierOnboardingHandler struct {
	svc    *supplieronboarding.Service
	logger zerolog.Logger
}

// NewSupplierOnboardingHandler builds the handler.
func NewSupplierOnboardingHandler(svc *supplieronboarding.Service, logger zerolog.Logger) *SupplierOnboardingHandler {
	return &SupplierOnboardingHandler{svc: svc, logger: logger}
}

type supplierDetailsJSON struct {
	OrgType      string   `json:"org_type"`
	LegalName    string   `json:"legal_name"`
	DisplayName  string   `json:"display_name"`
	Description  string   `json:"description"`
	Phone        string   `json:"phone"`
	Email        string   `json:"email"`
	Country      string   `json:"country"`
	Province     string   `json:"province"`
	City         string   `json:"city"`
	Address      string   `json:"address"`
	LegalForm    string   `json:"legal_form"`
	RCCM         string   `json:"rccm"`
	IDNat        string   `json:"id_nat"`
	NIF          string   `json:"nif"`
	Website      string   `json:"website"`
	ContactName  string   `json:"contact_name"`
	ContactRole  string   `json:"contact_role"`
	Sectors      []string `json:"sectors"`
	Capabilities string   `json:"capabilities"`
	MinOrder     string   `json:"min_order"`
	LeadTimeDays *int     `json:"lead_time_days"`
	PaymentTerms string   `json:"payment_terms"`
	ServiceAreas []string `json:"service_areas"`
}

func (d supplierDetailsJSON) details() supplieronboarding.Details {
	return supplieronboarding.Details{
		OrgType: d.OrgType, LegalName: d.LegalName, DisplayName: d.DisplayName, Description: d.Description, Phone: d.Phone,
		Email: d.Email, Country: d.Country, Province: d.Province, City: d.City, Address: d.Address, LegalForm: d.LegalForm,
		RCCM: d.RCCM, IDNat: d.IDNat, NIF: d.NIF, Website: d.Website, ContactName: d.ContactName, ContactRole: d.ContactRole,
		Sectors: d.Sectors, Capabilities: d.Capabilities, MinOrder: d.MinOrder, LeadTimeDays: d.LeadTimeDays,
		PaymentTerms: d.PaymentTerms, ServiceAreas: d.ServiceAreas,
	}
}

func applicationJSON(a *domain.SupplierApplication) map[string]interface{} {
	docs := make([]map[string]interface{}, 0, len(a.Documents))
	for _, d := range a.Documents {
		docs = append(docs, map[string]interface{}{
			"id": d.ID, "kind": d.Kind, "filename": d.Filename, "content_type": d.ContentType, "size_bytes": d.SizeBytes,
			"status": d.Status, "created_at": isoTime(d.CreatedAt),
			"url": fmt.Sprintf("/api/v1/supplier/applications/%s/documents/%s", a.OrganizationID, d.ID),
		})
	}
	out := map[string]interface{}{
		"organization_id": a.OrganizationID, "status": a.Status, "org_type": a.OrgType, "legal_name": a.LegalName,
		"display_name": a.DisplayName, "description": a.Description, "phone": a.Phone, "email": a.Email, "country": a.Country,
		"province": a.Province, "city": a.City, "address": a.Address, "legal_form": a.LegalForm, "rccm": a.RCCM, "id_nat": a.IDNat,
		"nif": a.NIF, "website": a.Website, "contact_name": a.ContactName, "contact_role": a.ContactRole, "sectors": nonNilStrings(a.Sectors),
		"capabilities": a.Capabilities, "min_order": a.MinOrder, "lead_time_days": a.LeadTimeDays, "payment_terms": a.PaymentTerms,
		"service_areas": nonNilStrings(a.ServiceAreas), "review_note": a.ReviewNote, "documents": docs,
		"missing": a.MissingForSubmission(), "editable": a.Status.Editable(),
		"created_at": isoTime(a.CreatedAt), "updated_at": isoTime(a.UpdatedAt),
	}
	if a.SubmittedAt != 0 {
		out["submitted_at"] = isoTime(a.SubmittedAt)
	}
	if a.ReviewedAt != 0 {
		out["reviewed_at"] = isoTime(a.ReviewedAt)
	}
	return out
}

// fail answers an error of the service. Someone else's business answers 404, like one that does not exist.
func (h *SupplierOnboardingHandler) fail(w http.ResponseWriter, err error) {
	var invalid *supplieronboarding.ValidationError
	var incomplete *supplieronboarding.IncompleteError
	var active *supplieronboarding.ActiveApplicationError
	switch {
	case errors.As(err, &invalid):
		writeJSON(w, map[string]interface{}{"error": map[string]string{"code": "INVALID_FIELD", "message": invalid.Error(), "field": invalid.Field}}, http.StatusBadRequest)
	case errors.As(err, &incomplete):
		writeJSON(w, map[string]interface{}{"error": map[string]interface{}{"code": "INCOMPLETE", "message": "The application is not complete", "missing": incomplete.Missing}}, http.StatusUnprocessableEntity)
	case errors.As(err, &active):
		writeJSON(w, map[string]interface{}{"error": map[string]string{"code": "APPLICATION_IN_PROGRESS", "message": "This account already has an application in progress", "organization_id": active.OrganizationID}}, http.StatusConflict)
	case errors.Is(err, supplieronboarding.ErrForbidden):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Not allowed"}}, http.StatusForbidden)
	case errors.Is(err, supplieronboarding.ErrSelfReview):
		writeJSON(w, ErrorResponse{Error: Error{Code: "SELF_REVIEW", Message: "You cannot review a business you belong to"}}, http.StatusForbidden)
	case errors.Is(err, supplieronboarding.ErrNoteRequired):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOTE_REQUIRED", Message: "Explain the decision to the supplier"}}, http.StatusBadRequest)
	case errors.Is(err, supplieronboarding.ErrDocumentType):
		writeJSON(w, ErrorResponse{Error: Error{Code: "DOCUMENT_TYPE", Message: "Send a PDF, JPEG, PNG or WebP file"}}, http.StatusUnsupportedMediaType)
	case errors.Is(err, supplieronboarding.ErrDocumentSize):
		writeJSON(w, ErrorResponse{Error: Error{Code: "DOCUMENT_TOO_LARGE", Message: "A document must be 5 MB or smaller"}}, http.StatusRequestEntityTooLarge)
	case errors.Is(err, domain.ErrSupplierDuplicateDocument):
		writeJSON(w, ErrorResponse{Error: Error{Code: "DUPLICATE_DOCUMENT", Message: "This document was already sent"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrSupplierNotEditable), errors.Is(err, domain.ErrSupplierStatusChanged):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_EDITABLE", Message: "The application cannot be changed in its current status (or has reached 10 documents)"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrSupplierApplicationNotFound), errors.Is(err, domain.ErrSupplierDocumentNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Not found"}}, http.StatusNotFound)
	default:
		h.logger.Error().Err(err).Msg("Supplier onboarding failed")
		writeJSON(w, ErrorResponse{Error: Error{Code: "SUPPLIER_ONBOARDING_FAILED", Message: "The request could not be completed"}}, http.StatusInternalServerError)
	}
}

// Register mounts the routes on a Go 1.22+ mux.
func (h *SupplierOnboardingHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/supplier/applications", h.start)
	mux.HandleFunc("GET /api/v1/supplier/applications", h.mine)
	mux.HandleFunc("GET /api/v1/supplier/applications/{org}", h.get)
	mux.HandleFunc("PATCH /api/v1/supplier/applications/{org}", h.update)
	mux.HandleFunc("POST /api/v1/supplier/applications/{org}/documents", h.addDocument)
	mux.HandleFunc("GET /api/v1/supplier/applications/{org}/documents/{doc}", h.document)
	mux.HandleFunc("DELETE /api/v1/supplier/applications/{org}/documents/{doc}", h.deleteDocument)
	mux.HandleFunc("POST /api/v1/supplier/applications/{org}/submit", h.submit)
	mux.HandleFunc("GET /api/v1/admin/supplier-applications", h.queue)
	mux.HandleFunc("POST /api/v1/admin/supplier-applications/{org}/review", h.review)
}

func (h *SupplierOnboardingHandler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	return auth.RequireUser(w, r)
}

func (h *SupplierOnboardingHandler) start(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body supplierDetailsJSON
	if err := readJSONBody(r, &body); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	app, err := h.svc.Start(r.Context(), userID, body.details())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, applicationJSON(app), http.StatusCreated)
}

func (h *SupplierOnboardingHandler) mine(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Mine(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		items = append(items, applicationJSON(&list[i]))
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

func (h *SupplierOnboardingHandler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	app, err := h.svc.Get(r.Context(), userID, r.PathValue("org"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, applicationJSON(app), http.StatusOK)
}

func (h *SupplierOnboardingHandler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body supplierDetailsJSON
	if err := readJSONBody(r, &body); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	app, err := h.svc.Update(r.Context(), userID, r.PathValue("org"), body.details())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, applicationJSON(app), http.StatusOK)
}

func (h *SupplierOnboardingHandler) addDocument(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	limit := int64(domain.SupplierDocumentMaxBytes + 64<<10)
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := r.ParseMultipartForm(limit); err != nil {
		h.fail(w, supplieronboarding.ErrDocumentSize)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Send the document as the form field \"file\""}}, http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(domain.SupplierDocumentMaxBytes)+1))
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "The document could not be read"}}, http.StatusBadRequest)
		return
	}
	doc, err := h.svc.AddDocument(r.Context(), userID, r.PathValue("org"), domain.SupplierDocumentKind(strings.TrimSpace(r.FormValue("kind"))), header.Filename, data)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"id": doc.ID, "kind": doc.Kind, "filename": doc.Filename, "content_type": doc.ContentType,
		"size_bytes": doc.SizeBytes, "status": doc.Status, "created_at": isoTime(doc.CreatedAt)}, http.StatusCreated)
}

func (h *SupplierOnboardingHandler) document(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	doc, data, err := h.svc.Document(r.Context(), userID, r.PathValue("org"), r.PathValue("doc"))
	if err != nil {
		h.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", doc.Filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	_, _ = w.Write(data)
}

func (h *SupplierOnboardingHandler) deleteDocument(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteDocument(r.Context(), userID, r.PathValue("org"), r.PathValue("doc")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SupplierOnboardingHandler) submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	app, err := h.svc.Submit(r.Context(), userID, r.PathValue("org"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, applicationJSON(app), http.StatusOK)
}

func (h *SupplierOnboardingHandler) queue(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Queue(r.Context(), userID, domain.SupplierApplicationStatus(r.URL.Query().Get("status")))
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for i := range list {
		items = append(items, applicationJSON(&list[i]))
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

func (h *SupplierOnboardingHandler) review(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.caller(w, r)
	if !ok {
		return
	}
	var body struct {
		Decision string `json:"decision"`
		Note     string `json:"note"`
	}
	if err := readJSONBody(r, &body); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	app, err := h.svc.Review(r.Context(), userID, r.PathValue("org"), domain.SupplierReviewDecision(body.Decision), body.Note)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, applicationJSON(app), http.StatusOK)
}
