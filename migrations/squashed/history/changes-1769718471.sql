-- +migrate Up
-- Add two-factor authentication fields to users table
ALTER TABLE users
ADD COLUMN IF NOT EXISTS two_factor_method VARCHAR(20) DEFAULT NULL,
ADD COLUMN IF NOT EXISTS totp_secret_encrypted VARCHAR(512) DEFAULT NULL,
ADD COLUMN IF NOT EXISTS two_factor_enabled_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;

-- Add comment to explain two-factor method values
COMMENT ON COLUMN users.two_factor_method IS 'Two-factor authentication method: totp, sms, or email';
COMMENT ON COLUMN users.totp_secret_encrypted IS 'Encrypted TOTP secret for authenticator apps';
COMMENT ON COLUMN users.two_factor_enabled_at IS 'Timestamp when two-factor authentication was enabled';

-- Create recovery_codes table for backup codes
CREATE TABLE IF NOT EXISTS recovery_codes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash VARCHAR(255) NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE
);

-- Add comments to recovery_codes columns
COMMENT ON COLUMN recovery_codes.code_hash IS 'bcrypt hash of the recovery code';
COMMENT ON COLUMN recovery_codes.used_at IS 'Timestamp when code was used';

-- Create indexes on recovery_codes for efficient querying
CREATE INDEX IF NOT EXISTS idx_recovery_codes_user_tenant ON recovery_codes(user_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_recovery_codes_unused ON recovery_codes(user_id, tenant_id) WHERE used_at IS NULL;

-- Create two_factor_otps table for one-time passwords
CREATE TABLE IF NOT EXISTS two_factor_otps (
    id BIGSERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL,
    code_hash VARCHAR(255) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE DEFAULT NULL,
    attempts INT DEFAULT 0 NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- Add comments to two_factor_otps columns
COMMENT ON COLUMN two_factor_otps.identifier IS 'Email or phone number for OTP delivery';
COMMENT ON COLUMN two_factor_otps.code_hash IS 'bcrypt hash of the OTP code';
COMMENT ON COLUMN two_factor_otps.channel IS 'Channel for delivery: sms or email';
COMMENT ON COLUMN two_factor_otps.expires_at IS 'When OTP expires';
COMMENT ON COLUMN two_factor_otps.used_at IS 'When OTP was used';
COMMENT ON COLUMN two_factor_otps.attempts IS 'Number of failed verification attempts';

-- Create indexes on two_factor_otps for efficient querying
CREATE INDEX IF NOT EXISTS idx_two_factor_otps_identifier_tenant ON two_factor_otps(identifier, tenant_id);
CREATE INDEX IF NOT EXISTS idx_two_factor_otps_active ON two_factor_otps(expires_at, tenant_id) WHERE used_at IS NULL;

-- Add session status field to track session state
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'active' NOT NULL;

-- Add comment to sessions.status column
COMMENT ON COLUMN sessions.status IS 'Session status: active, expired, terminated, or revoked';

-- Create index on session status for efficient filtering
CREATE INDEX IF NOT EXISTS idx_sessions_status_tenant ON sessions(status, tenant_id);

-- +migrate Down
-- Remove session status index and column
DROP INDEX IF EXISTS idx_sessions_status_tenant;
ALTER TABLE sessions DROP COLUMN IF EXISTS status;

-- Drop OTPs table and related indexes
DROP INDEX IF EXISTS idx_two_factor_otps_active;
DROP INDEX IF EXISTS idx_two_factor_otps_identifier_tenant;
DROP TABLE IF EXISTS two_factor_otps;

-- Drop recovery codes table and related indexes
DROP INDEX IF EXISTS idx_recovery_codes_unused;
DROP INDEX IF EXISTS idx_recovery_codes_user_tenant;
DROP TABLE IF EXISTS recovery_codes;

-- Remove 2FA columns from users table
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_enabled_at;
ALTER TABLE users DROP COLUMN IF EXISTS totp_secret_encrypted;
ALTER TABLE users DROP COLUMN IF EXISTS two_factor_method;
