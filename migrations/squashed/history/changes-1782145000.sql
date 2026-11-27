-- +migrate Up
-- Extend users.ui_language from VARCHAR(3) to VARCHAR(10).
--
-- VARCHAR(3) was sufficient for the original 2-letter locale codes (en, ru,
-- uz, zh) but does not fit region-tagged locales that the project already
-- supports elsewhere, such as uz-Cyrl, zh-cn, and pt-BR. Without this change
-- any new regional locale truncates on insert and the session middleware
-- cannot match the user's saved preference.
--
-- The column participates in the analytics.users row-level-security view,
-- which must be dropped before the ALTER and recreated with the same shape
-- afterwards (postgres refuses to alter a column referenced by a view).

DROP VIEW IF EXISTS analytics.users;
ALTER TABLE users
    ALTER COLUMN ui_language TYPE VARCHAR(10) USING ui_language::VARCHAR(10);
CREATE VIEW analytics.users AS
    SELECT id,
           first_name,
           last_name,
           middle_name,
           ui_language,
           email,
           password,
           avatar_id,
           last_login,
           last_ip,
           last_action,
           created_at,
           updated_at,
           phone,
           tenant_id,
           type,
           is_blocked,
           block_reason,
           blocked_at,
           blocked_by,
           blocked_by_tenant_id,
           two_factor_method,
           totp_secret_encrypted,
           two_factor_enabled_at
      FROM users
     WHERE tenant_id = current_setting('app.tenant_id'::text, true)::uuid;

-- +migrate Down
-- Best-effort rollback. Drops the view, shrinks the column, recreates the
-- view. Existing rows whose ui_language is longer than 3 characters will be
-- truncated, which is why this Down is destructive: any user with a
-- regional locale (e.g. pt-BR) loses it.

DROP VIEW IF EXISTS analytics.users;
ALTER TABLE users
    ALTER COLUMN ui_language TYPE VARCHAR(3) USING SUBSTRING(ui_language FROM 1 FOR 3);
CREATE VIEW analytics.users AS
    SELECT id,
           first_name,
           last_name,
           middle_name,
           ui_language,
           email,
           password,
           avatar_id,
           last_login,
           last_ip,
           last_action,
           created_at,
           updated_at,
           phone,
           tenant_id,
           type,
           is_blocked,
           block_reason,
           blocked_at,
           blocked_by,
           blocked_by_tenant_id,
           two_factor_method,
           totp_secret_encrypted,
           two_factor_enabled_at
      FROM users
     WHERE tenant_id = current_setting('app.tenant_id'::text, true)::uuid;
