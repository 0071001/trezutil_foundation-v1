package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// organizationRow represents a row from the organizations table.
type organizationRow struct {
	ID                 string
	Type               string
	LegalName          string
	DisplayName        string
	Description        sql.NullString
	Phone              string
	Email              sql.NullString
	Status             string
	Country            string
	Province           sql.NullString
	City               sql.NullString
	Address            sql.NullString
	VerificationStatus string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// sellerProfileRow represents a row from the seller_profiles table.
type sellerProfileRow struct {
	ID                 string
	UserID             sql.NullString
	OrganizationID     sql.NullString
	SellerType         string
	DisplayName        string
	Bio                sql.NullString
	Phone              string
	Email              sql.NullString
	Status             string
	VerificationStatus string
	TrustLevel         int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// storeRow represents a row from the stores table.
type storeRow struct {
	ID             string
	OrganizationID string
	Name           string
	Slug           string
	Description    sql.NullString
	Phone          string
	Email          sql.NullString
	Country        string
	Province       sql.NullString
	City           sql.NullString
	Address        sql.NullString
	Latitude       sql.NullFloat64
	Longitude      sql.NullFloat64
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// toTime converts an int64 Unix timestamp to time.Time.
func toTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}
	return time.Unix(ts, 0)
}

// toUnix converts a time.Time to int64 Unix timestamp.
func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// SellerRepository implements domain.SellerRepository using PostgreSQL.
type SellerRepository struct {
	db *sql.DB
}

// NewSellerRepository creates a new SellerRepository.
func NewSellerRepository(db *sql.DB) *SellerRepository {
	return &SellerRepository{db: db}
}

// Create inserts a new seller profile into the database.
func (r *SellerRepository) Create(ctx context.Context, seller *domain.SellerProfile) error {
	query := `INSERT INTO seller_profiles (id, user_id, organization_id, seller_type, display_name, bio, phone, email, status, verification_status, trust_level, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var userID, orgID, bio, email sql.NullString
	if seller.UserID != nil {
		userID = sql.NullString{String: *seller.UserID, Valid: true}
	}
	if seller.OrganizationID != nil {
		orgID = sql.NullString{String: *seller.OrganizationID, Valid: true}
	}
	if seller.Bio != nil {
		bio = sql.NullString{String: *seller.Bio, Valid: true}
	}
	if seller.Email != nil {
		email = sql.NullString{String: *seller.Email, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, seller.ID, userID, orgID, seller.SellerType, seller.DisplayName, bio, seller.Phone, email, seller.Status, seller.VerificationStatus, seller.TrustLevel, toTime(seller.CreatedAt), toTime(seller.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create seller profile: %w", err)
	}
	return nil
}

// FindByID retrieves a seller profile by ID.
func (r *SellerRepository) FindByID(ctx context.Context, id string) (*domain.SellerProfile, error) {
	query := `SELECT id, user_id, organization_id, seller_type, display_name, bio, phone, email, status, verification_status, trust_level, created_at, updated_at FROM seller_profiles WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u sellerProfileRow
	var userID, orgID, bio, email sql.NullString
	err := row.Scan(&u.ID, &userID, &orgID, &u.SellerType, &u.DisplayName, &bio, &u.Phone, &email, &u.Status, &u.VerificationStatus, &u.TrustLevel, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to find seller profile: %w", err)
	}

	seller := &domain.SellerProfile{
		ID:                 u.ID,
		SellerType:         domain.SellerType(u.SellerType),
		DisplayName:        u.DisplayName,
		Phone:              u.Phone,
		Status:             domain.SellerStatus(u.Status),
		VerificationStatus: domain.VerificationStatus(u.VerificationStatus),
		TrustLevel:         u.TrustLevel,
		CreatedAt:          toUnix(u.CreatedAt),
		UpdatedAt:          toUnix(u.UpdatedAt),
	}

	if userID.Valid {
		uid := userID.String
		seller.UserID = &uid
	}
	if orgID.Valid {
		oid := orgID.String
		seller.OrganizationID = &oid
	}
	if bio.Valid {
		b := bio.String
		seller.Bio = &b
	}
	if email.Valid {
		e := email.String
		seller.Email = &e
	}

	return seller, nil
}

// Update updates an existing seller profile.
func (r *SellerRepository) Update(ctx context.Context, seller *domain.SellerProfile) error {
	query := `UPDATE seller_profiles SET seller_type = $1, display_name = $2, bio = $3, phone = $4, email = $5, status = $6, verification_status = $7, trust_level = $8, updated_at = $9 WHERE id = $10`

	var bio, email sql.NullString
	if seller.Bio != nil {
		bio = sql.NullString{String: *seller.Bio, Valid: true}
	}
	if seller.Email != nil {
		email = sql.NullString{String: *seller.Email, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, seller.SellerType, seller.DisplayName, bio, seller.Phone, email, seller.Status, seller.VerificationStatus, seller.TrustLevel, toTime(seller.UpdatedAt), seller.ID)
	if err != nil {
		return fmt.Errorf("failed to update seller profile: %w", err)
	}
	return nil
}

// FindByUserID retrieves a seller profile by user ID.
func (r *SellerRepository) FindByUserID(ctx context.Context, userID string) (*domain.SellerProfile, error) {
	query := `SELECT id, user_id, organization_id, seller_type, display_name, bio, phone, email, status, verification_status, trust_level, created_at, updated_at FROM seller_profiles WHERE user_id = $1`
	row := r.db.QueryRowContext(ctx, query, userID)

	var u sellerProfileRow
	var userIDVal, orgID, bio, email sql.NullString
	err := row.Scan(&u.ID, &userIDVal, &orgID, &u.SellerType, &u.DisplayName, &bio, &u.Phone, &email, &u.Status, &u.VerificationStatus, &u.TrustLevel, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrSellerNotFound
		}
		return nil, fmt.Errorf("failed to find seller profile by user ID: %w", err)
	}

	seller := &domain.SellerProfile{
		ID:                 u.ID,
		SellerType:         domain.SellerType(u.SellerType),
		DisplayName:        u.DisplayName,
		Phone:              u.Phone,
		Status:             domain.SellerStatus(u.Status),
		VerificationStatus: domain.VerificationStatus(u.VerificationStatus),
		TrustLevel:         u.TrustLevel,
		CreatedAt:          toUnix(u.CreatedAt),
		UpdatedAt:          toUnix(u.UpdatedAt),
	}

	if userIDVal.Valid {
		uid := userIDVal.String
		seller.UserID = &uid
	}
	if orgID.Valid {
		oid := orgID.String
		seller.OrganizationID = &oid
	}
	if bio.Valid {
		b := bio.String
		seller.Bio = &b
	}
	if email.Valid {
		e := email.String
		seller.Email = &e
	}

	return seller, nil
}

// FindByOrganizationID retrieves all seller profiles for an organization.
func (r *SellerRepository) FindByOrganizationID(ctx context.Context, organizationID string) ([]domain.SellerProfile, error) {
	query := `SELECT id, user_id, organization_id, seller_type, display_name, bio, phone, email, status, verification_status, trust_level, created_at, updated_at FROM seller_profiles WHERE organization_id = $1`
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query seller profiles: %w", err)
	}
	defer rows.Close()

	var sellers []domain.SellerProfile
	for rows.Next() {
		var u sellerProfileRow
		var userID, orgID, bio, email sql.NullString
		if err := rows.Scan(&u.ID, &userID, &orgID, &u.SellerType, &u.DisplayName, &bio, &u.Phone, &email, &u.Status, &u.VerificationStatus, &u.TrustLevel, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan seller profile: %w", err)
		}

		seller := domain.SellerProfile{
			ID:                 u.ID,
			SellerType:         domain.SellerType(u.SellerType),
			DisplayName:        u.DisplayName,
			Phone:              u.Phone,
			Status:             domain.SellerStatus(u.Status),
			VerificationStatus: domain.VerificationStatus(u.VerificationStatus),
			TrustLevel:         u.TrustLevel,
			CreatedAt:          toUnix(u.CreatedAt),
			UpdatedAt:          toUnix(u.UpdatedAt),
		}

		if userID.Valid {
			uid := userID.String
			seller.UserID = &uid
		}
		if orgID.Valid {
			oid := orgID.String
			seller.OrganizationID = &oid
		}
		if bio.Valid {
			b := bio.String
			seller.Bio = &b
		}
		if email.Valid {
			e := email.String
			seller.Email = &e
		}

		sellers = append(sellers, seller)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating seller profiles: %w", err)
	}

	if sellers == nil {
		sellers = []domain.SellerProfile{}
	}

	return sellers, nil
}

