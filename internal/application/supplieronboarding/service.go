// Package supplieronboarding is supplier self-registration: a signed-in person registers a business, describes it,
// sends its documents and submits it; Trezutil staff verify it, refuse it or ask for changes. Registering never makes
// a supplier verified or trusted (migration 000034).
package supplieronboarding

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
)

var (
	ErrForbidden         = errors.New("not allowed")
	ErrInvalid           = errors.New("invalid supplier details")
	ErrActiveApplication = errors.New("this account already has an application in progress")
	ErrIncomplete        = errors.New("the application is not complete")
	ErrSelfReview        = errors.New("staff cannot review a business they belong to")
	ErrNoteRequired      = errors.New("a note is required for this decision")
	ErrDocumentType      = errors.New("unsupported document type")
	ErrDocumentSize      = errors.New("document too large")
)

// ValidationError names the field that is wrong.
type ValidationError struct{ Field, Reason string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

// IncompleteError lists what is missing before submission.
type IncompleteError struct{ Missing []string }

func (e *IncompleteError) Error() string { return "missing: " + strings.Join(e.Missing, ", ") }
func (e *IncompleteError) Unwrap() error { return ErrIncomplete }

// ActiveApplicationError points at the application already in progress.
type ActiveApplicationError struct{ OrganizationID string }

func (e *ActiveApplicationError) Error() string { return ErrActiveApplication.Error() }
func (e *ActiveApplicationError) Unwrap() error { return ErrActiveApplication }

// Service applies the rules of supplier registration.
type Service struct {
	store domain.SupplierOnboardingStore
	orgs  domain.OrganizationRepository
	users domain.UserRepository
	audit domain.AuditLogger
}

// NewService builds the service; audit may be nil.
func NewService(store domain.SupplierOnboardingStore, orgs domain.OrganizationRepository, users domain.UserRepository, audit domain.AuditLogger) *Service {
	return &Service{store: store, orgs: orgs, users: users, audit: audit}
}

// Details is what an applicant declares; the same shape starts and updates an application.
type Details struct {
	OrgType      string
	LegalName    string
	DisplayName  string
	Description  string
	Phone        string
	Email        string
	Country      string
	Province     string
	City         string
	Address      string
	LegalForm    string
	RCCM         string
	IDNat        string
	NIF          string
	Website      string
	ContactName  string
	ContactRole  string
	Sectors      []string
	Capabilities string
	MinOrder     string
	LeadTimeDays *int
	PaymentTerms string
	ServiceAreas []string
}

var (
	allowedOrgTypes = map[domain.OrganizationType]bool{
		domain.OrganizationTypeIndividual: true, domain.OrganizationTypeBusiness: true, domain.OrganizationTypeCompany: true,
		domain.OrganizationTypeCooperative: true, domain.OrganizationTypeAssociation: true, domain.OrganizationTypeOther: true,
	}
	emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9 ]{6,20}$`)
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func clean(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

func cleanList(in []string, maxItems, maxLen int) []string {
	out, seen := []string{}, map[string]bool{}
	for _, v := range in {
		v = clean(v, maxLen)
		key := strings.ToLower(v)
		if v == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
		if len(out) == maxItems {
			break
		}
	}
	return out
}

// apply checks and normalises details onto an application.
func (d Details) apply(a *domain.SupplierApplication) error {
	t := domain.OrganizationType(strings.ToLower(strings.TrimSpace(d.OrgType)))
	if t == "" {
		t = domain.OrganizationTypeBusiness
	}
	if !allowedOrgTypes[t] {
		return &ValidationError{Field: "org_type", Reason: "unknown type"}
	}
	a.OrgType = t
	a.LegalName, a.DisplayName = clean(d.LegalName, 255), clean(d.DisplayName, 255)
	if a.DisplayName == "" {
		a.DisplayName = a.LegalName
	}
	if a.LegalName == "" {
		return &ValidationError{Field: "legal_name", Reason: "required"}
	}
	a.Description = clean(d.Description, 2000)
	a.Phone = clean(d.Phone, 20)
	if !phonePattern.MatchString(a.Phone) {
		return &ValidationError{Field: "phone", Reason: "a phone number is required"}
	}
	a.Email = strings.ToLower(clean(d.Email, 255))
	if a.Email != "" && !emailPattern.MatchString(a.Email) {
		return &ValidationError{Field: "email", Reason: "not an email address"}
	}
	a.Country = strings.ToUpper(clean(d.Country, 100))
	if a.Country == "" {
		a.Country = "CD"
	}
	a.Province, a.City, a.Address = clean(d.Province, 100), clean(d.City, 100), clean(d.Address, 500)
	a.LegalForm, a.RCCM, a.IDNat, a.NIF = clean(d.LegalForm, 40), clean(d.RCCM, 60), clean(d.IDNat, 60), clean(d.NIF, 60)
	a.Website = clean(d.Website, 255)
	if a.Website != "" && !strings.HasPrefix(a.Website, "https://") && !strings.HasPrefix(a.Website, "http://") {
		a.Website = "https://" + a.Website
	}
	a.ContactName, a.ContactRole = clean(d.ContactName, 120), clean(d.ContactRole, 80)
	a.Sectors = cleanList(d.Sectors, 10, 80)
	a.Capabilities = clean(d.Capabilities, 2000)
	a.MinOrder, a.PaymentTerms = clean(d.MinOrder, 200), clean(d.PaymentTerms, 300)
	if d.LeadTimeDays != nil && (*d.LeadTimeDays < 0 || *d.LeadTimeDays > 365) {
		return &ValidationError{Field: "lead_time_days", Reason: "between 0 and 365"}
	}
	a.LeadTimeDays = d.LeadTimeDays
	a.ServiceAreas = cleanList(d.ServiceAreas, 30, 100)
	return nil
}

func (s *Service) user(ctx context.Context, userID string) (*domain.User, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil || u == nil || u.Status == domain.UserStatusSuspended {
		return nil, ErrForbidden
	}
	return u, nil
}

func isStaff(u *domain.User) bool { return u.HasRole(domain.RoleAdmin) || u.HasRole(domain.RoleOperator) }

// role returns the caller's membership role in the business, or "".
func (s *Service) role(ctx context.Context, orgID, userID string) domain.OrganizationRole {
	r, err := s.orgs.GetMemberRole(ctx, orgID, userID)
	if err != nil {
		return ""
	}
	return r
}

func canEdit(r domain.OrganizationRole) bool {
	return r == domain.OrganizationRoleOwner || r == domain.OrganizationRoleAdmin || r == domain.OrganizationRoleManager
}

func canSubmit(r domain.OrganizationRole) bool {
	return r == domain.OrganizationRoleOwner || r == domain.OrganizationRoleAdmin
}

func (s *Service) record(ctx context.Context, actor, action, orgID string, meta map[string]interface{}) {
	if s.audit != nil {
		_ = s.audit.Record(ctx, domain.AuditEntry{ActorID: actor, Action: action, TargetType: "organization", TargetID: orgID, Metadata: meta})
	}
}

// Start registers a business for the caller, who becomes its owner. At most one application per person may be in
// progress (draft, submitted or changes requested).
func (s *Service) Start(ctx context.Context, userID string, d Details) (*domain.SupplierApplication, error) {
	if _, err := s.user(ctx, userID); err != nil {
		return nil, err
	}
	mine, err := s.store.ListByApplicant(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, a := range mine {
		if a.Status.Editable() || a.Status == domain.SupplierSubmitted {
			return nil, &ActiveApplicationError{OrganizationID: a.OrganizationID}
		}
	}
	app := &domain.SupplierApplication{ApplicantID: userID}
	if err := d.apply(app); err != nil {
		return nil, err
	}
	if err := s.store.Create(ctx, app); err != nil {
		return nil, err
	}
	s.record(ctx, userID, "supplier.registered", app.OrganizationID, map[string]interface{}{"display_name": app.DisplayName})
	return s.store.Find(ctx, app.OrganizationID)
}

// Mine lists the applications the caller registered.
func (s *Service) Mine(ctx context.Context, userID string) ([]domain.SupplierApplication, error) {
	if _, err := s.user(ctx, userID); err != nil {
		return nil, err
	}
	return s.store.ListByApplicant(ctx, userID)
}

// Get returns an application to a member of the business or to staff; anyone else gets "not found".
func (s *Service) Get(ctx context.Context, userID, orgID string) (*domain.SupplierApplication, error) {
	u, err := s.user(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !uuidPattern.MatchString(orgID) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	if !isStaff(u) && s.role(ctx, orgID, userID) == "" {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	return s.store.Find(ctx, orgID)
}

// Update saves new details while the application is editable (owner, admin or manager of the business).
func (s *Service) Update(ctx context.Context, userID, orgID string, d Details) (*domain.SupplierApplication, error) {
	if _, err := s.user(ctx, userID); err != nil {
		return nil, err
	}
	if !uuidPattern.MatchString(orgID) || !canEdit(s.role(ctx, orgID, userID)) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	app, err := s.store.Find(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !app.Status.Editable() {
		return nil, domain.ErrSupplierNotEditable
	}
	if err := d.apply(app); err != nil {
		return nil, err
	}
	if err := s.store.UpdateDetails(ctx, app); err != nil {
		return nil, err
	}
	return s.store.Find(ctx, orgID)
}

// documentTypes are the accepted documents, decided from their bytes.
var documentTypes = map[string]bool{"application/pdf": true, "image/jpeg": true, "image/png": true, "image/webp": true}

// AddDocument stores a document of the business while the application is editable.
func (s *Service) AddDocument(ctx context.Context, userID, orgID string, kind domain.SupplierDocumentKind, filename string, data []byte) (*domain.SupplierDocument, error) {
	if _, err := s.user(ctx, userID); err != nil {
		return nil, err
	}
	if !uuidPattern.MatchString(orgID) || !canEdit(s.role(ctx, orgID, userID)) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	if !kind.IsValid() {
		return nil, &ValidationError{Field: "kind", Reason: "unknown document kind"}
	}
	if len(data) == 0 || len(data) > domain.SupplierDocumentMaxBytes {
		return nil, ErrDocumentSize
	}
	contentType := http.DetectContentType(data)
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		contentType = "application/pdf"
	}
	if !documentTypes[contentType] {
		return nil, ErrDocumentType
	}
	name := clean(strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, path.Base(strings.ReplaceAll(filename, "\\", "/"))), 200)
	if name == "" || name == "." {
		name = string(kind)
	}
	sum := sha256.Sum256(data)
	doc := &domain.SupplierDocument{Kind: kind, Filename: name, ContentType: contentType, SizeBytes: len(data), SHA256: hex.EncodeToString(sum[:]), UploadedBy: userID}
	if err := s.store.AddDocument(ctx, orgID, doc, data); err != nil {
		return nil, err
	}
	s.record(ctx, userID, "supplier.document_added", orgID, map[string]interface{}{"kind": string(kind), "document_id": doc.ID, "sha256": doc.SHA256})
	return doc, nil
}

// Document returns a document's bytes to a member of the business or to staff.
func (s *Service) Document(ctx context.Context, userID, orgID, docID string) (*domain.SupplierDocument, []byte, error) {
	u, err := s.user(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if !uuidPattern.MatchString(orgID) || !uuidPattern.MatchString(docID) {
		return nil, nil, domain.ErrSupplierDocumentNotFound
	}
	if !isStaff(u) && s.role(ctx, orgID, userID) == "" {
		return nil, nil, domain.ErrSupplierDocumentNotFound
	}
	return s.store.DocumentData(ctx, orgID, docID)
}

// DeleteDocument removes a document while the application is editable.
func (s *Service) DeleteDocument(ctx context.Context, userID, orgID, docID string) error {
	if _, err := s.user(ctx, userID); err != nil {
		return err
	}
	if !uuidPattern.MatchString(orgID) || !uuidPattern.MatchString(docID) || !canEdit(s.role(ctx, orgID, userID)) {
		return domain.ErrSupplierDocumentNotFound
	}
	if err := s.store.DeleteDocument(ctx, orgID, docID); err != nil {
		return err
	}
	s.record(ctx, userID, "supplier.document_removed", orgID, map[string]interface{}{"document_id": docID})
	return nil
}

// Submit sends a complete application for verification (owner or admin of the business). Nothing becomes verified.
func (s *Service) Submit(ctx context.Context, userID, orgID string) (*domain.SupplierApplication, error) {
	if _, err := s.user(ctx, userID); err != nil {
		return nil, err
	}
	if !uuidPattern.MatchString(orgID) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	role := s.role(ctx, orgID, userID)
	if role == "" {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	if !canSubmit(role) {
		return nil, ErrForbidden
	}
	app, err := s.store.Find(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if !app.Status.Editable() {
		return nil, domain.ErrSupplierNotEditable
	}
	if missing := app.MissingForSubmission(); len(missing) > 0 {
		return nil, &IncompleteError{Missing: missing}
	}
	if err := s.store.Transition(ctx, orgID, app.Status, domain.SupplierSubmitted, "", "", domain.VerificationStatusPending); err != nil {
		return nil, err
	}
	s.record(ctx, userID, "supplier.submitted", orgID, map[string]interface{}{"documents": len(app.Documents)})
	return s.store.Find(ctx, orgID)
}

// Queue lists applications in one status for staff (submitted by default).
func (s *Service) Queue(ctx context.Context, staffID string, status domain.SupplierApplicationStatus) ([]domain.SupplierApplication, error) {
	u, err := s.user(ctx, staffID)
	if err != nil || !isStaff(u) {
		return nil, ErrForbidden
	}
	if status == "" {
		status = domain.SupplierSubmitted
	}
	return s.store.ListByStatus(ctx, status, 100)
}

// Review records the staff decision on a submitted application: verify, reject (note required) or ask for changes
// (note required). Staff never review a business they belong to.
func (s *Service) Review(ctx context.Context, staffID, orgID string, decision domain.SupplierReviewDecision, note string) (*domain.SupplierApplication, error) {
	u, err := s.user(ctx, staffID)
	if err != nil || !isStaff(u) {
		return nil, ErrForbidden
	}
	if !uuidPattern.MatchString(orgID) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	if s.role(ctx, orgID, staffID) != "" {
		return nil, ErrSelfReview
	}
	note = clean(note, 1000)
	var to domain.SupplierApplicationStatus
	var verification domain.VerificationStatus
	switch decision {
	case domain.SupplierDecisionVerify:
		to, verification = domain.SupplierVerified, domain.VerificationStatusVerified
	case domain.SupplierDecisionReject:
		to, verification = domain.SupplierRejected, domain.VerificationStatusRejected
	case domain.SupplierDecisionRequestChanges:
		to, verification = domain.SupplierChangesRequested, domain.VerificationStatusUnverified
	default:
		return nil, &ValidationError{Field: "decision", Reason: "verify, reject or request_changes"}
	}
	if to != domain.SupplierVerified && note == "" {
		return nil, ErrNoteRequired
	}
	if err := s.store.Transition(ctx, orgID, domain.SupplierSubmitted, to, staffID, note, verification); err != nil {
		if errors.Is(err, domain.ErrSupplierStatusChanged) {
			if _, findErr := s.store.Find(ctx, orgID); errors.Is(findErr, domain.ErrSupplierApplicationNotFound) {
				return nil, findErr
			}
		}
		return nil, err
	}
	s.record(ctx, staffID, "supplier.reviewed", orgID, map[string]interface{}{"decision": string(decision), "note": note})
	return s.store.Find(ctx, orgID)
}
