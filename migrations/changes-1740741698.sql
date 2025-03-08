-- +migrate Up

-- Change CREATE_TABLE: uploads
CREATE TABLE uploads (
	id         SERIAL8 PRIMARY KEY,
	name       VARCHAR(255) NOT NULL,
	hash       VARCHAR(255) NOT NULL UNIQUE,
	path       VARCHAR(1024) DEFAULT '' NOT NULL,
	size       INT8 DEFAULT 0 NOT NULL,
	mimetype   VARCHAR(255) NOT NULL,
	type       VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: clients
CREATE TABLE clients (
	id            SERIAL8 PRIMARY KEY,
	first_name    VARCHAR(255) NOT NULL,
	last_name     VARCHAR(255),
	middle_name   VARCHAR(255),
	phone_number  VARCHAR(255) NOT NULL,
	address       TEXT,
	email         VARCHAR(255),
	hourly_rate   FLOAT8,
	date_of_birth DATE,
	gender        VARCHAR(15),
	created_at    TIMESTAMPTZ DEFAULT now(),
	updated_at    TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: counterparty
CREATE TABLE counterparty (
	id            SERIAL8 PRIMARY KEY,
	tin           VARCHAR(20),
	name          VARCHAR(255) NOT NULL,
	type          VARCHAR(255) NOT NULL,
	legal_type    VARCHAR(255) NOT NULL,
	legal_address VARCHAR(255),
	created_at    TIMESTAMPTZ DEFAULT now(),
	updated_at    TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: warehouse_units
CREATE TABLE warehouse_units (
	id          SERIAL8 PRIMARY KEY,
	title       VARCHAR(255) NOT NULL,
	short_title VARCHAR(255) NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: positions
CREATE TABLE positions (
	id          SERIAL8 PRIMARY KEY,
	name        VARCHAR(255) NOT NULL,
	description TEXT,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: permissions
CREATE TABLE permissions (
	id          UUID DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
	name        VARCHAR(255) NOT NULL UNIQUE,
	resource    VARCHAR(255) NOT NULL,
	action      VARCHAR(255) NOT NULL,
	modifier    VARCHAR(255) NOT NULL,
	description TEXT
);

-- Change CREATE_TABLE: roles
CREATE TABLE roles (
	id          SERIAL8 PRIMARY KEY,
	name        VARCHAR(255) NOT NULL UNIQUE,
	description TEXT,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: warehouse_orders
CREATE TABLE warehouse_orders (
	id         SERIAL8 PRIMARY KEY,
	type       VARCHAR(255) NOT NULL,
	status     VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: prompts
CREATE TABLE prompts (
	id          VARCHAR(30) PRIMARY KEY,
	title       VARCHAR(255) NOT NULL,
	description TEXT NOT NULL,
	prompt      TEXT NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: currencies
CREATE TABLE currencies (
	code       VARCHAR(3) NOT NULL PRIMARY KEY,
	name       VARCHAR(255) NOT NULL,
	symbol     VARCHAR(3) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: message_templates
CREATE TABLE message_templates (id SERIAL8 PRIMARY KEY, template TEXT NOT NULL, created_at TIMESTAMPTZ DEFAULT now());

-- Change CREATE_TABLE: inventory
CREATE TABLE inventory (
	id          SERIAL8 PRIMARY KEY,
	name        VARCHAR(255) NOT NULL,
	description TEXT,
	currency_id VARCHAR(3) REFERENCES currencies (code) ON DELETE SET NULL,
	price       DECIMAL(9,2) NOT NULL,
	quantity    INT8 NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: warehouse_positions
CREATE TABLE warehouse_positions (
	id          SERIAL8 PRIMARY KEY,
	title       VARCHAR(255) NOT NULL,
	barcode     VARCHAR(255) NOT NULL UNIQUE,
	description TEXT,
	unit_id     INT8 REFERENCES warehouse_units (id) ON DELETE SET NULL,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: employees
CREATE TABLE employees (
	id                 SERIAL8 PRIMARY KEY,
	first_name         VARCHAR(255) NOT NULL,
	last_name          VARCHAR(255) NOT NULL,
	middle_name        VARCHAR(255),
	email              VARCHAR(255) NOT NULL UNIQUE,
	phone              VARCHAR(255),
	salary             DECIMAL(9,2) NOT NULL,
	salary_currency_id VARCHAR(3) REFERENCES currencies (code) ON DELETE SET NULL,
	hourly_rate        DECIMAL(9,2) NOT NULL,
	coefficient        FLOAT8 NOT NULL,
	avatar_id          INT8 REFERENCES uploads (id) ON DELETE SET NULL,
	created_at         TIMESTAMPTZ DEFAULT now(),
	updated_at         TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: warehouse_position_images
CREATE TABLE warehouse_position_images (
	warehouse_position_id INT8 NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
	upload_id             INT8 NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
	PRIMARY KEY (upload_id, warehouse_position_id)
);

-- Change CREATE_TABLE: users
CREATE TABLE users (
	id          SERIAL8 PRIMARY KEY,
	first_name  VARCHAR(255) NOT NULL,
	last_name   VARCHAR(255) NOT NULL,
	middle_name VARCHAR(255),
	email       VARCHAR(255) NOT NULL UNIQUE,
	password    VARCHAR(255),
	ui_language VARCHAR(3) NOT NULL,
	avatar_id   INT8 REFERENCES uploads (id) ON DELETE SET NULL,
	last_login  TIMESTAMP NULL,
	last_ip     VARCHAR(255) NULL,
	last_action TIMESTAMPTZ NULL,
	created_at  TIMESTAMPTZ DEFAULT now() NOT NULL,
	updated_at  TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- Change CREATE_TABLE: role_permissions
CREATE TABLE role_permissions (
	role_id       INT8 NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	permission_id UUID NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
	PRIMARY KEY (role_id, permission_id)
);

-- Change CREATE_TABLE: action_logs
CREATE TABLE action_logs (
	id         SERIAL8 PRIMARY KEY,
	method     VARCHAR(255) NOT NULL,
	path       VARCHAR(255) NOT NULL,
	user_id    INT8 REFERENCES users (id) ON DELETE SET NULL,
	after      JSONB,
	before     JSONB,
	user_agent VARCHAR(255) NOT NULL,
	ip         VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: employee_contacts
CREATE TABLE employee_contacts (
	id          SERIAL8 PRIMARY KEY,
	employee_id INT8 NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
	type        VARCHAR(255) NOT NULL,
	value       VARCHAR(255) NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: companies
CREATE TABLE companies (
	id         SERIAL8 PRIMARY KEY,
	name       VARCHAR(255) NOT NULL,
	about      TEXT,
	address    VARCHAR(255),
	phone      VARCHAR(255),
	logo_id    INT8 REFERENCES uploads (id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: employee_meta
CREATE TABLE employee_meta (
	employee_id        INT8 NOT NULL PRIMARY KEY REFERENCES employees (id) ON DELETE CASCADE,
	primary_language   VARCHAR(255),
	secondary_language VARCHAR(255),
	tin                VARCHAR(255),
	pin                VARCHAR(255),
	notes              TEXT,
	birth_date         DATE,
	hire_date          DATE,
	resignation_date   DATE
);

-- Change CREATE_TABLE: warehouse_products
CREATE TABLE warehouse_products (
	id          SERIAL8 PRIMARY KEY,
	position_id INT8 NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
	rfid        VARCHAR(255) NULL UNIQUE,
	status      VARCHAR(255) NOT NULL,
	created_at  TIMESTAMPTZ DEFAULT now(),
	updated_at  TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: authentication_logs
CREATE TABLE authentication_logs (
	id         SERIAL8 PRIMARY KEY,
	user_id    INT8 NOT NULL CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
	ip         VARCHAR(255) NOT NULL,
	user_agent VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- Change CREATE_TABLE: uploaded_images
CREATE TABLE uploaded_images (
	id         SERIAL8 PRIMARY KEY,
	upload_id  INT8 NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
	type       VARCHAR(255) NOT NULL,
	size       FLOAT8 NOT NULL,
	width      INT8 NOT NULL,
	height     INT8 NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: inventory_checks
CREATE TABLE inventory_checks (
	id             SERIAL8 PRIMARY KEY,
	status         VARCHAR(255) NOT NULL,
	name           VARCHAR(255) NOT NULL,
	type           VARCHAR(255) NOT NULL,
	created_at     TIMESTAMPTZ DEFAULT now(),
	finished_at    TIMESTAMPTZ,
	created_by_id  INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	finished_by_id INT8 REFERENCES users (id) ON DELETE CASCADE
);

-- Change CREATE_TABLE: user_roles
CREATE TABLE user_roles (
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	role_id    INT8 NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
	created_at TIMESTAMPTZ DEFAULT now(),
	PRIMARY KEY (user_id, role_id)
);

-- Change CREATE_TABLE: sessions
CREATE TABLE sessions (
	token      VARCHAR(255) NOT NULL PRIMARY KEY,
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	ip         VARCHAR(255) NOT NULL,
	user_agent VARCHAR(255) NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

-- Change CREATE_TABLE: tabs
CREATE TABLE tabs (
	id         SERIAL8 PRIMARY KEY,
	href       VARCHAR(255) NOT NULL,
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	"position" INT8 DEFAULT 0 NOT NULL,
	UNIQUE (href, user_id)
);

-- Change CREATE_TABLE: counterparty_contacts
CREATE TABLE counterparty_contacts (
	id              SERIAL8 PRIMARY KEY,
	counterparty_id INT8 NOT NULL REFERENCES counterparty (id) ON DELETE CASCADE,
	first_name      VARCHAR(255) NOT NULL,
	last_name       VARCHAR(255) NOT NULL,
	middle_name     VARCHAR(255) NULL,
	email           VARCHAR(255),
	phone           VARCHAR(255),
	created_at      TIMESTAMPTZ DEFAULT now(),
	updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: money_accounts
CREATE TABLE money_accounts (
	id                  SERIAL8 PRIMARY KEY,
	name                VARCHAR(255) NOT NULL,
	account_number      VARCHAR(255) NOT NULL,
	description         TEXT,
	balance             DECIMAL(9,2) NOT NULL,
	balance_currency_id VARCHAR(3) NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
	created_at          TIMESTAMPTZ DEFAULT now(),
	updated_at          TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: dialogues
CREATE TABLE dialogues (
	id         SERIAL8 PRIMARY KEY,
	user_id    INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	label      VARCHAR(255) NOT NULL,
	messages   JSONB NOT NULL,
	created_at TIMESTAMPTZ DEFAULT now(),
	updated_at TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: expense_categories
CREATE TABLE expense_categories (
	id                 SERIAL8 PRIMARY KEY,
	name               VARCHAR(255) NOT NULL,
	description        TEXT,
	amount             DECIMAL(9,2) NOT NULL,
	amount_currency_id VARCHAR(3) NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
	created_at         TIMESTAMPTZ DEFAULT now(),
	updated_at         TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: chats
CREATE TABLE chats (
	id              SERIAL8 PRIMARY KEY,
	created_at      TIMESTAMP(3) DEFAULT now() NOT NULL,
	client_id       INT8 NOT NULL REFERENCES clients (id) ON DELETE RESTRICT ON UPDATE CASCADE,
	last_message_at TIMESTAMP(3) DEFAULT now()
);

-- Change CREATE_TABLE: employee_positions
CREATE TABLE employee_positions (
	employee_id INT8 NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
	position_id INT8 NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
	PRIMARY KEY (employee_id, position_id)
);

-- Change CREATE_TABLE: inventory_check_results
CREATE TABLE inventory_check_results (
	id                 SERIAL8 PRIMARY KEY,
	inventory_check_id INT8 NOT NULL REFERENCES inventory_checks (id) ON DELETE CASCADE,
	position_id        INT8 NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
	expected_quantity  INT8 NOT NULL,
	actual_quantity    INT8 NOT NULL,
	difference         INT8 NOT NULL,
	created_at         TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: warehouse_order_items
CREATE TABLE warehouse_order_items (
	warehouse_order_id   INT8 NOT NULL REFERENCES warehouse_orders (id) ON DELETE CASCADE,
	warehouse_product_id INT8 NOT NULL REFERENCES warehouse_products (id) ON DELETE CASCADE,
	PRIMARY KEY (warehouse_order_id, warehouse_product_id)
);

-- Change CREATE_TABLE: messages
CREATE TABLE messages (
	id               SERIAL8 PRIMARY KEY,
	created_at       TIMESTAMP(3) DEFAULT now() NOT NULL,
	chat_id          INT8 NOT NULL REFERENCES chats (id) ON DELETE RESTRICT ON UPDATE CASCADE,
	message          TEXT NOT NULL,
	sender_user_id   INT8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
	sender_client_id INT8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE,
	is_read          BOOL DEFAULT false NOT NULL,
	read_at          TIMESTAMP(3)
);

-- Change CREATE_TABLE: transactions
CREATE TABLE transactions (
	id                     SERIAL8 PRIMARY KEY,
	amount                 DECIMAL(9,2) NOT NULL,
	origin_account_id      INT8 REFERENCES money_accounts (id) ON DELETE RESTRICT,
	destination_account_id INT8 REFERENCES money_accounts (id) ON DELETE RESTRICT,
	transaction_date       DATE DEFAULT now()::DATE NOT NULL,
	accounting_period      DATE DEFAULT now()::DATE NOT NULL,
	transaction_type       VARCHAR(255) NOT NULL,
	comment                TEXT,
	created_at             TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: expenses
CREATE TABLE expenses (
	id             SERIAL8 PRIMARY KEY,
	transaction_id INT8 NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
	category_id    INT8 NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
	created_at     TIMESTAMPTZ DEFAULT now(),
	updated_at     TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_TABLE: message_media
CREATE TABLE message_media (
	message_id INT8 NOT NULL REFERENCES messages (id) ON DELETE CASCADE ON UPDATE CASCADE,
	upload_id  INT8 NOT NULL REFERENCES uploads (id) ON DELETE CASCADE ON UPDATE CASCADE,
	PRIMARY KEY (message_id, upload_id)
);

-- Change CREATE_TABLE: payments
CREATE TABLE payments (
	id              SERIAL8 PRIMARY KEY,
	transaction_id  INT8 NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
	counterparty_id INT8 NOT NULL REFERENCES counterparty (id) ON DELETE RESTRICT,
	created_at      TIMESTAMPTZ DEFAULT now(),
	updated_at      TIMESTAMPTZ DEFAULT now()
);

-- Change CREATE_INDEX: idx_messages_chat_id
CREATE INDEX idx_messages_chat_id ON messages (chat_id);

-- Change CREATE_INDEX: uploaded_images_upload_id_idx
CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

-- Change CREATE_INDEX: idx_messages_sender_user_id
CREATE INDEX idx_messages_sender_user_id ON messages (sender_user_id);

-- Change CREATE_INDEX: idx_chats_client_id
CREATE INDEX idx_chats_client_id ON chats (client_id);

-- Change CREATE_INDEX: authentication_logs_user_id_idx
CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);

-- Change CREATE_INDEX: role_permissions_role_id_idx
CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);

-- Change CREATE_INDEX: idx_messages_sender_client_id
CREATE INDEX idx_messages_sender_client_id ON messages (sender_client_id);

-- Change CREATE_INDEX: idx_customers_phone_number
CREATE INDEX idx_customers_phone_number ON clients (phone_number);

-- Change CREATE_INDEX: action_log_user_id_idx
CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

-- Change CREATE_INDEX: users_first_name_idx
CREATE INDEX users_first_name_idx ON users (first_name);

-- Change CREATE_INDEX: inventory_currency_id_idx
CREATE INDEX inventory_currency_id_idx ON inventory (currency_id);

-- Change CREATE_INDEX: transactions_origin_account_id_idx
CREATE INDEX transactions_origin_account_id_idx ON transactions (origin_account_id);

-- Change CREATE_INDEX: counterparty_contacts_counterparty_id_idx
CREATE INDEX counterparty_contacts_counterparty_id_idx ON counterparty_contacts (counterparty_id);

-- Change CREATE_INDEX: role_permissions_permission_id_idx
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

-- Change CREATE_INDEX: expenses_category_id_idx
CREATE INDEX expenses_category_id_idx ON expenses (category_id);

-- Change CREATE_INDEX: counterparty_tin_idx
CREATE INDEX counterparty_tin_idx ON counterparty (tin);

-- Change CREATE_INDEX: dialogues_user_id_idx
CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

-- Change CREATE_INDEX: idx_customers_last_name
CREATE INDEX idx_customers_last_name ON clients (last_name);

-- Change CREATE_INDEX: users_last_name_idx
CREATE INDEX users_last_name_idx ON users (last_name);

-- Change CREATE_INDEX: expenses_transaction_id_idx
CREATE INDEX expenses_transaction_id_idx ON expenses (transaction_id);

-- Change CREATE_INDEX: payments_counterparty_id_idx
CREATE INDEX payments_counterparty_id_idx ON payments (counterparty_id);

-- Change CREATE_INDEX: transactions_destination_account_id_idx
CREATE INDEX transactions_destination_account_id_idx ON transactions (destination_account_id);

-- Change CREATE_INDEX: money_accounts_balance_currency_id_idx
CREATE INDEX money_accounts_balance_currency_id_idx ON money_accounts (balance_currency_id);

-- Change CREATE_INDEX: payments_transaction_id_idx
CREATE INDEX payments_transaction_id_idx ON payments (transaction_id);

-- Change CREATE_INDEX: idx_customers_first_name
CREATE INDEX idx_customers_first_name ON clients (first_name);

-- Change CREATE_INDEX: sessions_expires_at_idx
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Change CREATE_INDEX: authentication_logs_created_at_idx
CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

-- Change CREATE_INDEX: sessions_user_id_idx
CREATE INDEX sessions_user_id_idx ON sessions (user_id);


-- +migrate Down

-- Undo CREATE_INDEX: sessions_user_id_idx
DROP INDEX sessions_user_id_idx;

-- Undo CREATE_INDEX: authentication_logs_created_at_idx
DROP INDEX authentication_logs_created_at_idx;

-- Undo CREATE_INDEX: sessions_expires_at_idx
DROP INDEX sessions_expires_at_idx;

-- Undo CREATE_INDEX: idx_customers_first_name
DROP INDEX idx_customers_first_name;

-- Undo CREATE_INDEX: payments_transaction_id_idx
DROP INDEX payments_transaction_id_idx;

-- Undo CREATE_INDEX: money_accounts_balance_currency_id_idx
DROP INDEX money_accounts_balance_currency_id_idx;

-- Undo CREATE_INDEX: transactions_destination_account_id_idx
DROP INDEX transactions_destination_account_id_idx;

-- Undo CREATE_INDEX: payments_counterparty_id_idx
DROP INDEX payments_counterparty_id_idx;

-- Undo CREATE_INDEX: expenses_transaction_id_idx
DROP INDEX expenses_transaction_id_idx;

-- Undo CREATE_INDEX: users_last_name_idx
DROP INDEX users_last_name_idx;

-- Undo CREATE_INDEX: idx_customers_last_name
DROP INDEX idx_customers_last_name;

-- Undo CREATE_INDEX: dialogues_user_id_idx
DROP INDEX dialogues_user_id_idx;

-- Undo CREATE_INDEX: counterparty_tin_idx
DROP INDEX counterparty_tin_idx;

-- Undo CREATE_INDEX: expenses_category_id_idx
DROP INDEX expenses_category_id_idx;

-- Undo CREATE_INDEX: role_permissions_permission_id_idx
DROP INDEX role_permissions_permission_id_idx;

-- Undo CREATE_INDEX: counterparty_contacts_counterparty_id_idx
DROP INDEX counterparty_contacts_counterparty_id_idx;

-- Undo CREATE_INDEX: transactions_origin_account_id_idx
DROP INDEX transactions_origin_account_id_idx;

-- Undo CREATE_INDEX: inventory_currency_id_idx
DROP INDEX inventory_currency_id_idx;

-- Undo CREATE_INDEX: users_first_name_idx
DROP INDEX users_first_name_idx;

-- Undo CREATE_INDEX: action_log_user_id_idx
DROP INDEX action_log_user_id_idx;

-- Undo CREATE_INDEX: idx_customers_phone_number
DROP INDEX idx_customers_phone_number;

-- Undo CREATE_INDEX: idx_messages_sender_client_id
DROP INDEX idx_messages_sender_client_id;

-- Undo CREATE_INDEX: role_permissions_role_id_idx
DROP INDEX role_permissions_role_id_idx;

-- Undo CREATE_INDEX: authentication_logs_user_id_idx
DROP INDEX authentication_logs_user_id_idx;

-- Undo CREATE_INDEX: idx_chats_client_id
DROP INDEX idx_chats_client_id;

-- Undo CREATE_INDEX: idx_messages_sender_user_id
DROP INDEX idx_messages_sender_user_id;

-- Undo CREATE_INDEX: uploaded_images_upload_id_idx
DROP INDEX uploaded_images_upload_id_idx;

-- Undo CREATE_INDEX: idx_messages_chat_id
DROP INDEX idx_messages_chat_id;

-- Undo CREATE_TABLE: payments
DROP TABLE IF EXISTS payments CASCADE;

-- Undo CREATE_TABLE: message_media
DROP TABLE IF EXISTS message_media CASCADE;

-- Undo CREATE_TABLE: expenses
DROP TABLE IF EXISTS expenses CASCADE;

-- Undo CREATE_TABLE: transactions
DROP TABLE IF EXISTS transactions CASCADE;

-- Undo CREATE_TABLE: messages
DROP TABLE IF EXISTS messages CASCADE;

-- Undo CREATE_TABLE: warehouse_order_items
DROP TABLE IF EXISTS warehouse_order_items CASCADE;

-- Undo CREATE_TABLE: inventory_check_results
DROP TABLE IF EXISTS inventory_check_results CASCADE;

-- Undo CREATE_TABLE: employee_positions
DROP TABLE IF EXISTS employee_positions CASCADE;

-- Undo CREATE_TABLE: chats
DROP TABLE IF EXISTS chats CASCADE;

-- Undo CREATE_TABLE: expense_categories
DROP TABLE IF EXISTS expense_categories CASCADE;

-- Undo CREATE_TABLE: dialogues
DROP TABLE IF EXISTS dialogues CASCADE;

-- Undo CREATE_TABLE: money_accounts
DROP TABLE IF EXISTS money_accounts CASCADE;

-- Undo CREATE_TABLE: counterparty_contacts
DROP TABLE IF EXISTS counterparty_contacts CASCADE;

-- Undo CREATE_TABLE: tabs
DROP TABLE IF EXISTS tabs CASCADE;

-- Undo CREATE_TABLE: sessions
DROP TABLE IF EXISTS sessions CASCADE;

-- Undo CREATE_TABLE: user_roles
DROP TABLE IF EXISTS user_roles CASCADE;

-- Undo CREATE_TABLE: inventory_checks
DROP TABLE IF EXISTS inventory_checks CASCADE;

-- Undo CREATE_TABLE: uploaded_images
DROP TABLE IF EXISTS uploaded_images CASCADE;

-- Undo CREATE_TABLE: authentication_logs
DROP TABLE IF EXISTS authentication_logs CASCADE;

-- Undo CREATE_TABLE: warehouse_products
DROP TABLE IF EXISTS warehouse_products CASCADE;

-- Undo CREATE_TABLE: employee_meta
DROP TABLE IF EXISTS employee_meta CASCADE;

-- Undo CREATE_TABLE: companies
DROP TABLE IF EXISTS companies CASCADE;

-- Undo CREATE_TABLE: employee_contacts
DROP TABLE IF EXISTS employee_contacts CASCADE;

-- Undo CREATE_TABLE: action_logs
DROP TABLE IF EXISTS action_logs CASCADE;

-- Undo CREATE_TABLE: role_permissions
DROP TABLE IF EXISTS role_permissions CASCADE;

-- Undo CREATE_TABLE: users
DROP TABLE IF EXISTS users CASCADE;

-- Undo CREATE_TABLE: warehouse_position_images
DROP TABLE IF EXISTS warehouse_position_images CASCADE;

-- Undo CREATE_TABLE: employees
DROP TABLE IF EXISTS employees CASCADE;

-- Undo CREATE_TABLE: warehouse_positions
DROP TABLE IF EXISTS warehouse_positions CASCADE;

-- Undo CREATE_TABLE: inventory
DROP TABLE IF EXISTS inventory CASCADE;

-- Undo CREATE_TABLE: message_templates
DROP TABLE IF EXISTS message_templates CASCADE;

-- Undo CREATE_TABLE: currencies
DROP TABLE IF EXISTS currencies CASCADE;

-- Undo CREATE_TABLE: prompts
DROP TABLE IF EXISTS prompts CASCADE;

-- Undo CREATE_TABLE: warehouse_orders
DROP TABLE IF EXISTS warehouse_orders CASCADE;

-- Undo CREATE_TABLE: roles
DROP TABLE IF EXISTS roles CASCADE;

-- Undo CREATE_TABLE: permissions
DROP TABLE IF EXISTS permissions CASCADE;

-- Undo CREATE_TABLE: positions
DROP TABLE IF EXISTS positions CASCADE;

-- Undo CREATE_TABLE: warehouse_units
DROP TABLE IF EXISTS warehouse_units CASCADE;

-- Undo CREATE_TABLE: counterparty
DROP TABLE IF EXISTS counterparty CASCADE;

-- Undo CREATE_TABLE: clients
DROP TABLE IF EXISTS clients CASCADE;

-- Undo CREATE_TABLE: uploads
DROP TABLE IF EXISTS uploads CASCADE;

