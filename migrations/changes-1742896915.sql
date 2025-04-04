-- +migrate Up

-- Change CREATE_TABLE: tenants
CREATE TABLE tenants (
	id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
	name       VARCHAR(255) NOT NULL UNIQUE,
	domain     VARCHAR(255),
	is_active  BOOL DEFAULT true NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change ADD_COLUMN: tenant_id
ALTER TABLE user_groups ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_units ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE expense_categories ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE roles ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_orders ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE uploads ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE positions ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE passports ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE counterparty ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE permissions ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE users ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE money_accounts ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE transactions ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE prompts ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE sessions ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE message_templates ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE tabs ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_positions ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE employees ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE companies ADD COLUMN tenant_id TEXT REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE clients ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE action_logs ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE warehouse_products ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory_checks ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE chats ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE dialogues ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change ADD_COLUMN: tenant_id
ALTER TABLE inventory_check_results ADD COLUMN tenant_id TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE;

-- Change CREATE_INDEX: employees_tenant_id_idx
CREATE INDEX employees_tenant_id_idx ON employees (tenant_id);

-- Change CREATE_INDEX: dialogues_tenant_id_idx
CREATE INDEX dialogues_tenant_id_idx ON dialogues (tenant_id);

-- Change CREATE_INDEX: sessions_tenant_id_idx
CREATE INDEX sessions_tenant_id_idx ON sessions (tenant_id);

-- Change CREATE_INDEX: idx_message_templates_tenant_id
CREATE INDEX idx_message_templates_tenant_id ON message_templates (tenant_id);

-- Change CREATE_INDEX: warehouse_orders_tenant_id_idx
CREATE INDEX warehouse_orders_tenant_id_idx ON warehouse_orders (tenant_id);

-- Change CREATE_INDEX: idx_chats_tenant_id
CREATE INDEX idx_chats_tenant_id ON chats (tenant_id);

-- Change CREATE_INDEX: permissions_tenant_id_idx
CREATE INDEX permissions_tenant_id_idx ON permissions (tenant_id);

-- Change CREATE_INDEX: tabs_tenant_id_idx
CREATE INDEX tabs_tenant_id_idx ON tabs (tenant_id);

-- Change CREATE_INDEX: employees_last_name_idx
CREATE INDEX employees_last_name_idx ON employees (last_name);

-- Change CREATE_INDEX: roles_tenant_id_idx
CREATE INDEX roles_tenant_id_idx ON roles (tenant_id);

-- Change CREATE_INDEX: inventory_tenant_id_idx
CREATE INDEX inventory_tenant_id_idx ON inventory (tenant_id);

-- Change CREATE_INDEX: warehouse_positions_tenant_id_idx
CREATE INDEX warehouse_positions_tenant_id_idx ON warehouse_positions (tenant_id);

-- Change CREATE_INDEX: employees_first_name_idx
CREATE INDEX employees_first_name_idx ON employees (first_name);

-- Change CREATE_INDEX: uploads_tenant_id_idx
CREATE INDEX uploads_tenant_id_idx ON uploads (tenant_id);

-- Change CREATE_INDEX: warehouse_units_tenant_id_idx
CREATE INDEX warehouse_units_tenant_id_idx ON warehouse_units (tenant_id);

-- Change CREATE_INDEX: users_tenant_id_idx
CREATE INDEX users_tenant_id_idx ON users (tenant_id);

-- Change CREATE_INDEX: inventory_checks_tenant_id_idx
CREATE INDEX inventory_checks_tenant_id_idx ON inventory_checks (tenant_id);

-- Change CREATE_INDEX: expense_categories_tenant_id_idx
CREATE INDEX expense_categories_tenant_id_idx ON expense_categories (tenant_id);

-- Change CREATE_INDEX: inventory_check_results_tenant_id_idx
CREATE INDEX inventory_check_results_tenant_id_idx ON inventory_check_results (tenant_id);

-- Change CREATE_INDEX: action_logs_tenant_id_idx
CREATE INDEX action_logs_tenant_id_idx ON action_logs (tenant_id);

-- Change CREATE_INDEX: idx_clients_tenant_id
CREATE INDEX idx_clients_tenant_id ON clients (tenant_id);

-- Change CREATE_INDEX: money_accounts_tenant_id_idx
CREATE INDEX money_accounts_tenant_id_idx ON money_accounts (tenant_id);

-- Change CREATE_INDEX: transactions_tenant_id_idx
CREATE INDEX transactions_tenant_id_idx ON transactions (tenant_id);

-- Change CREATE_INDEX: counterparty_tenant_id_idx
CREATE INDEX counterparty_tenant_id_idx ON counterparty (tenant_id);

-- Change CREATE_INDEX: warehouse_products_tenant_id_idx
CREATE INDEX warehouse_products_tenant_id_idx ON warehouse_products (tenant_id);

-- Change CREATE_INDEX: employees_phone_idx
CREATE INDEX employees_phone_idx ON employees (phone);

-- Change CREATE_INDEX: authentication_logs_tenant_id_idx
CREATE INDEX authentication_logs_tenant_id_idx ON authentication_logs (tenant_id);

-- Change CREATE_INDEX: prompts_tenant_id_idx
CREATE INDEX prompts_tenant_id_idx ON prompts (tenant_id);

-- Change CREATE_INDEX: user_groups_tenant_id_idx
CREATE INDEX user_groups_tenant_id_idx ON user_groups (tenant_id);

-- Change CREATE_INDEX: positions_tenant_id_idx
CREATE INDEX positions_tenant_id_idx ON positions (tenant_id);

-- Change CREATE_INDEX: employees_email_idx
CREATE INDEX employees_email_idx ON employees (email);

-- MANUAL
-- Change UNIQUE constraints to include tenant_id
-- Users
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;
ALTER TABLE users ADD CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email);
ALTER TABLE users ADD CONSTRAINT users_tenant_phone_key UNIQUE (tenant_id, phone);

