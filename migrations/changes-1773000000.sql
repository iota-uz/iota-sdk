-- +migrate Up
CREATE SCHEMA IF NOT EXISTS spotlight;
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pg_textsearch;

CREATE TABLE IF NOT EXISTS spotlight.documents (
    id TEXT NOT NULL,
    tenant_id UUID NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    url TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'en',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    access_policy JSONB NOT NULL DEFAULT '{"visibility":"restricted","owner_id":"","allowed_users":[],"allowed_roles":[],"allowed_permissions":[]}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    embedding VECTOR(1536),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS idx_spotlight_documents_scope
    ON spotlight.documents (tenant_id, provider, entity_type, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_spotlight_documents_metadata
    ON spotlight.documents USING GIN (metadata);

CREATE INDEX IF NOT EXISTS idx_spotlight_documents_bm25
    ON spotlight.documents
    USING bm25 (id, title, body)
    WITH (key_field = 'id');

CREATE INDEX IF NOT EXISTS idx_spotlight_documents_embedding
    ON spotlight.documents
    USING hnsw (embedding vector_cosine_ops);

CREATE TABLE IF NOT EXISTS spotlight.document_acl (
    tenant_id UUID NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    document_id TEXT NOT NULL,
    principal_type TEXT NOT NULL,
    principal_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, document_id, principal_type, principal_id),
    FOREIGN KEY (tenant_id, document_id)
        REFERENCES spotlight.documents (tenant_id, id)
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_spotlight_document_acl_lookup
    ON spotlight.document_acl (tenant_id, principal_type, principal_id);

CREATE TABLE IF NOT EXISTS spotlight.provider_state (
    tenant_id UUID NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    watermark TEXT NOT NULL DEFAULT '',
    last_indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, provider)
);

CREATE TABLE IF NOT EXISTS spotlight.outbox (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    event_type TEXT NOT NULL,
    document_id TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_spotlight_outbox_pending
    ON spotlight.outbox (tenant_id, created_at)
    WHERE processed_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_spotlight_outbox_tenant_created
    ON spotlight.outbox (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS spotlight.scope_config (
    tenant_id UUID NOT NULL REFERENCES public.tenants (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, provider, entity_type)
);

-- +migrate Down
DROP TABLE IF EXISTS spotlight.scope_config;
DROP TABLE IF EXISTS spotlight.outbox;
DROP TABLE IF EXISTS spotlight.provider_state;
DROP TABLE IF EXISTS spotlight.document_acl;
DROP TABLE IF EXISTS spotlight.documents;
DROP SCHEMA IF EXISTS spotlight;
DROP EXTENSION IF EXISTS pg_textsearch;
DROP EXTENSION IF EXISTS vector;
