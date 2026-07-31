-- +migrate Up
ALTER TABLE roles
    ADD CONSTRAINT roles_tenant_id_id_key UNIQUE (tenant_id, id);

CREATE TABLE lens_saved_views (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    dashboard_id VARCHAR(120) NOT NULL,
    name VARCHAR(120) NOT NULL,
    scope VARCHAR(16) NOT NULL CHECK (scope IN ('personal', 'team')),
    owner_user_id INT8 NOT NULL,
    state_url TEXT NOT NULL,
    default_role_id INT8,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lens_saved_views_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT lens_saved_views_tenant_dashboard_id_key UNIQUE (tenant_id, dashboard_id, id),
    CONSTRAINT lens_saved_views_owner_fkey FOREIGN KEY (tenant_id, owner_user_id)
        REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT lens_saved_views_state_url_check CHECK (
        char_length(state_url) BETWEEN 1 AND 8192
        AND left(state_url, 1) = '/'
        AND left(state_url, 2) <> '//'
        AND position(E'\\' IN state_url) = 0
    ),
    CONSTRAINT lens_saved_views_role_default_scope_check CHECK (default_role_id IS NULL OR scope = 'team'),
    CONSTRAINT lens_saved_views_default_role_fkey FOREIGN KEY (tenant_id, default_role_id)
        REFERENCES roles (tenant_id, id) ON DELETE SET NULL (default_role_id)
);

CREATE INDEX lens_saved_views_visible_idx
    ON lens_saved_views (tenant_id, dashboard_id, scope, owner_user_id);
CREATE INDEX lens_saved_views_owner_idx
    ON lens_saved_views (tenant_id, owner_user_id);
CREATE INDEX lens_saved_views_default_role_idx
    ON lens_saved_views (tenant_id, default_role_id)
    WHERE default_role_id IS NOT NULL;
CREATE UNIQUE INDEX lens_saved_views_role_default_idx
    ON lens_saved_views (tenant_id, dashboard_id, default_role_id)
    WHERE default_role_id IS NOT NULL;

CREATE TABLE lens_export_schedules (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    owner_user_id INT8 NOT NULL,
    dashboard_id VARCHAR(120) NOT NULL,
    view_id UUID NOT NULL,
    name VARCHAR(120) NOT NULL,
    cron_expr VARCHAR(120) NOT NULL,
    timezone VARCHAR(80) NOT NULL,
    recipients TEXT[] NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lens_export_schedules_recipients_check CHECK (cardinality(recipients) BETWEEN 1 AND 20),
    CONSTRAINT lens_export_schedules_owner_fkey FOREIGN KEY (tenant_id, owner_user_id)
        REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT lens_export_schedules_view_fkey FOREIGN KEY (tenant_id, dashboard_id, view_id)
        REFERENCES lens_saved_views (tenant_id, dashboard_id, id) ON DELETE CASCADE
);

CREATE INDEX lens_export_schedules_due_idx
    ON lens_export_schedules (next_run_at)
    WHERE enabled = TRUE;
CREATE INDEX lens_export_schedules_owner_idx
    ON lens_export_schedules (tenant_id, owner_user_id, dashboard_id);
CREATE INDEX lens_export_schedules_view_idx
    ON lens_export_schedules (tenant_id, dashboard_id, view_id);

-- +migrate Down
DROP TABLE IF EXISTS lens_export_schedules;
DROP TABLE IF EXISTS lens_saved_views;
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_id_id_key;
