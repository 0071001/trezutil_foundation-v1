package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application/supplieronboarding"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
	"github.com/trezutil/trezutil/packages/auth"
)

func newUser(t *testing.T, db *TestDB, ctx context.Context, id, phone string, roles ...domain.Role) {
	t.Helper()
	users := postgres.NewUserRepository(db.DB)
	if err := users.Create(ctx, &domain.User{ID: id, Phone: phone, DisplayName: "U " + phone, Status: domain.UserStatusActive,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(), Roles: []domain.Role{}}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	for _, r := range roles {
		if err := users.AssignRole(ctx, id, r); err != nil {
			t.Fatalf("assign %s: %v", r, err)
		}
	}
}

// Supplier self-registration on the real schema (migration 000034): registering, documents, submission and the staff
// review are distinct steps, and only staff verify.
func TestSupplierOnboarding_FullFlow(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	// The cleanup empties roles; production seeds them (migration 000018).
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO roles (name) VALUES ('customer'), ('seller'), ('admin'), ('operator') ON CONFLICT (name) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	applicant := "123e4567-e89b-12d3-a456-426614174181"
	stranger := "123e4567-e89b-12d3-a456-426614174182"
	staff := "123e4567-e89b-12d3-a456-426614174183"
	newUser(t, db, ctx, applicant, "+243810000181")
	newUser(t, db, ctx, stranger, "+243810000182")
	newUser(t, db, ctx, staff, "+243810000183", domain.RoleAdmin)

	store := postgres.NewSupplierOnboardingStore(db.DB)
	svc := supplieronboarding.NewService(store, postgres.NewOrganizationRepository(db.DB), postgres.NewUserRepository(db.DB),
		audit.NewLogger(zerolog.Nop()).WithStore(postgres.NewAuditRepository(db.DB)))

	// 1. Registering: one transaction creates the business, its owner, an unverified seller profile and the seller role.
	app, err := svc.Start(ctx, applicant, supplieronboarding.Details{OrgType: "business", LegalName: "Ciment du Congo SARL", Phone: "+243 81 000 0181", City: "Kinshasa"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	org := app.OrganizationID
	if app.Status != domain.SupplierDraft || app.DisplayName != "Ciment du Congo SARL" {
		t.Fatalf("a new application is a draft: %+v", app)
	}
	var orgVerif, sellerVerif, memberRole string
	var trust int
	_ = db.DB.QueryRowContext(ctx, `SELECT o.verification_status, s.verification_status, s.trust_level, m.membership_role
		FROM organizations o JOIN seller_profiles s ON s.organization_id = o.id JOIN organization_members m ON m.organization_id = o.id
		WHERE o.id = $1`, org).Scan(&orgVerif, &sellerVerif, &trust, &memberRole)
	if orgVerif != "unverified" || sellerVerif != "unverified" || trust != 0 || memberRole != "owner" {
		t.Fatalf("registering must not verify anything: org %s seller %s trust %d role %s", orgVerif, sellerVerif, trust, memberRole)
	}
	if ok, _ := postgres.NewUserRepository(db.DB).HasRole(ctx, applicant, domain.RoleSeller); !ok {
		t.Fatal("the applicant gets the seller role")
	}
	var active *supplieronboarding.ActiveApplicationError
	if _, err := svc.Start(ctx, applicant, supplieronboarding.Details{LegalName: "Autre", Phone: "+243810000181"}); !errors.As(err, &active) || active.OrganizationID != org {
		t.Fatalf("one application in progress per account: %v", err)
	}

	// 2. Incomplete: submitting lists what is missing.
	var incomplete *supplieronboarding.IncompleteError
	if _, err := svc.Submit(ctx, applicant, org); !errors.As(err, &incomplete) {
		t.Fatalf("an incomplete application cannot be submitted: %v", err)
	}

	// 3. Details and documents.
	lead := 3
	if _, err := svc.Update(ctx, applicant, org, supplieronboarding.Details{OrgType: "company", LegalName: "Ciment du Congo SARL", DisplayName: "Ciment du Congo",
		Phone: "+243810000181", City: "Kinshasa", RCCM: "CD/KIN/RCCM/24-B-0001", Sectors: []string{"Matériaux de construction", "matériaux de construction"},
		Capabilities: "Ciment en sacs de 50 kg, 2 000 sacs par semaine", LeadTimeDays: &lead, ServiceAreas: []string{"Gombe", "Limete"}}); err != nil {
		t.Fatalf("update: %v", err)
	}
	pdf := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF")
	doc, err := svc.AddDocument(ctx, applicant, org, domain.SupplierDocRCCM, `C:\scans\rccm.pdf`, pdf)
	if err != nil || doc.ContentType != "application/pdf" || doc.Filename != "rccm.pdf" {
		t.Fatalf("add document: %+v %v", doc, err)
	}
	if _, err := svc.AddDocument(ctx, applicant, org, domain.SupplierDocOther, "again.pdf", pdf); !errors.Is(err, domain.ErrSupplierDuplicateDocument) {
		t.Fatalf("the same file twice is refused: %v", err)
	}
	if _, err := svc.AddDocument(ctx, applicant, org, domain.SupplierDocOther, "x.html", []byte("<html><script>alert(1)</script>")); !errors.Is(err, supplieronboarding.ErrDocumentType) {
		t.Fatalf("an HTML file is refused whatever its name: %v", err)
	}
	if got, _ := svc.Get(ctx, applicant, org); len(got.Sectors) != 1 || len(got.MissingForSubmission()) != 0 {
		t.Fatalf("sectors are de-duplicated and the application is complete: %+v missing %v", got.Sectors, got.MissingForSubmission())
	}

	// 4. A stranger sees nothing and changes nothing.
	if _, err := svc.Get(ctx, stranger, org); !errors.Is(err, domain.ErrSupplierApplicationNotFound) {
		t.Fatalf("a stranger must get not found: %v", err)
	}
	if _, _, err := svc.Document(ctx, stranger, org, doc.ID); !errors.Is(err, domain.ErrSupplierDocumentNotFound) {
		t.Fatalf("a stranger cannot read a document: %v", err)
	}
	if _, err := svc.AddDocument(ctx, stranger, org, domain.SupplierDocOther, "y.pdf", []byte("%PDF-1.4 other")); !errors.Is(err, domain.ErrSupplierApplicationNotFound) {
		t.Fatalf("a stranger cannot add a document: %v", err)
	}

	// 5. Submitted: waiting for staff, locked, not verified.
	sub, err := svc.Submit(ctx, applicant, org)
	if err != nil || sub.Status != domain.SupplierSubmitted || sub.SubmittedAt == 0 {
		t.Fatalf("submit: %+v %v", sub, err)
	}
	_ = db.DB.QueryRowContext(ctx, `SELECT verification_status FROM organizations WHERE id = $1`, org).Scan(&orgVerif)
	if orgVerif != "pending" {
		t.Fatalf("submitted means pending, not verified: %s", orgVerif)
	}
	if _, err := svc.Update(ctx, applicant, org, supplieronboarding.Details{LegalName: "X", Phone: "+243810000181"}); !errors.Is(err, domain.ErrSupplierNotEditable) {
		t.Fatalf("a submitted application is locked: %v", err)
	}
	if _, err := svc.AddDocument(ctx, applicant, org, domain.SupplierDocOther, "z.pdf", []byte("%PDF-1.4 late")); !errors.Is(err, domain.ErrSupplierNotEditable) {
		t.Fatalf("no document after submission: %v", err)
	}

	// 6. The review is staff's: not the applicant, not a non-staff user, not staff who belong to the business.
	if _, err := svc.Review(ctx, applicant, org, domain.SupplierDecisionVerify, ""); !errors.Is(err, supplieronboarding.ErrForbidden) {
		t.Fatalf("an applicant cannot verify themself: %v", err)
	}
	if _, err := svc.Review(ctx, stranger, org, domain.SupplierDecisionVerify, ""); !errors.Is(err, supplieronboarding.ErrForbidden) {
		t.Fatalf("a non-staff user cannot verify: %v", err)
	}
	if _, err := svc.Review(ctx, staff, org, domain.SupplierDecisionRequestChanges, ""); !errors.Is(err, supplieronboarding.ErrNoteRequired) {
		t.Fatalf("asking for changes needs a note: %v", err)
	}
	back, err := svc.Review(ctx, staff, org, domain.SupplierDecisionRequestChanges, "Ajoutez le NIF.")
	if err != nil || back.Status != domain.SupplierChangesRequested || back.ReviewNote != "Ajoutez le NIF." || !back.Status.Editable() {
		t.Fatalf("changes requested: %+v %v", back, err)
	}
	if _, err := svc.AddDocument(ctx, applicant, org, domain.SupplierDocNIF, "nif.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 64)...)); err != nil {
		t.Fatalf("the owner can complete it again: %v", err)
	}
	if _, err := svc.Submit(ctx, applicant, org); err != nil {
		t.Fatalf("submit again: %v", err)
	}

	// 7. Verified by staff: the business, its seller profile and its documents.
	done, err := svc.Review(ctx, staff, org, domain.SupplierDecisionVerify, "")
	if err != nil || done.Status != domain.SupplierVerified || done.ReviewedAt == 0 {
		t.Fatalf("verify: %+v %v", done, err)
	}
	_ = db.DB.QueryRowContext(ctx, `SELECT o.verification_status, s.verification_status FROM organizations o JOIN seller_profiles s ON s.organization_id = o.id WHERE o.id = $1`, org).Scan(&orgVerif, &sellerVerif)
	if orgVerif != "verified" || sellerVerif != "verified" {
		t.Fatalf("verified by staff: org %s seller %s", orgVerif, sellerVerif)
	}
	for _, d := range done.Documents {
		if d.Status != "accepted" {
			t.Fatalf("documents of a verified business are accepted: %+v", d)
		}
	}
	if _, err := svc.Review(ctx, staff, org, domain.SupplierDecisionReject, "trop tard"); !errors.Is(err, domain.ErrSupplierStatusChanged) {
		t.Fatalf("a decided application cannot be decided again: %v", err)
	}

	// 8. Staff who belong to a business cannot review it.
	member := "123e4567-e89b-12d3-a456-426614174184"
	newUser(t, db, ctx, member, "+243810000184", domain.RoleOperator)
	own, err := svc.Start(ctx, member, supplieronboarding.Details{LegalName: "Opérateur SARL", Phone: "+243810000184"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Review(ctx, member, own.OrganizationID, domain.SupplierDecisionVerify, ""); !errors.Is(err, supplieronboarding.ErrSelfReview) {
		t.Fatalf("no self-review: %v", err)
	}

	var audited int
	_ = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM audit_events WHERE target_id = $1 AND action LIKE 'supplier.%'`, org).Scan(&audited)
	if audited < 6 {
		t.Fatalf("each step is audited, got %d events", audited)
	}
}

// The HTTP side: multipart upload, the document served with safe headers, guests refused, the queue for staff only.
func TestSupplierOnboarding_HTTP(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO roles (name) VALUES ('seller'), ('admin') ON CONFLICT (name) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	owner, staff := "123e4567-e89b-12d3-a456-426614174191", "123e4567-e89b-12d3-a456-426614174192"
	newUser(t, db, ctx, owner, "+243810000191")
	newUser(t, db, ctx, staff, "+243810000192", domain.RoleAdmin)
	h := api.NewSupplierOnboardingHandler(supplieronboarding.NewService(postgres.NewSupplierOnboardingStore(db.DB),
		postgres.NewOrganizationRepository(db.DB), postgres.NewUserRepository(db.DB), nil), zerolog.Nop())
	mux := http.NewServeMux()
	h.Register(mux)
	call := func(user string, req *http.Request) *httptest.ResponseRecorder {
		if user != "" {
			req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	if rec := call("", httptest.NewRequest(http.MethodGet, "/api/v1/supplier/applications", nil)); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a guest: want 401, got %d", rec.Code)
	}
	rec := call(owner, httptest.NewRequest(http.MethodPost, "/api/v1/supplier/applications", strings.NewReader(`{"legal_name":"Quincaillerie Gombe","phone":"+243810000191"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		OrganizationID string   `json:"organization_id"`
		Status         string   `json:"status"`
		Missing        []string `json:"missing"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Status != "draft" || len(created.Missing) == 0 {
		t.Fatalf("a draft with what is missing: %+v", created)
	}

	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	_ = form.WriteField("kind", "identity")
	part, _ := form.CreateFormFile("file", "carte.pdf")
	_, _ = part.Write([]byte("%PDF-1.7\nidentity\n%%EOF"))
	_ = form.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/supplier/applications/"+created.OrganizationID+"/documents", body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec = call(owner, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var doc struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &doc)

	rec = call(owner, httptest.NewRequest(http.MethodGet, "/api/v1/supplier/applications/"+created.OrganizationID+"/documents/"+doc.ID, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") || !strings.HasPrefix(rec.Body.String(), "%PDF") {
		t.Fatalf("document download: %d %v", rec.Code, rec.Header())
	}

	if rec := call(owner, httptest.NewRequest(http.MethodGet, "/api/v1/admin/supplier-applications", nil)); rec.Code != http.StatusForbidden {
		t.Fatalf("the queue is for staff: %d", rec.Code)
	}
	if rec := call(staff, httptest.NewRequest(http.MethodGet, "/api/v1/admin/supplier-applications?status=draft", nil)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), created.OrganizationID) {
		t.Fatalf("staff queue: %d %s", rec.Code, rec.Body)
	}
	if rec := call(owner, httptest.NewRequest(http.MethodPost, "/api/v1/supplier/applications/"+created.OrganizationID+"/submit", nil)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "service_areas") {
		t.Fatalf("an incomplete submission names what is missing: %d %s", rec.Code, rec.Body)
	}
}
