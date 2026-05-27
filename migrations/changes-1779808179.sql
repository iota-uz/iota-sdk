-- Migration: Add core organizational model (departments + user positions)
-- Date: 2026-05-26
-- Purpose: Introduce reusable, tenant-scoped department hierarchy and user
--   position aggregates in the core module. Foundation for EAI's EDO granular
--   permissions epic. Names/titles are multilingual jsonb (MultiLang).
--
-- Cross-tenant integrity is enforced at the database level: foreign keys are
-- tenant-qualified (they reference the composite (tenant_id, id) of the parent
-- table) so a row can never link to another tenant's row, regardless of the
-- application path that wrote it.

-- +migrate Up
CREATE SCHEMA IF NOT EXISTS core;

-- Tenant-qualified FK target: a position references both the user's id and
-- tenant, so a position can never point at a user in another tenant.
ALTER TABLE users
    ADD CONSTRAINT users_tenant_id_id_key UNIQUE (tenant_id, id);

CREATE TABLE core.departments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    parent_id uuid NULL,
    code varchar NOT NULL,
    name jsonb NOT NULL,
    "order" int NOT NULL DEFAULT 0,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, code),
    -- Tenant-qualified self-reference target for the parent FK below.
    CONSTRAINT departments_tenant_id_id_key UNIQUE (tenant_id, id),
    -- A parent department must live in the same tenant; cross-tenant parents
    -- are rejected by the database, not only the application.
    CONSTRAINT departments_parent_tenant_fkey FOREIGN KEY (tenant_id, parent_id)
        REFERENCES core.departments (tenant_id, id) ON DELETE SET NULL
);

CREATE INDEX departments_tenant_id_idx ON core.departments (tenant_id);

CREATE INDEX departments_tenant_id_parent_id_idx ON core.departments (tenant_id, parent_id);

CREATE TABLE core.user_positions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    department_id uuid NOT NULL,
    title jsonb NOT NULL,
    is_manager boolean NOT NULL DEFAULT FALSE,
    is_primary boolean NOT NULL DEFAULT FALSE,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    -- Both references are tenant-qualified so a position cannot bridge tenants.
    CONSTRAINT user_positions_department_tenant_fkey FOREIGN KEY (tenant_id, department_id)
        REFERENCES core.departments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT user_positions_user_tenant_fkey FOREIGN KEY (tenant_id, user_id)
        REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX user_positions_tenant_id_user_id_idx ON core.user_positions (tenant_id, user_id);

CREATE INDEX user_positions_tenant_id_department_id_idx ON core.user_positions (tenant_id, department_id);

-- +migrate Down
DROP TABLE IF EXISTS core.user_positions;

DROP TABLE IF EXISTS core.departments;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_tenant_id_id_key;
