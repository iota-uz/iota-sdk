-- +migrate Up

-- Change CREATE_TABLE: tenants
CREATE TABLE tenants (
	id         UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	name       VARCHAR(255) NOT NULL UNIQUE,
	domain     VARCHAR(255),
	is_active  BOOL DEFAULT true NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE prompts ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE permissions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE permissions ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE money_accounts ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE money_accounts ADD UNIQUE (tenant_id, account_number);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE message_templates ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_orders ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE counterparty ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE counterparty ADD UNIQUE (tenant_id, tin);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE positions ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE positions ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE transactions ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_passport_number_series_key;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE passports ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE passports
	ADD CONSTRAINT passports_tenant_passport_number_series_key UNIQUE (tenant_id, passport_number, series);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE user_groups ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_name_key;

ALTER TABLE user_groups ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE uploads ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_hash_key;

ALTER TABLE uploads ADD UNIQUE (tenant_id, hash);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE inventory ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE users ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;

ALTER TABLE users ADD UNIQUE (tenant_id, email);

ALTER TABLE users ADD UNIQUE (tenant_id, phone);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_units ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE warehouse_units ADD UNIQUE (tenant_id, title);

ALTER TABLE warehouse_units ADD UNIQUE (tenant_id, short_title);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE expense_categories ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE expense_categories ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE roles ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_key;

ALTER TABLE roles ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE sessions ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE clients ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE clients ADD UNIQUE (tenant_id, phone_number);

ALTER TABLE clients ADD UNIQUE (tenant_id, email);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_positions ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_barcode_key;

ALTER TABLE warehouse_positions ADD UNIQUE (tenant_id, barcode);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE tabs ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_href_user_id_key;

ALTER TABLE tabs ADD UNIQUE (tenant_id, href, user_id);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory_checks ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE companies ADD COLUMN tenant_id UUID REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE companies ADD UNIQUE (tenant_id, name);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE chats ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE dialogues ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE employees ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_email_key;

ALTER TABLE employees ADD UNIQUE (tenant_id, email);

ALTER TABLE employees ADD UNIQUE (tenant_id, phone);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory_check_results ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE action_logs ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_products ADD COLUMN tenant_id UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_rfid;

ALTER TABLE warehouse_products ADD UNIQUE (tenant_id, rfid);

-- Change CREATE_INDEX: user_groups_tenant_id_idx
CREATE INDEX user_groups_tenant_id_idx ON user_groups (tenant_id);

-- Change CREATE_INDEX: counterparty_tenant_id_idx
CREATE INDEX counterparty_tenant_id_idx ON counterparty (tenant_id);

-- Change CREATE_INDEX: action_logs_tenant_id_idx
CREATE INDEX action_logs_tenant_id_idx ON action_logs (tenant_id);

-- Change CREATE_INDEX: employees_tenant_id_idx
CREATE INDEX employees_tenant_id_idx ON employees (tenant_id);

-- Change CREATE_INDEX: warehouse_units_tenant_id_idx
CREATE INDEX warehouse_units_tenant_id_idx ON warehouse_units (tenant_id);

-- Change CREATE_INDEX: warehouse_positions_tenant_id_idx
CREATE INDEX warehouse_positions_tenant_id_idx ON warehouse_positions (tenant_id);

-- Change CREATE_INDEX: users_tenant_id_idx
CREATE INDEX users_tenant_id_idx ON users (tenant_id);

-- Change CREATE_INDEX: inventory_checks_tenant_id_idx
CREATE INDEX inventory_checks_tenant_id_idx ON inventory_checks (tenant_id);

-- Change CREATE_INDEX: employees_first_name_idx
CREATE INDEX employees_first_name_idx ON employees (first_name);

-- Change CREATE_INDEX: inventory_tenant_id_idx
CREATE INDEX inventory_tenant_id_idx ON inventory (tenant_id);

-- Change CREATE_INDEX: authentication_logs_tenant_id_idx
CREATE INDEX authentication_logs_tenant_id_idx ON authentication_logs (tenant_id);

-- Change CREATE_INDEX: dialogues_tenant_id_idx
CREATE INDEX dialogues_tenant_id_idx ON dialogues (tenant_id);

-- Change CREATE_INDEX: employees_email_idx
CREATE INDEX employees_email_idx ON employees (email);

-- Change CREATE_INDEX: inventory_check_results_tenant_id_idx
CREATE INDEX inventory_check_results_tenant_id_idx ON inventory_check_results (tenant_id);

-- Change CREATE_INDEX: sessions_tenant_id_idx
CREATE INDEX sessions_tenant_id_idx ON sessions (tenant_id);

-- Change CREATE_INDEX: prompts_tenant_id_idx
CREATE INDEX prompts_tenant_id_idx ON prompts (tenant_id);

-- Change CREATE_INDEX: warehouse_products_tenant_id_idx
CREATE INDEX warehouse_products_tenant_id_idx ON warehouse_products (tenant_id);

-- Change CREATE_INDEX: transactions_tenant_id_idx
CREATE INDEX transactions_tenant_id_idx ON transactions (tenant_id);

-- Change CREATE_INDEX: idx_message_templates_tenant_id
CREATE INDEX idx_message_templates_tenant_id ON message_templates (tenant_id);

-- Change CREATE_INDEX: positions_tenant_id_idx
CREATE INDEX positions_tenant_id_idx ON positions (tenant_id);

-- Change CREATE_INDEX: warehouse_orders_tenant_id_idx
CREATE INDEX warehouse_orders_tenant_id_idx ON warehouse_orders (tenant_id);

-- Change CREATE_INDEX: permissions_tenant_id_idx
CREATE INDEX permissions_tenant_id_idx ON permissions (tenant_id);

-- Change CREATE_INDEX: idx_chats_tenant_id
CREATE INDEX idx_chats_tenant_id ON chats (tenant_id);

-- Change CREATE_INDEX: expense_categories_tenant_id_idx
CREATE INDEX expense_categories_tenant_id_idx ON expense_categories (tenant_id);

-- Change CREATE_INDEX: employees_phone_idx
CREATE INDEX employees_phone_idx ON employees (phone);

-- Change CREATE_INDEX: roles_tenant_id_idx
CREATE INDEX roles_tenant_id_idx ON roles (tenant_id);

-- Change CREATE_INDEX: uploads_tenant_id_idx
CREATE INDEX uploads_tenant_id_idx ON uploads (tenant_id);

-- Change CREATE_INDEX: tabs_tenant_id_idx
CREATE INDEX tabs_tenant_id_idx ON tabs (tenant_id);

-- Change CREATE_INDEX: money_accounts_tenant_id_idx
CREATE INDEX money_accounts_tenant_id_idx ON money_accounts (tenant_id);

-- Change CREATE_INDEX: idx_clients_tenant_id
CREATE INDEX idx_clients_tenant_id ON clients (tenant_id);

-- Change CREATE_INDEX: employees_last_name_idx
CREATE INDEX employees_last_name_idx ON employees (last_name);


-- +migrate Down

-- Undo CREATE_INDEX: employees_last_name_idx
DROP INDEX employees_last_name_idx;

-- Undo CREATE_INDEX: idx_clients_tenant_id
DROP INDEX idx_clients_tenant_id;

-- Undo CREATE_INDEX: money_accounts_tenant_id_idx
DROP INDEX money_accounts_tenant_id_idx;

-- Undo CREATE_INDEX: tabs_tenant_id_idx
DROP INDEX tabs_tenant_id_idx;

-- Undo CREATE_INDEX: uploads_tenant_id_idx
DROP INDEX uploads_tenant_id_idx;

-- Undo CREATE_INDEX: roles_tenant_id_idx
DROP INDEX roles_tenant_id_idx;

-- Undo CREATE_INDEX: employees_phone_idx
DROP INDEX employees_phone_idx;

-- Undo CREATE_INDEX: expense_categories_tenant_id_idx
DROP INDEX expense_categories_tenant_id_idx;

-- Undo CREATE_INDEX: idx_chats_tenant_id
DROP INDEX idx_chats_tenant_id;

-- Undo CREATE_INDEX: permissions_tenant_id_idx
DROP INDEX permissions_tenant_id_idx;

-- Undo CREATE_INDEX: warehouse_orders_tenant_id_idx
DROP INDEX warehouse_orders_tenant_id_idx;

-- Undo CREATE_INDEX: positions_tenant_id_idx
DROP INDEX positions_tenant_id_idx;

-- Undo CREATE_INDEX: idx_message_templates_tenant_id
DROP INDEX idx_message_templates_tenant_id;

-- Undo CREATE_INDEX: transactions_tenant_id_idx
DROP INDEX transactions_tenant_id_idx;

-- Undo CREATE_INDEX: warehouse_products_tenant_id_idx
DROP INDEX warehouse_products_tenant_id_idx;

-- Undo CREATE_INDEX: prompts_tenant_id_idx
DROP INDEX prompts_tenant_id_idx;

-- Undo CREATE_INDEX: sessions_tenant_id_idx
DROP INDEX sessions_tenant_id_idx;

-- Undo CREATE_INDEX: inventory_check_results_tenant_id_idx
DROP INDEX inventory_check_results_tenant_id_idx;

-- Undo CREATE_INDEX: employees_email_idx
DROP INDEX employees_email_idx;

-- Undo CREATE_INDEX: dialogues_tenant_id_idx
DROP INDEX dialogues_tenant_id_idx;

-- Undo CREATE_INDEX: authentication_logs_tenant_id_idx
DROP INDEX authentication_logs_tenant_id_idx;

-- Undo CREATE_INDEX: inventory_tenant_id_idx
DROP INDEX inventory_tenant_id_idx;

-- Undo CREATE_INDEX: employees_first_name_idx
DROP INDEX employees_first_name_idx;

-- Undo CREATE_INDEX: inventory_checks_tenant_id_idx
DROP INDEX inventory_checks_tenant_id_idx;

-- Undo CREATE_INDEX: users_tenant_id_idx
DROP INDEX users_tenant_id_idx;

-- Undo CREATE_INDEX: warehouse_positions_tenant_id_idx
DROP INDEX warehouse_positions_tenant_id_idx;

-- Undo CREATE_INDEX: warehouse_units_tenant_id_idx
DROP INDEX warehouse_units_tenant_id_idx;

-- Undo CREATE_INDEX: employees_tenant_id_idx
DROP INDEX employees_tenant_id_idx;

-- Undo CREATE_INDEX: action_logs_tenant_id_idx
DROP INDEX action_logs_tenant_id_idx;

-- Undo CREATE_INDEX: counterparty_tenant_id_idx
DROP INDEX counterparty_tenant_id_idx;

-- Undo CREATE_INDEX: user_groups_tenant_id_idx
DROP INDEX user_groups_tenant_id_idx;

ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_tenant_id_rfid_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_products DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE warehouse_products ADD CONSTRAINT warehouse_products_rfid UNIQUE (rfid);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE action_logs DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory_check_results DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_id_phone_key;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_id_email_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE employees DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE employees ADD CONSTRAINT employees_email_key UNIQUE (email);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE dialogues DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE chats DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE companies DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory_checks DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_tenant_id_href_user_id_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE tabs DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE tabs ADD CONSTRAINT tabs_href_user_id_key UNIQUE (href, user_id);

ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_tenant_id_barcode_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_positions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE warehouse_positions ADD CONSTRAINT warehouse_positions_barcode_key UNIQUE (barcode);

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_id_email_key;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_id_phone_number_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE clients DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE sessions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE roles DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);

ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE expense_categories DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_id_short_title_key;

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_id_title_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_units DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_phone_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_email_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE users ADD CONSTRAINT users_phone_key UNIQUE (phone);

ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_tenant_id_hash_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE uploads DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE uploads ADD CONSTRAINT uploads_hash_key UNIQUE (hash);

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE user_groups DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE user_groups ADD CONSTRAINT user_groups_name_key UNIQUE (name);

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_tenant_passport_number_series_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE passports DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE passports ADD CONSTRAINT passports_passport_number_series_key UNIQUE (passport_number, series);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE transactions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE positions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tenant_id_tin_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE counterparty DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_orders DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE message_templates DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_tenant_id_account_number_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE money_accounts DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_id_name_key;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE permissions DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE prompts DROP COLUMN IF EXISTS tenant_id;

-- Undo CREATE_TABLE: tenants
DROP TABLE IF EXISTS tenants CASCADE;

