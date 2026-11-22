package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/domain"
)

// SupplierOnboardingStore implements domain.SupplierOnboardingStore (migration 000034).
type SupplierOnboardingStore struct {
	db *sql.DB
}

// NewSupplierOnboardingStore creates the store.
func NewSupplierOnboardingStore(db *sql.DB) *SupplierOnboardingStore {
	return &SupplierOnboardingStore{db: db}
}

func nullableUUID(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Create registers the business in one transaction: the organisation (unverified), the applicant as its owner, a
// seller profile (unverified, trust 0), the seller role, and the draft application.
func (s *SupplierOnboardingStore) Create(ctx context.Context, app *domain.SupplierApplication) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin supplier registration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO organizations (type, legal_name, display_name, description, phone, email, country, province, city, address, status, verification_status)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, NULLIF($6, ''), $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), 'active', 'unverified')
		RETURNING id`,
		string(app.OrgType), app.LegalName, app.DisplayName, app.Description, app.Phone, app.Email, app.Country,
		app.Province, app.City, app.Address).Scan(&app.OrganizationID)
	if err != nil {
		return fmt.Errorf("create organisation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO organization_members (organization_id, user_id, membership_role, status) VALUES ($1, $2, 'owner', 'active')`,
		app.OrganizationID, app.ApplicantID); err != nil {
		return fmt.Errorf("add owner: %w", err)
	}
	sellerType := domain.SellerTypeBusiness
	if app.OrgType == domain.OrganizationTypeIndividual {
		sellerType = domain.SellerTypeIndividual
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO seller_profiles (user_id, organization_id, seller_type, display_name, bio, phone, email, status, verification_status, trust_level)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), 'active', 'unverified', 0)`,
		app.ApplicantID, app.OrganizationID, string(sellerType), app.DisplayName, app.Description, app.Phone, app.Email); err != nil {
		return fmt.Errorf("create seller profile: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'seller'
		ON CONFLICT DO NOTHING`, app.ApplicantID); err != nil {
		return fmt.Errorf("grant seller role: %w", err)
	}
	app.Status = domain.SupplierDraft
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_applications (organization_id, applicant_id, status, legal_form, rccm, id_nat, nif, website, contact_name, contact_role,
		                                   sectors, capabilities, min_order, lead_time_days, payment_terms, service_areas)
		VALUES ($1, $2, 'draft', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		app.OrganizationID, app.ApplicantID, app.LegalForm, app.RCCM, app.IDNat, app.NIF, app.Website, app.ContactName, app.ContactRole,
		pq.Array(nonNil(app.Sectors)), app.Capabilities, app.MinOrder, app.LeadTimeDays, app.PaymentTerms, pq.Array(nonNil(app.ServiceAreas))); err != nil {
		return fmt.Errorf("create application: %w", err)
	}
	return tx.Commit()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

const applicationColumns = `
	a.organization_id, a.applicant_id, a.status, o.type, o.legal_name, o.display_name, COALESCE(o.description, ''), o.phone,
	COALESCE(o.email, ''), o.country, COALESCE(o.province, ''), COALESCE(o.city, ''), COALESCE(o.address, ''),
	a.legal_form, a.rccm, a.id_nat, a.nif, a.website, a.contact_name, a.contact_role, a.sectors, a.capabilities, a.min_order,
	a.lead_time_days, a.payment_terms, a.service_areas, a.submitted_at, a.reviewed_at, COALESCE(a.reviewer_id::text, ''),
	a.review_note, a.created_at, a.updated_at`

func scanApplication(row rowScanner) (*domain.SupplierApplication, error) {
	var a domain.SupplierApplication
	var status, orgType string
	var sectors, areas pq.StringArray
	var lead sql.NullInt64
	var submitted, reviewed sql.NullTime
	var created, updated time.Time
	if err := row.Scan(&a.OrganizationID, &a.ApplicantID, &status, &orgType, &a.LegalName, &a.DisplayName, &a.Description, &a.Phone,
		&a.Email, &a.Country, &a.Province, &a.City, &a.Address,
		&a.LegalForm, &a.RCCM, &a.IDNat, &a.NIF, &a.Website, &a.ContactName, &a.ContactRole, &sectors, &a.Capabilities, &a.MinOrder,
		&lead, &a.PaymentTerms, &areas, &submitted, &reviewed, &a.ReviewerID, &a.ReviewNote, &created, &updated); err != nil {
		return nil, err
	}
	a.Status = domain.SupplierApplicationStatus(status)
	a.OrgType = domain.OrganizationType(orgType)
	a.Sectors, a.ServiceAreas = []string(sectors), []string(areas)
	if lead.Valid {
		v := int(lead.Int64)
		a.LeadTimeDays = &v
	}
	if submitted.Valid {
		a.SubmittedAt = submitted.Time.Unix()
	}
	if reviewed.Valid {
		a.ReviewedAt = reviewed.Time.Unix()
	}
	a.CreatedAt, a.UpdatedAt = created.Unix(), updated.Unix()
	return &a, nil
}

