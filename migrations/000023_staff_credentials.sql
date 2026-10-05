-- 000023_staff_credentials.sql
-- Email-and-password sign-in for platform staff (admin, operator, moderator).
--
-- Customers and sellers keep signing in with a phone number and a one-time
-- code. Staff accounts are different: they are created by an operator
-- (trezutil-admin create-admin), have an email address instead of a phone, and
-- sign in with a password.
--
-- Contract:
--   * Only a salted PBKDF2 hash is stored, never the password.
--   * A phone number becomes optional so a staff account can exist without one;
--     the existing unique index on phone still holds for the accounts that have
--     one (NULLs do not collide).
--   * An email address is unique ignoring case.
--   * Repeated wrong passwords lock the account for a while (locked_until).

-- +goose Up
ALTER TABLE users ALTER COLUMN phone DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP WITH TIME ZONE;

CREATE UNIQUE INDEX IF NOT EXISTS uq_users_email_lower ON users (lower(email)) WHERE email IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uq_users_email_lower;
ALTER TABLE users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE users DROP COLUMN IF EXISTS locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_attempts;
ALTER TABLE users DROP COLUMN IF EXISTS password_changed_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
-- phone stays nullable on rollback: staff accounts created meanwhile have none.