-- Positions
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_name_key;
ALTER TABLE positions ADD CONSTRAINT positions_tenant_name_key UNIQUE (tenant_id, name);

-- Warehouse Units
ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_title_key;
ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_short_title_key;
ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_tenant_title_key UNIQUE (tenant_id, title);
ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_tenant_short_title_key UNIQUE (tenant_id, short_title);

-- Warehouse Positions
ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_barcode_key;
ALTER TABLE warehouse_positions ADD CONSTRAINT warehouse_positions_tenant_barcode_key UNIQUE (tenant_id, barcode);

-- Warehouse Products
ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_rfid_key;
ALTER TABLE warehouse_products ADD CONSTRAINT warehouse_products_tenant_rfid_key UNIQUE (tenant_id, rfid);

-- Inventory
ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_name_key;
ALTER TABLE inventory ADD CONSTRAINT inventory_tenant_name_key UNIQUE (tenant_id, name);

-- Expense Categories
ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_name_key;
ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_tenant_name_key UNIQUE (tenant_id, name);

-- Money Accounts
ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_account_number_key;
ALTER TABLE money_accounts ADD CONSTRAINT money_accounts_tenant_account_number_key UNIQUE (tenant_id, account_number);

-- Roles
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_key;
ALTER TABLE roles ADD CONSTRAINT roles_tenant_name_key UNIQUE (tenant_id, name);

-- User Groups
ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_name_key;
ALTER TABLE user_groups ADD CONSTRAINT user_groups_tenant_name_key UNIQUE (tenant_id, name);

-- Permissions
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;
ALTER TABLE permissions ADD CONSTRAINT permissions_tenant_name_key UNIQUE (tenant_id, name);

-- Tabs
ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_href_user_id_key;
ALTER TABLE tabs ADD CONSTRAINT tabs_tenant_href_user_id_key UNIQUE (tenant_id, href, user_id);

-- Companies
ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_name_key;
ALTER TABLE companies ADD CONSTRAINT companies_tenant_name_key UNIQUE (tenant_id, name);

-- Clients
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_phone_number_key;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_email_key;
ALTER TABLE clients ADD CONSTRAINT clients_tenant_phone_number_key UNIQUE (tenant_id, phone_number);
ALTER TABLE clients ADD CONSTRAINT clients_tenant_email_key UNIQUE (tenant_id, email);

