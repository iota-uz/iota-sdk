-- +migrate Up

-- Change CREATE_TABLE: tenants
CREATE TABLE tenants (
	id         UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	name       VARCHAR(255) NOT NULL,
	domain     VARCHAR(255),
	is_active  BOOL DEFAULT true NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now(),
	CONSTRAINT tenants_name_key UNIQUE (name)
);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE positions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE positions ADD CONSTRAINT positions_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE passports ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_passport_number_series_key;

ALTER TABLE passports
	ADD CONSTRAINT passports_tenant_passport_number_series_key UNIQUE (tenant_id, passport_number, series);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE uploads ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_hash_key;

ALTER TABLE uploads ADD CONSTRAINT uploads_tenant_hash_key UNIQUE (tenant_id, hash);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE users ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

ALTER TABLE users ADD CONSTRAINT users_tenant_phone_key UNIQUE (tenant_id, phone);

ALTER TABLE users ADD CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE user_groups ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_name_key;

ALTER TABLE user_groups ADD CONSTRAINT user_groups_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE counterparty ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE counterparty ADD CONSTRAINT counterparty_tenant_tin_key UNIQUE (tenant_id, tin);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE permissions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;

ALTER TABLE permissions ADD CONSTRAINT permissions_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE dialogues ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE tabs ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_href_user_id_key;

ALTER TABLE tabs ADD CONSTRAINT tabs_tenant_href_user_id_key UNIQUE (tenant_id, href, user_id);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE message_templates ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE money_accounts ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE money_accounts ADD CONSTRAINT money_accounts_tenant_account_number_key UNIQUE (tenant_id, account_number);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE expense_categories ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE roles ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_key;

ALTER TABLE roles ADD CONSTRAINT roles_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE inventory ADD CONSTRAINT inventory_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE action_logs ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE transactions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE prompts ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE clients ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE clients ADD CONSTRAINT clients_tenant_phone_number UNIQUE (tenant_id, phone_number);

ALTER TABLE clients ADD CONSTRAINT clients_tenant_email UNIQUE (tenant_id, email);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE employees ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_email_key;

ALTER TABLE employees ADD CONSTRAINT employees_tenant_email_key UNIQUE (tenant_id, email);

ALTER TABLE employees ADD CONSTRAINT employees_tenant_phone_key UNIQUE (tenant_id, phone);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE companies ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE companies ADD CONSTRAINT companies_tenant_name_key UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE sessions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE chats ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change CREATE_INDEX: prompts_tenant_id_idx
CREATE INDEX prompts_tenant_id_idx ON prompts (tenant_id);

-- Change CREATE_INDEX: transactions_tenant_id_idx
CREATE INDEX transactions_tenant_id_idx ON transactions (tenant_id);

-- Change CREATE_INDEX: dialogues_tenant_id_idx
CREATE INDEX dialogues_tenant_id_idx ON dialogues (tenant_id);

-- +migrate Down

-- Undo CREATE_INDEX: dialogues_tenant_id_idx
DROP INDEX dialogues_tenant_id_idx;

-- Undo CREATE_INDEX: transactions_tenant_id_idx
DROP INDEX transactions_tenant_id_idx;

-- Undo CREATE_INDEX: prompts_tenant_id_idx
DROP INDEX prompts_tenant_id_idx;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE chats DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE sessions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_tenant_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE companies DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_phone_key;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_email_key;

ALTER TABLE employees ADD CONSTRAINT employees_email_key UNIQUE (email);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE employees DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_email;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_phone_number;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE clients DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE prompts DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE transactions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE action_logs DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_tenant_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_name_key;

ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE roles DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_tenant_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE expense_categories DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_tenant_account_number_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE money_accounts DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE message_templates DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_tenant_href_user_id_key;

ALTER TABLE tabs ADD CONSTRAINT tabs_href_user_id_key UNIQUE (href, user_id);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE tabs DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE dialogues DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_name_key;

ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE permissions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tenant_tin_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE counterparty DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_tenant_name_key;

ALTER TABLE user_groups ADD CONSTRAINT user_groups_name_key UNIQUE (name);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE user_groups DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_email_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_phone_key;

ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_tenant_hash_key;

ALTER TABLE uploads ADD CONSTRAINT uploads_hash_key UNIQUE (hash);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE uploads DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_tenant_passport_number_series_key;

ALTER TABLE passports ADD CONSTRAINT passports_passport_number_series_key UNIQUE (passport_number, series);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE passports DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_tenant_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE positions DROP COLUMN IF EXISTS tenant_id;

-- Undo CREATE_TABLE: tenants
DROP TABLE IF EXISTS tenants CASCADE;

