-- Migration: Add core organizational model (departments + user positions)
-- Date: 2026-05-26
-- Purpose: Introduce reusable, tenant-scoped department hierarchy and user
--   position aggregates in the core module. Foundation for EAI's EDO granular
--   permissions epic. Names/titles are multilingual jsonb (MultiLang).

-- +migrate Up
CREATE SCHEMA IF NOT EXISTS core;

CREATE TABLE core.departments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    parent_id uuid NULL REFERENCES core.departments (id) ON DELETE SET NULL,
    code varchar NOT NULL,
    name jsonb NOT NULL,
    "order" int NOT NULL DEFAULT 0,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, code)
);

CREATE INDEX departments_tenant_id_idx ON core.departments (tenant_id);

CREATE INDEX departments_tenant_id_parent_id_idx ON core.departments (tenant_id, parent_id);

CREATE TABLE core.user_positions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    department_id uuid NOT NULL REFERENCES core.departments (id) ON DELETE CASCADE,
    title jsonb NOT NULL,
    is_manager boolean NOT NULL DEFAULT FALSE,
    is_primary boolean NOT NULL DEFAULT FALSE,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE INDEX user_positions_tenant_id_user_id_idx ON core.user_positions (tenant_id, user_id);

CREATE INDEX user_positions_tenant_id_department_id_idx ON core.user_positions (tenant_id, department_id);

-- +migrate Down
DROP TABLE IF EXISTS core.user_positions;

DROP TABLE IF EXISTS core.departments;