-- Passports
ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_passport_number_series_key;
ALTER TABLE passports ADD CONSTRAINT passports_tenant_passport_number_series_key UNIQUE (tenant_id, passport_number, series);

-- Employees
ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_email_key;
ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_phone_key;
ALTER TABLE employees ADD CONSTRAINT employees_tenant_email_key UNIQUE (tenant_id, email);
ALTER TABLE employees ADD CONSTRAINT employees_tenant_phone_key UNIQUE (tenant_id, phone);

-- Counterparty
ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tin_key;
ALTER TABLE counterparty ADD CONSTRAINT counterparty_tenant_tin_key UNIQUE (tenant_id, tin);

-- Uploads
ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_hash_key;
ALTER TABLE uploads ADD CONSTRAINT uploads_tenant_hash_key UNIQUE (tenant_id, hash);
-- MANUAL

-- +migrate Down

-- Undo CREATE_INDEX: employees_email_idx
DROP INDEX employees_email_idx;

-- Undo CREATE_INDEX: positions_tenant_id_idx
DROP INDEX positions_tenant_id_idx;

-- Undo CREATE_INDEX: user_groups_tenant_id_idx
DROP INDEX user_groups_tenant_id_idx;

-- Undo CREATE_INDEX: prompts_tenant_id_idx
DROP INDEX prompts_tenant_id_idx;

-- Undo CREATE_INDEX: authentication_logs_tenant_id_idx
DROP INDEX authentication_logs_tenant_id_idx;

-- Undo CREATE_INDEX: employees_phone_idx
DROP INDEX employees_phone_idx;

-- Undo CREATE_INDEX: warehouse_products_tenant_id_idx
DROP INDEX warehouse_products_tenant_id_idx;

-- Undo CREATE_INDEX: counterparty_tenant_id_idx
DROP INDEX counterparty_tenant_id_idx;

-- Undo CREATE_INDEX: transactions_tenant_id_idx
DROP INDEX transactions_tenant_id_idx;

-- Undo CREATE_INDEX: money_accounts_tenant_id_idx
DROP INDEX money_accounts_tenant_id_idx;

-- Undo CREATE_INDEX: idx_clients_tenant_id
DROP INDEX idx_clients_tenant_id;

-- Undo CREATE_INDEX: action_logs_tenant_id_idx
DROP INDEX action_logs_tenant_id_idx;

-- Undo CREATE_INDEX: inventory_check_results_tenant_id_idx
DROP INDEX inventory_check_results_tenant_id_idx;

-- Undo CREATE_INDEX: expense_categories_tenant_id_idx
DROP INDEX expense_categories_tenant_id_idx;

-- Undo CREATE_INDEX: inventory_checks_tenant_id_idx
DROP INDEX inventory_checks_tenant_id_idx;

-- Undo CREATE_INDEX: users_tenant_id_idx
DROP INDEX users_tenant_id_idx;

-- Undo CREATE_INDEX: warehouse_units_tenant_id_idx
DROP INDEX warehouse_units_tenant_id_idx;

-- Undo CREATE_INDEX: uploads_tenant_id_idx
DROP INDEX uploads_tenant_id_idx;

-- Undo CREATE_INDEX: employees_first_name_idx
DROP INDEX employees_first_name_idx;

-- Undo CREATE_INDEX: warehouse_positions_tenant_id_idx
DROP INDEX warehouse_positions_tenant_id_idx;

-- Undo CREATE_INDEX: inventory_tenant_id_idx
DROP INDEX inventory_tenant_id_idx;

-- Undo CREATE_INDEX: roles_tenant_id_idx
DROP INDEX roles_tenant_id_idx;

-- Undo CREATE_INDEX: employees_last_name_idx
DROP INDEX employees_last_name_idx;

-- Undo CREATE_INDEX: tabs_tenant_id_idx
DROP INDEX tabs_tenant_id_idx;

-- Undo CREATE_INDEX: permissions_tenant_id_idx
DROP INDEX permissions_tenant_id_idx;

-- Undo CREATE_INDEX: idx_chats_tenant_id
DROP INDEX idx_chats_tenant_id;

