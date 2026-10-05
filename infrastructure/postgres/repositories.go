package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// userRow represents a row from the users table.
type userRow struct {
	ID          string
	Phone       sql.NullString // NULL for staff accounts, which sign in with an email
	Email       sql.NullString
	DisplayName string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	VerifiedAt  sql.NullTime
}

// UserRepository implements domain.UserRepository using PostgreSQL.
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// FindByID retrieves a user by their ID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `SELECT id, phone, email, display_name, status, created_at, updated_at, verified_at FROM users WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u userRow
	var emailVal sql.NullString
	err := row.Scan(&u.ID, &u.Phone, &emailVal, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.VerifiedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w: %s", domain.ErrUserNotFound, id)
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	email := (*string)(nil)
	if emailVal.Valid {
		email = &emailVal.String
	}

	user := &domain.User{
		ID:          u.ID,
		Phone:       u.Phone.String,
		Email:       email,
		DisplayName: u.DisplayName,
		Status:      domain.UserStatus(u.Status),
		CreatedAt:   toUnix(u.CreatedAt),
		UpdatedAt:   toUnix(u.UpdatedAt),
		Roles:       []domain.Role{},
	}

	if u.VerifiedAt.Valid {
		ts := toUnix(u.VerifiedAt.Time)
		user.VerifiedAt = &ts
	}

	// Load user roles from user_roles table
	roles, err := r.GetUserRoles(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user roles: %w", err)
	}
	user.Roles = roles

	return user, nil
}

// FindByPhone retrieves a user by their phone number.
func (r *UserRepository) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := `SELECT id, phone, email, display_name, status, created_at, updated_at, verified_at FROM users WHERE phone = $1`
	row := r.db.QueryRowContext(ctx, query, phone)

	var u userRow
	var emailVal sql.NullString
	err := row.Scan(&u.ID, &u.Phone, &emailVal, &u.DisplayName, &u.Status, &u.CreatedAt, &u.UpdatedAt, &u.VerifiedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by phone: %w", err)
	}

	email := (*string)(nil)
	if emailVal.Valid {
		email = &emailVal.String
	}

	user := &domain.User{
		ID:          u.ID,
		Phone:       u.Phone.String,
		Email:       email,
		DisplayName: u.DisplayName,
		Status:      domain.UserStatus(u.Status),
		CreatedAt:   toUnix(u.CreatedAt),
		UpdatedAt:   toUnix(u.UpdatedAt),
		Roles:       []domain.Role{},
	}

	if u.VerifiedAt.Valid {
		ts := toUnix(u.VerifiedAt.Time)
		user.VerifiedAt = &ts
	}

	// Load user roles
	roles, err := r.GetUserRoles(ctx, u.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load user roles: %w", err)
	}
	user.Roles = roles

	return user, nil
}

// Create inserts a new user into the database.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, phone, email, display_name, status, created_at, updated_at, verified_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	var email sql.NullString
	if user.Email != nil {
		email = sql.NullString{String: *user.Email, Valid: true}
	}

	var verifiedAt sql.NullTime
	if user.VerifiedAt != nil {
		verifiedAt = sql.NullTime{Valid: true, Time: toTime(*user.VerifiedAt)}
	}

	phone := sql.NullString{String: user.Phone, Valid: user.Phone != ""}
	_, err := r.db.ExecContext(ctx, query, user.ID, phone, email, user.DisplayName, user.Status, toTime(user.CreatedAt), toTime(user.UpdatedAt), verifiedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// Update updates an existing user in the database.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET phone = $1, email = $2, display_name = $3, status = $4, updated_at = $5, verified_at = $6 WHERE id = $7`

	var email sql.NullString
	if user.Email != nil {
		email = sql.NullString{String: *user.Email, Valid: true}
	}

	var verifiedAt sql.NullTime
	if user.VerifiedAt != nil {
		verifiedAt = sql.NullTime{Valid: true, Time: toTime(*user.VerifiedAt)}
	}

	phone := sql.NullString{String: user.Phone, Valid: user.Phone != ""}
	_, err := r.db.ExecContext(ctx, query, phone, email, user.DisplayName, user.Status, toTime(user.UpdatedAt), verifiedAt, user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// AssignRole assigns a role to a user using the user_roles junction table.
// This is the CORRECT implementation - uses actual RBAC, NOT user.Status hack.
func (r *UserRepository) AssignRole(ctx context.Context, userID string, role domain.Role) error {
	// Check if already assigned
	exists, err := r.HasRole(ctx, userID, role)
	if err != nil {
		return fmt.Errorf("failed to check existing role: %w", err)
	}
	if exists {
		return domain.ErrRoleAlreadyAssigned
	}

	query := `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`
	result, err := r.db.ExecContext(ctx, query, userID, string(role))
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("role not found: %s", string(role))
	}

	return nil
}

// RemoveRole removes a role from a user using the user_roles junction table.
// This is the CORRECT implementation - uses actual RBAC, NOT user.Status hack.
func (r *UserRepository) RemoveRole(ctx context.Context, userID string, role domain.Role) error {
	query := `DELETE FROM user_roles WHERE user_id = $1 AND role_id = (SELECT id FROM roles WHERE name = $2)`
	result, err := r.db.ExecContext(ctx, query, userID, string(role))
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrRoleNotAssigned
	}

	return nil
}

