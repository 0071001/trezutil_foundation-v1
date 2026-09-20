package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// RequestOTPUseCase initiates OTP generation and delivery for the given phone number.
type RequestOTPUseCase struct {
	otpStore    domain.OTPStore
	otpProvider domain.OTPProvider
	userRepo    domain.UserRepository
	rateLimiter RateLimiter
}

// RateLimiter defines the interface for rate limiting.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// NewRequestOTPUseCase creates a new RequestOTP use case.
func NewRequestOTPUseCase(otpStore domain.OTPStore, otpProvider domain.OTPProvider, userRepo domain.UserRepository, rateLimiter RateLimiter) *RequestOTPUseCase {
	return &RequestOTPUseCase{
		otpStore:    otpStore,
		otpProvider: otpProvider,
		userRepo:    userRepo,
		rateLimiter: rateLimiter,
	}
}

// Execute sends the OTP to the user's phone.
func (u *RequestOTPUseCase) Execute(ctx context.Context, phone string) error {
	// Rate limiting: check if we can send another OTP
	allowed, err := u.rateLimiter.Allow(ctx, "otp_request:"+phone)
	if err != nil {
		return fmt.Errorf("rate limit check failed: %w", err)
	}
	if !allowed {
		return fmt.Errorf("too many OTP requests, please try again later")
	}

	// Normalize phone number
	normalizedPhone, err := domain.NormalizePhone(phone)
	if err != nil {
		return fmt.Errorf("invalid phone number: %w", err)
	}

	// Generate a 6-digit OTP code
	code := fmt.Sprintf("%06d", time.Now().Unix()%1000000)

	// Store OTP in Redis with 5-minute expiration (300 seconds)
	// Key includes phone to prevent OTP reuse across different phones
	err = u.otpStore.Set(ctx, normalizedPhone, code, 300)
	if err != nil {
		return fmt.Errorf("failed to store OTP: %w", err)
	}

	// Send OTP via the OTP provider
	err = u.otpProvider.Send(ctx, normalizedPhone, code)
	if err != nil {
		return fmt.Errorf("failed to send OTP: %w", err)
	}

	return nil
}

// VerifyOTPUseCase verifies an OTP code for the given phone number.
type VerifyOTPUseCase struct {
	otpStore    domain.OTPStore
	userRepo    domain.UserRepository
	auditLogger domain.AuditLogger
}

// NewVerifyOTPUseCase creates a new VerifyOTP use case.
func NewVerifyOTPUseCase(otpStore domain.OTPStore, userRepo domain.UserRepository, auditLogger domain.AuditLogger) *VerifyOTPUseCase {
	return &VerifyOTPUseCase{
		otpStore:    otpStore,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

// Execute verifies the OTP code and returns the authenticated user if valid.
func (u *VerifyOTPUseCase) Execute(ctx context.Context, phone, code string) (*domain.User, error) {
	// Normalize phone number
	normalizedPhone, err := domain.NormalizePhone(phone)
	if err != nil {
		return nil, fmt.Errorf("invalid phone number: %w", err)
	}

	// Retrieve stored OTP from Redis
	storedCode, err := u.otpStore.Get(ctx, normalizedPhone)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve OTP: %w", err)
	}

	// Check if OTP matches
	if storedCode == nil || *storedCode != code {
		// Log failed attempt for audit
		if u.auditLogger != nil {
			u.auditLogger.Record(ctx, domain.AuditEntry{
				Action:     "otp.verification_failed",
				TargetType: "user",
				TargetID:   normalizedPhone,
				Metadata: map[string]interface{}{
					"reason": "invalid_or_expired_otp",
				},
			})
		}
		return nil, domain.ErrInvalidOTP
	}

	// OTP is valid - invalidate it immediately to prevent reuse
	err = u.otpStore.Delete(ctx, normalizedPhone)
	if err != nil {
		// Log but don't fail - the OTP was already verified
		fmt.Printf("Warning: failed to delete OTP after verification: %v\n", err)
	}

	// OTP is valid - get or create the user
	user, err := u.userRepo.FindByPhone(ctx, normalizedPhone)
	if err != nil {
		// User doesn't exist yet; create a new one
		user = domain.NewUser(normalizedPhone, normalizedPhone)
		err = u.userRepo.Create(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Mark user as verified if this is their first OTP verification
	if !user.IsVerified() {
		now := time.Now().Unix()
		user.VerifiedAt = &now
		user.Status = domain.UserStatusActive
		err = u.userRepo.Update(ctx, user)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Log successful verification for audit
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    user.ID,
			Action:     "otp.verified",
			TargetType: "user",
			TargetID:   user.ID,
		})
	}

	return user, nil
}

// GetCurrentUserUseCase retrieves the current user profile.
type GetCurrentUserUseCase struct {
	userRepo domain.UserRepository
}

// NewGetCurrentUserUseCase creates a new GetCurrentUser use case.
func NewGetCurrentUserUseCase(userRepo domain.UserRepository) *GetCurrentUserUseCase {
	return &GetCurrentUserUseCase{
		userRepo: userRepo,
	}
}

// Execute returns the current user's profile.
func (u *GetCurrentUserUseCase) Execute(ctx context.Context, userID string) (*domain.User, error) {
	return u.userRepo.FindByID(ctx, userID)
}

// UpdateCurrentUserUseCase updates the current user's profile.
type UpdateCurrentUserUseCase struct {
	userRepo domain.UserRepository
}

// NewUpdateCurrentUserUseCase creates a new UpdateCurrentUser use case.
func NewUpdateCurrentUserUseCase(userRepo domain.UserRepository) *UpdateCurrentUserUseCase {
	return &UpdateCurrentUserUseCase{
		userRepo: userRepo,
	}
}

// Execute updates the current user's profile with the given data.
func (u *UpdateCurrentUserUseCase) Execute(ctx context.Context, user *domain.User) error {
	now := time.Now().Unix()
	user.UpdatedAt = now
	return u.userRepo.Update(ctx, user)
}

// AssignRoleUseCase assigns a role to a user.
type AssignRoleUseCase struct {
	userRepo    domain.UserRepository
	roleRepo    domain.RoleRepository
	auditLogger domain.AuditLogger
}

// NewAssignRoleUseCase creates a new AssignRole use case.
func NewAssignRoleUseCase(userRepo domain.UserRepository, roleRepo domain.RoleRepository, auditLogger domain.AuditLogger) *AssignRoleUseCase {
	return &AssignRoleUseCase{
		userRepo:    userRepo,
		roleRepo:    roleRepo,
		auditLogger: auditLogger,
	}
}

// Execute assigns a role to a user.
func (u *AssignRoleUseCase) Execute(ctx context.Context, actorID, userID, roleName string) error {
	// Validate role name
	role := domain.Role(roleName)
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", roleName)
	}

	// Check role exists
	exists, err := u.roleRepo.Exists(ctx, roleName)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("role does not exist: %s", roleName)
	}

	// Get target user
	targetUser, err := u.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find target user: %w", err)
	}

	// Get actor user for permission check
	actorUser, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor user: %w", err)
	}

	// Check permission using ACTUAL roles, NOT status hacks
	// Only users with admin.manage permission can assign roles
	if !actorUser.HasRole(domain.RoleAdmin) && !actorUser.HasRole(domain.RoleOperator) {
		return domain.ErrInsufficientPermissions
	}

	// Assign the role
	err = u.userRepo.AssignRole(ctx, userID, role)
	if err != nil {
		return err
	}

	// Reload user roles
	roles, err := u.userRepo.GetUserRoles(ctx, userID)
	if err == nil {
		targetUser.Roles = roles
	}

	// Log for audit
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "role.assigned",
			TargetType: "user",
			TargetID:   userID,
			Metadata: map[string]interface{}{
				"role": roleName,
			},
		})
	}

	return nil
}