-- Undo CREATE_INDEX: warehouse_orders_tenant_id_idx
DROP INDEX warehouse_orders_tenant_id_idx;

-- Undo CREATE_INDEX: idx_message_templates_tenant_id
DROP INDEX idx_message_templates_tenant_id;

-- Undo CREATE_INDEX: sessions_tenant_id_idx
DROP INDEX sessions_tenant_id_idx;

-- Undo CREATE_INDEX: dialogues_tenant_id_idx
DROP INDEX dialogues_tenant_id_idx;

-- Undo CREATE_INDEX: employees_tenant_id_idx
DROP INDEX employees_tenant_id_idx;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory_check_results DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE dialogues DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE chats DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory_checks DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_products DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE action_logs DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE clients DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE companies DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE authentication_logs DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE employees DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_positions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE tabs DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE message_templates DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE sessions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE prompts DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE transactions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE money_accounts DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE permissions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE counterparty DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE passports DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE inventory DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE positions DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE uploads DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_orders DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE roles DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE expense_categories DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE warehouse_units DROP COLUMN IF EXISTS tenant_id;

-- Undo ADD_COLUMN: tenant_id
ALTER TABLE user_groups DROP COLUMN IF EXISTS tenant_id;

-- MANUAL
-- Undo UNIQUE constraints changes
-- Uploads
ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_tenant_hash_key;
ALTER TABLE uploads ADD CONSTRAINT uploads_hash_key UNIQUE (hash);

-- Counterparty
ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tenant_tin_key;
ALTER TABLE counterparty ADD CONSTRAINT counterparty_tin_key UNIQUE (tin);

-- Employees
ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_email_key;
ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_phone_key;
ALTER TABLE employees ADD CONSTRAINT employees_email_key UNIQUE (email);
ALTER TABLE employees ADD CONSTRAINT employees_phone_key UNIQUE (phone);

-- Passports
ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_tenant_passport_number_series_key;
ALTER TABLE passports ADD CONSTRAINT passports_passport_number_series_key UNIQUE (passport_number, series);

-- Clients
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_phone_number_key;
ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_email_key;
ALTER TABLE clients ADD CONSTRAINT clients_phone_number_key UNIQUE (phone_number);
ALTER TABLE clients ADD CONSTRAINT clients_email_key UNIQUE (email);

-- Companies
ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_tenant_name_key;
ALTER TABLE companies ADD CONSTRAINT companies_name_key UNIQUE (name);

-- Tabs
ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_tenant_href_user_id_key;
ALTER TABLE tabs ADD CONSTRAINT tabs_href_user_id_key UNIQUE (href, user_id);

-- Permissions
ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_name_key;
ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);

-- User Groups
ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_tenant_name_key;
ALTER TABLE user_groups ADD CONSTRAINT user_groups_name_key UNIQUE (name);

-- Roles
ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_name_key;
ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);

-- Money Accounts
ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_tenant_account_number_key;
ALTER TABLE money_accounts ADD CONSTRAINT money_accounts_account_number_key UNIQUE (account_number);

-- Expense Categories
ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_tenant_name_key;
ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_name_key UNIQUE (name);

-- Inventory
ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_tenant_name_key;
ALTER TABLE inventory ADD CONSTRAINT inventory_name_key UNIQUE (name);

-- Warehouse Products
ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_tenant_rfid_key;
ALTER TABLE warehouse_products ADD CONSTRAINT warehouse_products_rfid_key UNIQUE (rfid);

-- Warehouse Positions
ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_tenant_barcode_key;
ALTER TABLE warehouse_positions ADD CONSTRAINT warehouse_positions_barcode_key UNIQUE (barcode);

-- Warehouse Units
ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_title_key;
ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_short_title_key;
ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_title_key UNIQUE (title);
ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_short_title_key UNIQUE (short_title);

-- Positions
ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_tenant_name_key;
ALTER TABLE positions ADD CONSTRAINT positions_name_key UNIQUE (name);

-- Users
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_email_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_phone_key;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
ALTER TABLE users ADD CONSTRAINT users_phone_key UNIQUE (phone);
-- MANUAL

-- Undo CREATE_TABLE: tenants
DROP TABLE IF EXISTS tenants CASCADE;
