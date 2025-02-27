-- +migrate Up

-- Change CREATE_TABLE: payments
CREATE TABLE payments (id SERIAL8 PRIMARY KEY, transaction_id INT8 NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT, counterparty_id INT8 NOT NULL REFERENCES counterparty (id) ON DELETE RESTRICT, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: sessions
CREATE TABLE sessions (expires_at TIMESTAMPTZ NOT NULL, ip VARCHAR(255) NOT NULL, user_agent VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp(), token VARCHAR(255) NOT NULL PRIMARY KEY, user_id INT8 NOT NULL CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE);

-- Change CREATE_TABLE: counterparty_contacts
CREATE TABLE counterparty_contacts (last_name VARCHAR(255) NOT NULL, middle_name VARCHAR(255) NULL, email VARCHAR(255), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), counterparty_id INT8 NOT NULL REFERENCES counterparty (id) ON DELETE CASCADE, first_name VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, phone VARCHAR(255));

-- Change CREATE_TABLE: roles
CREATE TABLE roles (created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL UNIQUE, description STRING);

-- Change CREATE_TABLE: companies
CREATE TABLE companies (logo_id INT8, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL, about STRING, address VARCHAR(255), phone VARCHAR(255));

-- Change CREATE_TABLE: expense_categories
CREATE TABLE expense_categories (updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL, description STRING, amount DECIMAL(9,2) NOT NULL, amount_currency_id VARCHAR(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT, created_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: currencies
CREATE TABLE currencies (name VARCHAR(255) NOT NULL, symbol VARCHAR(3) NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), code VARCHAR(3) NOT NULL PRIMARY KEY);

-- Change CREATE_TABLE: users
CREATE TABLE users (password VARCHAR(255), avatar_id INT8 REFERENCES uploads (id) ON DELETE SET NULL, last_login TIMESTAMP NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp(), last_ip VARCHAR(255) NULL, last_action TIMESTAMPTZ NULL, id SERIAL8 PRIMARY KEY, first_name VARCHAR(255) NOT NULL, last_name VARCHAR(255) NOT NULL, middle_name VARCHAR(255), email VARCHAR(255) NOT NULL UNIQUE, ui_language VARCHAR(3) NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp());

-- Change CREATE_TABLE: role_permissions
CREATE TABLE role_permissions (role_id INT8 NOT NULL UNIQUE CONSTRAINT fk_role_permissions_role REFERENCES roles (role_id) ON DELETE CASCADE, permission_id UUID NOT NULL UNIQUE CONSTRAINT fk_permission REFERENCES permissions (permission_id) ON DELETE CASCADE);

-- Change CREATE_TABLE: positions
CREATE TABLE positions (id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL, description STRING, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: uploaded_images
CREATE TABLE uploaded_images (type VARCHAR(255) NOT NULL, size FLOAT8 NOT NULL, width INT8 NOT NULL, height INT8 NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, upload_id INT8 NOT NULL REFERENCES uploads (id) ON DELETE CASCADE);

-- Change CREATE_TABLE: employees
CREATE TABLE employees (id SERIAL8 PRIMARY KEY, first_name VARCHAR(255) NOT NULL, email VARCHAR(255) NOT NULL UNIQUE, salary_currency_id VARCHAR(3) REFERENCES currencies (code) ON DELETE SET NULL, avatar_id INT8 REFERENCES uploads (id) ON DELETE SET NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), last_name VARCHAR(255) NOT NULL, middle_name VARCHAR(255), phone VARCHAR(255), salary DECIMAL(9,2) NOT NULL, hourly_rate DECIMAL(9,2) NOT NULL, coefficient FLOAT8 NOT NULL, updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: prompts
CREATE TABLE prompts (title VARCHAR(255) NOT NULL, description STRING NOT NULL, prompt STRING NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), id VARCHAR(30) PRIMARY KEY);

-- Change CREATE_TABLE: counterparty
CREATE TABLE counterparty (created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, tin VARCHAR(20), name VARCHAR(255) NOT NULL, type VARCHAR(255) NOT NULL, legal_type VARCHAR(255) NOT NULL, legal_address VARCHAR(255));

-- Change CREATE_TABLE: tabs
CREATE TABLE tabs ("position" INT8 NOT NULL DEFAULT 0, id SERIAL8 PRIMARY KEY, href VARCHAR(255) NOT NULL UNIQUE, user_id INT8 NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE);

-- Change CREATE_TABLE: employee_positions
CREATE TABLE employee_positions (employee_id INT8 NOT NULL UNIQUE REFERENCES employees (id) ON DELETE CASCADE, position_id INT8 NOT NULL UNIQUE REFERENCES positions (id) ON DELETE CASCADE);

-- Change CREATE_TABLE: inventory
CREATE TABLE inventory (created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL, description STRING, currency_id VARCHAR(3) REFERENCES currencies (code) ON DELETE SET NULL, price DECIMAL(9,2) NOT NULL, quantity INT8 NOT NULL);

-- Change CREATE_TABLE: transactions
CREATE TABLE transactions (destination_account_id INT8 REFERENCES money_accounts (id) ON DELETE RESTRICT, transaction_date DATE NOT NULL DEFAULT current_date(), accounting_period DATE NOT NULL DEFAULT current_date(), transaction_type VARCHAR(255) NOT NULL, comment STRING, created_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, origin_account_id INT8 REFERENCES money_accounts (id) ON DELETE RESTRICT, amount DECIMAL(9,2) NOT NULL);

-- Change CREATE_TABLE: permissions
CREATE TABLE permissions (id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(), name VARCHAR(255) NOT NULL UNIQUE, resource VARCHAR(255) NOT NULL, action VARCHAR(255) NOT NULL, modifier VARCHAR(255) NOT NULL, description STRING);

-- Change CREATE_TABLE: expenses
CREATE TABLE expenses (id SERIAL8 PRIMARY KEY, transaction_id INT8 NOT NULL REFERENCES transactions (id) ON DELETE CASCADE, category_id INT8 NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: messages
CREATE TABLE messages (id SERIAL8 PRIMARY KEY, created_at TIMESTAMP(3) NOT NULL DEFAULT current_timestamp(), chat_id INT8 NOT NULL REFERENCES chats (id) ON DELETE RESTRICT ON UPDATE CASCADE, message STRING NOT NULL, sender_user_id INT8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE, sender_client_id INT8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE, is_read BOOL NOT NULL DEFAULT false, read_at TIMESTAMP(3));

-- Change CREATE_TABLE: message_media
CREATE TABLE message_media (message_id INT8 NOT NULL UNIQUE REFERENCES messages (id) ON DELETE CASCADE ON UPDATE CASCADE, upload_id INT8 NOT NULL UNIQUE REFERENCES uploads (id) ON DELETE CASCADE ON UPDATE CASCADE);

-- Change CREATE_TABLE: chats
CREATE TABLE chats (id SERIAL8 PRIMARY KEY, created_at TIMESTAMP(3) NOT NULL DEFAULT current_timestamp(), client_id INT8 NOT NULL REFERENCES clients (id) ON DELETE RESTRICT ON UPDATE CASCADE, last_message_at TIMESTAMP(3) DEFAULT current_timestamp());

-- Change CREATE_TABLE: employee_contacts
CREATE TABLE employee_contacts (id SERIAL8 PRIMARY KEY, employee_id INT8 NOT NULL REFERENCES employees (id) ON DELETE CASCADE, type VARCHAR(255) NOT NULL, value VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: action_logs
CREATE TABLE action_logs (user_id INT8 REFERENCES users (id) ON DELETE SET NULL, user_agent VARCHAR(255) NOT NULL, ip VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), before JSONB, id SERIAL8 PRIMARY KEY, method VARCHAR(255) NOT NULL, path VARCHAR(255) NOT NULL, after JSONB);

-- Change CREATE_TABLE: money_accounts
CREATE TABLE money_accounts (id SERIAL8 PRIMARY KEY, name VARCHAR(255) NOT NULL, account_number VARCHAR(255) NOT NULL, description STRING, balance DECIMAL(9,2) NOT NULL, balance_currency_id VARCHAR(3) NOT NULL REFERENCES currencies (code) ON DELETE CASCADE, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: authentication_logs
CREATE TABLE authentication_logs (id SERIAL8 PRIMARY KEY, user_id INT8 NOT NULL CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE, ip VARCHAR(255) NOT NULL, user_agent VARCHAR(255) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT current_timestamp());

-- Change CREATE_TABLE: user_roles
CREATE TABLE user_roles (user_id INT8 NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE, role_id INT8 NOT NULL UNIQUE REFERENCES roles (id) ON DELETE CASCADE, created_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: employee_meta
CREATE TABLE employee_meta (tin VARCHAR(255), birth_date DATE, resignation_date DATE, employee_id INT8 NOT NULL PRIMARY KEY REFERENCES employees (id) ON DELETE CASCADE, primary_language VARCHAR(255), secondary_language VARCHAR(255), pin VARCHAR(255), notes STRING, hire_date DATE);

-- Change CREATE_TABLE: message_templates
CREATE TABLE message_templates (id SERIAL8 PRIMARY KEY, template STRING NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp());

-- Change CREATE_TABLE: clients
CREATE TABLE clients (email VARCHAR(255), hourly_rate FLOAT8, gender VARCHAR(15), created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, last_name VARCHAR(255), address STRING, date_of_birth DATE, first_name VARCHAR(255) NOT NULL, middle_name VARCHAR(255), phone_number VARCHAR(255) NOT NULL);

-- Change CREATE_TABLE: uploads
CREATE TABLE uploads (created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, hash VARCHAR(255) NOT NULL UNIQUE, path VARCHAR(1024) NOT NULL DEFAULT '', size INT8 NOT NULL DEFAULT 0, mimetype VARCHAR(255) NOT NULL, type VARCHAR(255) NOT NULL);

-- Change CREATE_TABLE: dialogues
CREATE TABLE dialogues (label VARCHAR(255) NOT NULL, messages JSONB NOT NULL, created_at TIMESTAMPTZ DEFAULT current_timestamp(), updated_at TIMESTAMPTZ DEFAULT current_timestamp(), id SERIAL8 PRIMARY KEY, user_id INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE);


-- +migrate Down

-- Undo CREATE_TABLE: dialogues
DROP TABLE IF EXISTS dialogues CASCADE;

-- Undo CREATE_TABLE: uploads
DROP TABLE IF EXISTS uploads CASCADE;

-- Undo CREATE_TABLE: clients
DROP TABLE IF EXISTS clients CASCADE;

-- Undo CREATE_TABLE: message_templates
DROP TABLE IF EXISTS message_templates CASCADE;

-- Undo CREATE_TABLE: employee_meta
DROP TABLE IF EXISTS employee_meta CASCADE;

-- Undo CREATE_TABLE: user_roles
DROP TABLE IF EXISTS user_roles CASCADE;

-- Undo CREATE_TABLE: authentication_logs
DROP TABLE IF EXISTS authentication_logs CASCADE;

-- Undo CREATE_TABLE: money_accounts
DROP TABLE IF EXISTS money_accounts CASCADE;

-- Undo CREATE_TABLE: action_logs
DROP TABLE IF EXISTS action_logs CASCADE;

-- Undo CREATE_TABLE: employee_contacts
DROP TABLE IF EXISTS employee_contacts CASCADE;

-- Undo CREATE_TABLE: chats
DROP TABLE IF EXISTS chats CASCADE;

-- Undo CREATE_TABLE: message_media
DROP TABLE IF EXISTS message_media CASCADE;

-- Undo CREATE_TABLE: messages
DROP TABLE IF EXISTS messages CASCADE;

-- Undo CREATE_TABLE: expenses
DROP TABLE IF EXISTS expenses CASCADE;

-- Undo CREATE_TABLE: permissions
DROP TABLE IF EXISTS permissions CASCADE;

-- Undo CREATE_TABLE: transactions
DROP TABLE IF EXISTS transactions CASCADE;

-- Undo CREATE_TABLE: inventory
DROP TABLE IF EXISTS inventory CASCADE;

-- Undo CREATE_TABLE: employee_positions
DROP TABLE IF EXISTS employee_positions CASCADE;

-- Undo CREATE_TABLE: tabs
DROP TABLE IF EXISTS tabs CASCADE;

-- Undo CREATE_TABLE: counterparty
DROP TABLE IF EXISTS counterparty CASCADE;

-- Undo CREATE_TABLE: prompts
DROP TABLE IF EXISTS prompts CASCADE;

-- Undo CREATE_TABLE: employees
DROP TABLE IF EXISTS employees CASCADE;

-- Undo CREATE_TABLE: uploaded_images
DROP TABLE IF EXISTS uploaded_images CASCADE;

-- Undo CREATE_TABLE: positions
DROP TABLE IF EXISTS positions CASCADE;

-- Undo CREATE_TABLE: role_permissions
DROP TABLE IF EXISTS role_permissions CASCADE;

-- Undo CREATE_TABLE: users
DROP TABLE IF EXISTS users CASCADE;

-- Undo CREATE_TABLE: currencies
DROP TABLE IF EXISTS currencies CASCADE;

-- Undo CREATE_TABLE: expense_categories
DROP TABLE IF EXISTS expense_categories CASCADE;

-- Undo CREATE_TABLE: companies
DROP TABLE IF EXISTS companies CASCADE;

-- Undo CREATE_TABLE: roles
DROP TABLE IF EXISTS roles CASCADE;

-- Undo CREATE_TABLE: counterparty_contacts
DROP TABLE IF EXISTS counterparty_contacts CASCADE;

-- Undo CREATE_TABLE: sessions
DROP TABLE IF EXISTS sessions CASCADE;

-- Undo CREATE_TABLE: payments
DROP TABLE IF EXISTS payments CASCADE;