// Find returns one application with its documents (without their bytes).
func (s *SupplierOnboardingStore) Find(ctx context.Context, organizationID string) (*domain.SupplierApplication, error) {
	app, err := scanApplication(s.db.QueryRowContext(ctx, `SELECT `+applicationColumns+`
		FROM supplier_applications a JOIN organizations o ON o.id = a.organization_id WHERE a.organization_id = $1`, organizationID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSupplierApplicationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read application: %w", err)
	}
	if app.Documents, err = s.documents(ctx, organizationID); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *SupplierOnboardingStore) documents(ctx context.Context, organizationID string) ([]domain.SupplierDocument, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, filename, content_type, size_bytes, sha256, status, COALESCE(uploaded_by::text, ''), created_at
		FROM supplier_documents WHERE organization_id = $1 ORDER BY created_at, id`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()
	out := []domain.SupplierDocument{}
	for rows.Next() {
		var d domain.SupplierDocument
		var kind string
		var created time.Time
		if err := rows.Scan(&d.ID, &kind, &d.Filename, &d.ContentType, &d.SizeBytes, &d.SHA256, &d.Status, &d.UploadedBy, &created); err != nil {
			return nil, err
		}
		d.Kind, d.CreatedAt = domain.SupplierDocumentKind(kind), created.Unix()
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *SupplierOnboardingStore) list(ctx context.Context, where string, args ...interface{}) ([]domain.SupplierApplication, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+applicationColumns+`
		FROM supplier_applications a JOIN organizations o ON o.id = a.organization_id `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()
	out := []domain.SupplierApplication{}
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *app)
	}
	return out, rows.Err()
}

// ListByApplicant returns the applications of the businesses the user registered, newest first.
func (s *SupplierOnboardingStore) ListByApplicant(ctx context.Context, userID string) ([]domain.SupplierApplication, error) {
	return s.list(ctx, `WHERE a.applicant_id = $1 ORDER BY a.created_at DESC`, userID)
}

// ListByStatus returns applications in one status, the oldest submission first (the review queue).
func (s *SupplierOnboardingStore) ListByStatus(ctx context.Context, status domain.SupplierApplicationStatus, limit int) ([]domain.SupplierApplication, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return s.list(ctx, `WHERE a.status = $1 ORDER BY a.submitted_at NULLS LAST, a.created_at LIMIT $2`, string(status), limit)
}

// UpdateDetails saves the declared fields while the application is editable (checked in the same statements).
func (s *SupplierOnboardingStore) UpdateDetails(ctx context.Context, app *domain.SupplierApplication) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `
		UPDATE supplier_applications SET legal_form = $2, rccm = $3, id_nat = $4, nif = $5, website = $6, contact_name = $7, contact_role = $8,
		       sectors = $9, capabilities = $10, min_order = $11, lead_time_days = $12, payment_terms = $13, service_areas = $14, updated_at = NOW()
		WHERE organization_id = $1 AND status IN ('draft', 'changes_requested')`,
		app.OrganizationID, app.LegalForm, app.RCCM, app.IDNat, app.NIF, app.Website, app.ContactName, app.ContactRole,
		pq.Array(nonNil(app.Sectors)), app.Capabilities, app.MinOrder, app.LeadTimeDays, app.PaymentTerms, pq.Array(nonNil(app.ServiceAreas)))
	if err != nil {
		return fmt.Errorf("update application: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSupplierNotEditable
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE organizations SET type = $2, legal_name = $3, display_name = $4, description = NULLIF($5, ''), phone = $6, email = NULLIF($7, ''),
		       country = $8, province = NULLIF($9, ''), city = NULLIF($10, ''), address = NULLIF($11, ''), updated_at = NOW()
		WHERE id = $1`,
		app.OrganizationID, string(app.OrgType), app.LegalName, app.DisplayName, app.Description, app.Phone, app.Email, app.Country,
		app.Province, app.City, app.Address); err != nil {
		return fmt.Errorf("update organisation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE seller_profiles SET display_name = $2, bio = NULLIF($3, ''), phone = $4, email = NULLIF($5, ''), updated_at = NOW()
		WHERE organization_id = $1`, app.OrganizationID, app.DisplayName, app.Description, app.Phone, app.Email); err != nil {
		return fmt.Errorf("update seller profile: %w", err)
	}
	return tx.Commit()
}

// Transition changes the status only if it is still `from`, records the review (for staff decisions), and sets the
// verification status of the organisation and of its seller profiles. Verified documents become accepted.
func (s *SupplierOnboardingStore) Transition(ctx context.Context, organizationID string, from, to domain.SupplierApplicationStatus, reviewerID, note string, orgVerification domain.VerificationStatus) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var res sql.Result
	if to == domain.SupplierSubmitted {
		res, err = tx.ExecContext(ctx, `UPDATE supplier_applications SET status = $3, submitted_at = NOW(), updated_at = NOW()
			WHERE organization_id = $1 AND status = $2`, organizationID, string(from), string(to))
	} else {
		res, err = tx.ExecContext(ctx, `UPDATE supplier_applications SET status = $3, reviewed_at = NOW(), reviewer_id = $4, review_note = $5, updated_at = NOW()
			WHERE organization_id = $1 AND status = $2`, organizationID, string(from), string(to), nullableUUID(reviewerID), note)
	}
	if err != nil {
		return fmt.Errorf("change application status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSupplierStatusChanged
	}
	if _, err := tx.ExecContext(ctx, `UPDATE organizations SET verification_status = $2, updated_at = NOW() WHERE id = $1`, organizationID, string(orgVerification)); err != nil {
		return fmt.Errorf("set organisation verification: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE seller_profiles SET verification_status = $2, updated_at = NOW() WHERE organization_id = $1`, organizationID, string(orgVerification)); err != nil {
		return fmt.Errorf("set seller verification: %w", err)
	}
	if to == domain.SupplierVerified {
		if _, err := tx.ExecContext(ctx, `UPDATE supplier_documents SET status = 'accepted' WHERE organization_id = $1`, organizationID); err != nil {
			return fmt.Errorf("accept documents: %w", err)
		}
	}
	return tx.Commit()
}

// AddDocument stores a document while the application is editable and below the document limit.
func (s *SupplierOnboardingStore) AddDocument(ctx context.Context, organizationID string, doc *domain.SupplierDocument, data []byte) error {
	var created time.Time
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO supplier_documents (organization_id, kind, filename, content_type, size_bytes, sha256, data, uploaded_by)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8
		FROM supplier_applications a
		WHERE a.organization_id = $1 AND a.status IN ('draft', 'changes_requested')
		  AND (SELECT count(*) FROM supplier_documents d WHERE d.organization_id = $1) < $9
		RETURNING id, status, created_at`,
		organizationID, string(doc.Kind), doc.Filename, doc.ContentType, doc.SizeBytes, doc.SHA256, data, nullableUUID(doc.UploadedBy),
		domain.SupplierMaxDocuments).Scan(&doc.ID, &doc.Status, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrSupplierNotEditable
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return domain.ErrSupplierDuplicateDocument
	}
	if err != nil {
		return fmt.Errorf("store document: %w", err)
	}
	doc.CreatedAt = created.Unix()
	return nil
}

// DocumentData returns one document with its bytes.
func (s *SupplierOnboardingStore) DocumentData(ctx context.Context, organizationID, documentID string) (*domain.SupplierDocument, []byte, error) {
	var d domain.SupplierDocument
	var kind string
	var data []byte
	var created time.Time
	err := s.db.QueryRowContext(ctx, `SELECT id, kind, filename, content_type, size_bytes, sha256, status, created_at, data
		FROM supplier_documents WHERE organization_id = $1 AND id = $2`, organizationID, documentID).
		Scan(&d.ID, &kind, &d.Filename, &d.ContentType, &d.SizeBytes, &d.SHA256, &d.Status, &created, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, domain.ErrSupplierDocumentNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read document: %w", err)
	}
	d.Kind, d.CreatedAt = domain.SupplierDocumentKind(kind), created.Unix()
	return &d, data, nil
}

// DeleteDocument removes a document while the application is editable.
func (s *SupplierOnboardingStore) DeleteDocument(ctx context.Context, organizationID, documentID string) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM supplier_documents d USING supplier_applications a
		WHERE d.organization_id = $1 AND d.id = $2 AND a.organization_id = d.organization_id AND a.status IN ('draft', 'changes_requested')`,
		organizationID, documentID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrSupplierNotEditable
	}
	return nil
}

var _ domain.SupplierOnboardingStore = (*SupplierOnboardingStore)(nil)