// StoreRepository implements domain.StoreRepository using PostgreSQL.
type StoreRepository struct {
	db *sql.DB
}

// NewStoreRepository creates a new StoreRepository.
func NewStoreRepository(db *sql.DB) *StoreRepository {
	return &StoreRepository{db: db}
}

// Create inserts a new store into the database.
func (r *StoreRepository) Create(ctx context.Context, store *domain.Store) error {
	query := `INSERT INTO stores (id, organization_id, name, slug, description, phone, email, country, province, city, address, latitude, longitude, status, created_at, updated_at) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	var desc, email, province, city, address sql.NullString
	if store.Description != nil {
		desc = sql.NullString{String: *store.Description, Valid: true}
	}
	if store.Email != nil {
		email = sql.NullString{String: *store.Email, Valid: true}
	}
	if store.Province != nil {
		province = sql.NullString{String: *store.Province, Valid: true}
	}
	if store.City != nil {
		city = sql.NullString{String: *store.City, Valid: true}
	}
	if store.Address != nil {
		address = sql.NullString{String: *store.Address, Valid: true}
	}

	var lat, lng sql.NullFloat64
	if store.Latitude != nil {
		lat = sql.NullFloat64{Float64: *store.Latitude, Valid: true}
	}
	if store.Longitude != nil {
		lng = sql.NullFloat64{Float64: *store.Longitude, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, store.ID, store.OrganizationID, store.Name, store.Slug, desc, store.Phone, email, store.Country, province, city, address, lat, lng, store.Status, toTime(store.CreatedAt), toTime(store.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}
	return nil
}

// FindByID retrieves a store by ID.
func (r *StoreRepository) FindByID(ctx context.Context, id string) (*domain.Store, error) {
	query := `SELECT id, organization_id, name, slug, description, phone, email, country, province, city, address, latitude, longitude, status, created_at, updated_at FROM stores WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u storeRow
	var desc, email, province, city, address sql.NullString
	var lat, lng sql.NullFloat64
	err := row.Scan(&u.ID, &u.OrganizationID, &u.Name, &u.Slug, &desc, &u.Phone, &email, &u.Country, &u.Province, &u.City, &u.Address, &lat, &lng, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrStoreNotFound
		}
		return nil, fmt.Errorf("failed to find store: %w", err)
	}

	store := &domain.Store{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		Name:           u.Name,
		Slug:           u.Slug,
		Phone:          u.Phone,
		Country:        u.Country,
		Status:         domain.StoreStatus(u.Status),
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
	}

	if desc.Valid {
		d := desc.String
		store.Description = &d
	}
	if email.Valid {
		e := email.String
		store.Email = &e
	}
	if province.Valid {
		p := province.String
		store.Province = &p
	}
	if city.Valid {
		c := city.String
		store.City = &c
	}
	if address.Valid {
		a := address.String
		store.Address = &a
	}
	if lat.Valid {
		l := lat.Float64
		store.Latitude = &l
	}
	if lng.Valid {
		ln := lng.Float64
		store.Longitude = &ln
	}

	return store, nil
}

