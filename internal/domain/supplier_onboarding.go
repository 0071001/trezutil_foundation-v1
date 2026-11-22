package domain

import (
	"context"
	"errors"
)

// SupplierApplicationStatus is where a supplier's registration stands (migration 000034). Registering never makes a
// supplier verified: only Trezutil staff decide, after the application was submitted complete.
type SupplierApplicationStatus string

const (
	SupplierDraft            SupplierApplicationStatus = "draft"             // registered; the owner fills it in
	SupplierSubmitted        SupplierApplicationStatus = "submitted"         // complete, documents sent, awaiting staff
	SupplierChangesRequested SupplierApplicationStatus = "changes_requested" // staff asked for changes; editable again
	SupplierVerified         SupplierApplicationStatus = "verified"          // staff verified the business
	SupplierRejected         SupplierApplicationStatus = "rejected"          // staff refused it
)

// Editable reports whether the owner may change the application (and its documents).
func (s SupplierApplicationStatus) Editable() bool {
	return s == SupplierDraft || s == SupplierChangesRequested
}

// SupplierApplication is a business registered as a supplier, with what it declared and the review.
type SupplierApplication struct {
	OrganizationID string
	ApplicantID    string
	Status         SupplierApplicationStatus

	// The business (stored in organizations).
	OrgType     OrganizationType
	LegalName   string
	DisplayName string
	Description string
	Phone       string
	Email       string
	Country     string
	Province    string
	City        string
	Address     string

	// What a supplier adds.
	LegalForm    string
	RCCM         string // Registre du Commerce et du Crédit Mobilier
	IDNat        string // identification nationale
	NIF          string // numéro d'impôt
	Website      string
	ContactName  string
	ContactRole  string
	Sectors      []string
	Capabilities string
	MinOrder     string
	LeadTimeDays *int
	PaymentTerms string
	ServiceAreas []string

	SubmittedAt int64
	ReviewedAt  int64
	ReviewerID  string
	ReviewNote  string
	CreatedAt   int64
	UpdatedAt   int64

	Documents []SupplierDocument
}

// SupplierDocumentKind is what a document proves.
type SupplierDocumentKind string

const (
	SupplierDocRCCM     SupplierDocumentKind = "rccm"
	SupplierDocIDNat    SupplierDocumentKind = "id_nat"
	SupplierDocNIF      SupplierDocumentKind = "nif"
	SupplierDocIdentity SupplierDocumentKind = "identity"
	SupplierDocOther    SupplierDocumentKind = "other"
)

// IsValid reports whether k is a known kind.
func (k SupplierDocumentKind) IsValid() bool {
	switch k {
	case SupplierDocRCCM, SupplierDocIDNat, SupplierDocNIF, SupplierDocIdentity, SupplierDocOther:
		return true
	}
	return false
}

// SupplierDocument is one document sent with an application (its bytes are read separately).
type SupplierDocument struct {
	ID          string
	Kind        SupplierDocumentKind
	Filename    string
	ContentType string
	SizeBytes   int
	SHA256      string
	Status      string // submitted | accepted | rejected
	UploadedBy  string
	CreatedAt   int64
}

// Limits of an application.
const (
	SupplierDocumentMaxBytes = 5 << 20
	SupplierMaxDocuments     = 10
)

// MissingForSubmission lists what must be filled in before the application can be submitted ("" fields by name),
// empty when it is complete. A legal document (RCCM, national id number or an identity document) is required.
func (a *SupplierApplication) MissingForSubmission() []string {
	missing := []string{}
	need := func(ok bool, name string) {
		if !ok {
			missing = append(missing, name)
		}
	}
	need(a.LegalName != "", "legal_name")
	need(a.DisplayName != "", "display_name")
	need(a.Phone != "", "phone")
	need(a.City != "", "city")
	need(len(a.Sectors) > 0, "sectors")
	need(a.Capabilities != "", "capabilities")
	need(len(a.ServiceAreas) > 0, "service_areas")
	legal := false
	for _, d := range a.Documents {
		if d.Kind == SupplierDocRCCM || d.Kind == SupplierDocIDNat || d.Kind == SupplierDocIdentity {
			legal = true
		}
	}
	need(legal, "legal_document")
	return missing
}

// SupplierReviewDecision is what staff decide on a submitted application.
type SupplierReviewDecision string

const (
	SupplierDecisionVerify         SupplierReviewDecision = "verify"
	SupplierDecisionReject         SupplierReviewDecision = "reject"
	SupplierDecisionRequestChanges SupplierReviewDecision = "request_changes"
)

var (
	ErrSupplierApplicationNotFound = errors.New("supplier application not found")
	ErrSupplierNotEditable         = errors.New("the application cannot be changed in its current status")
	ErrSupplierStatusChanged       = errors.New("the application changed meanwhile")
	ErrSupplierDocumentNotFound    = errors.New("supplier document not found")
	ErrSupplierDuplicateDocument   = errors.New("this document was already sent")
)

// SupplierOnboardingStore keeps supplier applications. Create is one transaction: the organisation, its owner, the
// seller profile, the seller role and the application exist together or not at all.
type SupplierOnboardingStore interface {
	Create(ctx context.Context, app *SupplierApplication) error
	Find(ctx context.Context, organizationID string) (*SupplierApplication, error)
	ListByApplicant(ctx context.Context, userID string) ([]SupplierApplication, error)
	ListByStatus(ctx context.Context, status SupplierApplicationStatus, limit int) ([]SupplierApplication, error)
	// UpdateDetails saves the declared fields (organisation and application) while the status is still editable.
	UpdateDetails(ctx context.Context, app *SupplierApplication) error
	// Transition moves the application from one status to another (compare-and-set) and applies the verification
	// status to the organisation and its seller profile in the same transaction.
	Transition(ctx context.Context, organizationID string, from, to SupplierApplicationStatus, reviewerID, note string, orgVerification VerificationStatus) error
	AddDocument(ctx context.Context, organizationID string, doc *SupplierDocument, data []byte) error
	DocumentData(ctx context.Context, organizationID, documentID string) (*SupplierDocument, []byte, error)
	DeleteDocument(ctx context.Context, organizationID, documentID string) error
}