// HasRole checks if a user has a specific role.
func (r *UserRepository) HasRole(ctx context.Context, userID string, role domain.Role) (bool, error) {
	query := `SELECT COUNT(*) FROM user_roles WHERE user_id = $1 AND role_id = (SELECT id FROM roles WHERE name = $2)`
	var count int
	err := r.db.QueryRowContext(ctx, query, userID, string(role)).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check role: %w", err)
	}
	return count > 0, nil
}

// GetUserRoles retrieves all roles for a user.
func (r *UserRepository) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	query := `SELECT r.name FROM roles r INNER JOIN user_roles ur ON r.id = ur.role_id WHERE ur.user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var roleName string
		if err := rows.Scan(&roleName); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, domain.Role(roleName))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating roles: %w", err)
	}

	if roles == nil {
		roles = []domain.Role{}
	}

	return roles, nil
}

// roleRow represents a row from the roles table.
type roleRow struct {
	ID        string
	Name      string
	CreatedAt int64
	UpdatedAt int64
}

// RoleRepository implements domain.RoleRepository using PostgreSQL.
type RoleRepository struct {
	db *sql.DB
}

// NewRoleRepository creates a new RoleRepository.
func NewRoleRepository(db *sql.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// Exists checks if a role exists in the database.
func (r *RoleRepository) Exists(ctx context.Context, name string) (bool, error) {
	query := `SELECT COUNT(*) FROM roles WHERE name = $1`
	var count int
	err := r.db.QueryRowContext(ctx, query, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check role existence: %w", err)
	}
	return count > 0, nil
}

// GetAll retrieves all roles from the database.
func (r *RoleRepository) GetAll(ctx context.Context) ([]domain.Role, error) {
	query := `SELECT name FROM roles`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var roleName string
		if err := rows.Scan(&roleName); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		roles = append(roles, domain.Role(roleName))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating roles: %w", err)
	}

	if roles == nil {
		roles = []domain.Role{}
	}

	return roles, nil
}

// GetPermissions retrieves all permissions for a role.
func (r *RoleRepository) GetPermissions(ctx context.Context, role domain.Role) ([]domain.Permission, error) {
	query := `SELECT p.name FROM permissions p INNER JOIN role_permissions rp ON p.id = rp.permission_id WHERE rp.role_id = (SELECT id FROM roles WHERE name = $1)`
	rows, err := r.db.QueryContext(ctx, query, string(role))
	if err != nil {
		return nil, fmt.Errorf("failed to query role permissions: %w", err)
	}
	defer rows.Close()

	var perms []domain.Permission
	for rows.Next() {
		var permName string
		if err := rows.Scan(&permName); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perms = append(perms, domain.Permission(permName))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating permissions: %w", err)
	}

	if perms == nil {
		perms = []domain.Permission{}
	}

	return perms, nil
}