// Update updates an existing store.
func (r *StoreRepository) Update(ctx context.Context, store *domain.Store) error {
	query := `UPDATE stores SET name = $1, slug = $2, description = $3, phone = $4, email = $5, country = $6, province = $7, city = $8, address = $9, latitude = $10, longitude = $11, status = $12, updated_at = $13 WHERE id = $14`

	var desc, email, province, city, address sql.NullString
	if store.Description != nil {
		desc = sql.NullString{String: *store.Description, Valid: true}
	}
	if store.Email != nil {
		email = sql.NullString{String: *store.Email, Valid: true}
	}
	if store.Province != nil {
		province = sql.NullString{String: *store.Province, Valid: true}
	}
	if store.City != nil {
		city = sql.NullString{String: *store.City, Valid: true}
	}
	if store.Address != nil {
		address = sql.NullString{String: *store.Address, Valid: true}
	}

	var lat, lng sql.NullFloat64
	if store.Latitude != nil {
		lat = sql.NullFloat64{Float64: *store.Latitude, Valid: true}
	}
	if store.Longitude != nil {
		lng = sql.NullFloat64{Float64: *store.Longitude, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, store.Name, store.Slug, desc, store.Phone, email, store.Country, province, city, address, lat, lng, store.Status, toTime(store.UpdatedAt), store.ID)
	if err != nil {
		return fmt.Errorf("failed to update store: %w", err)
	}
	return nil
}

// FindByOrganizationID retrieves all stores for an organization.
func (r *StoreRepository) FindByOrganizationID(ctx context.Context, organizationID string) ([]domain.Store, error) {
	query := `SELECT id, organization_id, name, slug, description, phone, email, country, province, city, address, latitude, longitude, status, created_at, updated_at FROM stores WHERE organization_id = $1`
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query stores: %w", err)
	}
	defer rows.Close()

	var stores []domain.Store
	for rows.Next() {
		var u storeRow
		var desc, email, province, city, address sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&u.ID, &u.OrganizationID, &u.Name, &u.Slug, &desc, &u.Phone, &email, &u.Country, &u.Province, &u.City, &u.Address, &lat, &lng, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan store: %w", err)
		}

		store := domain.Store{
			ID:             u.ID,
			OrganizationID: u.OrganizationID,
			Name:           u.Name,
			Slug:           u.Slug,
			Phone:          u.Phone,
			Country:        u.Country,
			Status:         domain.StoreStatus(u.Status),
			CreatedAt:      toUnix(u.CreatedAt),
			UpdatedAt:      toUnix(u.UpdatedAt),
		}

		if desc.Valid {
			d := desc.String
			store.Description = &d
		}
		if email.Valid {
			e := email.String
			store.Email = &e
		}
		if province.Valid {
			p := province.String
			store.Province = &p
		}
		if city.Valid {
			c := city.String
			store.City = &c
		}
		if address.Valid {
			a := address.String
			store.Address = &a
		}
		if lat.Valid {
			l := lat.Float64
			store.Latitude = &l
		}
		if lng.Valid {
			ln := lng.Float64
			store.Longitude = &ln
		}

		stores = append(stores, store)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stores: %w", err)
	}

	if stores == nil {
		stores = []domain.Store{}
	}

	return stores, nil
}

