-- +migrate Up

-- Change DROP_TABLE: tabs
DROP TABLE IF EXISTS tabs CASCADE;

-- Change DROP_INDEX: tabs_tenant_id_idx
DROP INDEX IF EXISTS tabs_tenant_id_idx;

-- +migrate Down

-- Undo DROP_TABLE: tabs
CREATE TABLE tabs (
	id         SERIAL8 PRIMARY KEY,
	tenant_id  UUID NOT NULL,
	href       VARCHAR(255) NOT NULL,
	position   INT8 DEFAULT 0 NOT NULL,
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	CONSTRAINT tabs_tenant_id_href_user_id_key UNIQUE (tenant_id, href, user_id)
);

-- Undo DROP_INDEX: tabs_tenant_id_idx
CREATE INDEX tabs_tenant_id_idx ON tabs (tenant_id);