// RemoveRoleUseCase removes a role from a user.
type RemoveRoleUseCase struct {
	userRepo    domain.UserRepository
	roleRepo    domain.RoleRepository
	auditLogger domain.AuditLogger
}

// NewRemoveRoleUseCase creates a new RemoveRole use case.
func NewRemoveRoleUseCase(userRepo domain.UserRepository, roleRepo domain.RoleRepository, auditLogger domain.AuditLogger) *RemoveRoleUseCase {
	return &RemoveRoleUseCase{
		userRepo:    userRepo,
		roleRepo:    roleRepo,
		auditLogger: auditLogger,
	}
}

// Execute removes a role from a user.
func (u *RemoveRoleUseCase) Execute(ctx context.Context, actorID, userID, roleName string) error {
	// Validate role name
	role := domain.Role(roleName)
	if !role.IsValid() {
		return fmt.Errorf("invalid role: %s", roleName)
	}

	// Check role exists
	exists, err := u.roleRepo.Exists(ctx, roleName)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("role does not exist: %s", roleName)
	}

	// Get actor user for permission check
	actorUser, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor user: %w", err)
	}

	// Check permission using ACTUAL roles, NOT status hacks
	if !actorUser.HasRole(domain.RoleAdmin) && !actorUser.HasRole(domain.RoleOperator) {
		return domain.ErrInsufficientPermissions
	}

	// Remove the role
	err = u.userRepo.RemoveRole(ctx, userID, role)
	if err != nil {
		return err
	}

	// Reload user roles
	targetUser, _ := u.userRepo.FindByID(ctx, userID)
	if targetUser != nil {
		roles, err := u.userRepo.GetUserRoles(ctx, userID)
		if err == nil {
			targetUser.Roles = roles
		}
	}

	// Log for audit
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "role.revoked",
			TargetType: "user",
			TargetID:   userID,
			Metadata: map[string]interface{}{
				"role": roleName,
			},
		})
	}

	return nil
}

// CheckPermissionUseCase checks if a user has a specific permission.
type CheckPermissionUseCase struct {
	roleRepo domain.RoleRepository
}

// NewCheckPermissionUseCase creates a new CheckPermission use case.
func NewCheckPermissionUseCase(roleRepo domain.RoleRepository) *CheckPermissionUseCase {
	return &CheckPermissionUseCase{
		roleRepo: roleRepo,
	}
}

// Execute checks if the user has the specified permission by looking up their roles.
func (u *CheckPermissionUseCase) Execute(ctx context.Context, user *domain.User, perm domain.Permission) bool {
	if user == nil {
		return false
	}

	// Check each of the user's actual roles
	for _, role := range user.Roles {
		permissions, err := u.roleRepo.GetPermissions(ctx, role)
		if err != nil {
			continue
		}
		for _, p := range permissions {
			if p == perm {
				return true
			}
		}
	}

	return false
}