// OrganizationRepository implements domain.OrganizationRepository using PostgreSQL.
type OrganizationRepository struct {
	db *sql.DB
}

// NewOrganizationRepository creates a new OrganizationRepository.
func NewOrganizationRepository(db *sql.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// Create inserts a new organization into the database.
func (r *OrganizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	query := `INSERT INTO organizations (id, type, legal_name, display_name, description, phone, email, status, country, province, city, address, verification_status, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	var desc, email, province, city, address sql.NullString
	if org.Description != nil {
		desc = sql.NullString{String: *org.Description, Valid: true}
	}
	if org.Email != nil {
		email = sql.NullString{String: *org.Email, Valid: true}
	}
	if org.Province != nil {
		province = sql.NullString{String: *org.Province, Valid: true}
	}
	if org.City != nil {
		city = sql.NullString{String: *org.City, Valid: true}
	}
	if org.Address != nil {
		address = sql.NullString{String: *org.Address, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, org.ID, org.Type, org.LegalName, org.DisplayName, desc, org.Phone, email, org.Status, org.Country, province, city, address, org.VerificationStatus, toTime(org.CreatedAt), toTime(org.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}
	return nil
}

// FindByID retrieves an organization by ID.
func (r *OrganizationRepository) FindByID(ctx context.Context, id string) (*domain.Organization, error) {
	query := `SELECT id, type, legal_name, display_name, description, phone, email, status, country, province, city, address, verification_status, created_at, updated_at FROM organizations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u organizationRow
	var desc, email, province, city, address sql.NullString
	err := row.Scan(&u.ID, &u.Type, &u.LegalName, &u.DisplayName, &desc, &u.Phone, &email, &u.Status, &u.Country, &u.Province, &u.City, &u.Address, &u.VerificationStatus, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrOrganizationNotFound
		}
		return nil, fmt.Errorf("failed to find organization: %w", err)
	}

	org := &domain.Organization{
		ID:                 u.ID,
		Type:               domain.OrganizationType(u.Type),
		LegalName:          u.LegalName,
		DisplayName:        u.DisplayName,
		Phone:              u.Phone,
		Status:             domain.OrganizationStatus(u.Status),
		Country:            u.Country,
		VerificationStatus: domain.VerificationStatus(u.VerificationStatus),
		CreatedAt:          toUnix(u.CreatedAt),
		UpdatedAt:          toUnix(u.UpdatedAt),
	}

	if desc.Valid {
		d := desc.String
		org.Description = &d
	}
	if email.Valid {
		e := email.String
		org.Email = &e
	}
	if province.Valid {
		p := province.String
		org.Province = &p
	}
	if city.Valid {
		c := city.String
		org.City = &c
	}
	if address.Valid {
		a := address.String
		org.Address = &a
	}

	return org, nil
}

// Update updates an existing organization.
func (r *OrganizationRepository) Update(ctx context.Context, org *domain.Organization) error {
	query := `UPDATE organizations SET type = $1, legal_name = $2, display_name = $3, description = $4, phone = $5, email = $6, status = $7, country = $8, province = $9, city = $10, address = $11, verification_status = $12, updated_at = $13 WHERE id = $14`

	var desc, email, province, city, address sql.NullString
	if org.Description != nil {
		desc = sql.NullString{String: *org.Description, Valid: true}
	}
	if org.Email != nil {
		email = sql.NullString{String: *org.Email, Valid: true}
	}
	if org.Province != nil {
		province = sql.NullString{String: *org.Province, Valid: true}
	}
	if org.City != nil {
		city = sql.NullString{String: *org.City, Valid: true}
	}
	if org.Address != nil {
		address = sql.NullString{String: *org.Address, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query, org.Type, org.LegalName, org.DisplayName, desc, org.Phone, email, org.Status, org.Country, province, city, address, org.VerificationStatus, toTime(org.UpdatedAt), org.ID)
	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}
	return nil
}

// AddMember inserts a new organization member.
func (r *OrganizationRepository) AddMember(ctx context.Context, member *domain.OrganizationMember) error {
	query := `INSERT INTO organization_members (organization_id, user_id, membership_role, status, joined_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, member.OrganizationID, member.UserID, member.Role, member.Status, toTime(member.JoinedAt))
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}
	return nil
}

// RemoveMember deletes an organization member.
func (r *OrganizationRepository) RemoveMember(ctx context.Context, organizationID, userID string) error {
	query := `DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, organizationID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	return nil
}

// GetMembers retrieves all members of an organization.
func (r *OrganizationRepository) GetMembers(ctx context.Context, organizationID string) ([]domain.OrganizationMember, error) {
	query := `SELECT organization_id, user_id, membership_role, status, joined_at FROM organization_members WHERE organization_id = $1`
	rows, err := r.db.QueryContext(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query members: %w", err)
	}
	defer rows.Close()

	var members []domain.OrganizationMember
	for rows.Next() {
		var m domain.OrganizationMember
		var joinedAt time.Time
		if err := rows.Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.Status, &joinedAt); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		m.JoinedAt = toUnix(joinedAt)
		members = append(members, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating members: %w", err)
	}

	if members == nil {
		members = []domain.OrganizationMember{}
	}

	return members, nil
}

// HasMember checks if a user is a member of an organization.
func (r *OrganizationRepository) HasMember(ctx context.Context, organizationID, userID string) (bool, error) {
	query := `SELECT COUNT(*) FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	var count int
	err := r.db.QueryRowContext(ctx, query, organizationID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check membership: %w", err)
	}
	return count > 0, nil
}

// GetMemberRole retrieves the role of a user in an organization.
func (r *OrganizationRepository) GetMemberRole(ctx context.Context, organizationID, userID string) (domain.OrganizationRole, error) {
	query := `SELECT membership_role FROM organization_members WHERE organization_id = $1 AND user_id = $2`
	var role domain.OrganizationRole
	err := r.db.QueryRowContext(ctx, query, organizationID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrInsufficientPermissions
		}
		return "", fmt.Errorf("failed to get member role: %w", err)
	}
	return role, nil
}
