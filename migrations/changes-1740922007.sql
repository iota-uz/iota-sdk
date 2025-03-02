-- +migrate Up

-- Change CREATE_TABLE: passports
CREATE TABLE passports (
	id                    UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	first_name            VARCHAR(255),
	last_name             VARCHAR(255),
	middle_name           VARCHAR(255),
	gender                VARCHAR(10),
	birth_date            DATE,
	birth_place           VARCHAR(255),
	nationality           VARCHAR(100),
	passport_type         VARCHAR(20),
	passport_number       VARCHAR(20) UNIQUE,
	series                VARCHAR(20),
	issuing_country       VARCHAR(100),
	issued_at             DATE,
	issued_by             VARCHAR(255),
	expires_at            DATE,
	machine_readable_zone VARCHAR(88),
	biometric_data        JSONB,
	signature_image       BYTEA,
	remarks               TEXT,
	created_at            TIMESTAMPTZ DEFAULT now(),
	updated_at            TIMESTAMPTZ DEFAULT now()
);

-- Change ADD_COLUMN: phone
ALTER TABLE users ADD COLUMN phone VARCHAR(255) UNIQUE;

-- Change ADD_COLUMN: passport_id
ALTER TABLE clients ADD COLUMN passport_id UUID REFERENCES passports (id) ON DELETE SET NULL ON UPDATE CASCADE;

-- Change ADD_COLUMN: pin
ALTER TABLE clients ADD COLUMN pin VARCHAR(128);

-- Change CREATE_TABLE: client_contacts
CREATE TABLE client_contacts (
	id            SERIAL8 PRIMARY KEY,
	client_id     INT8 NOT NULL REFERENCES clients (id) ON DELETE CASCADE ON UPDATE CASCADE,
	contact_type  VARCHAR(20) NOT NULL,
	contact_value VARCHAR(255) NOT NULL,
	created_at    TIMESTAMPTZ DEFAULT now(),
	updated_at    TIMESTAMPTZ DEFAULT now()
);

-- Change ADD_COLUMN: source
ALTER TABLE messages ADD COLUMN source VARCHAR(20) NOT NULL;

-- Change CREATE_INDEX: idx_clients_phone_number
CREATE INDEX idx_clients_phone_number ON clients (phone_number);

-- Change CREATE_INDEX: idx_clients_first_name
CREATE INDEX idx_clients_first_name ON clients (first_name);

-- Change CREATE_INDEX: idx_clients_last_name
CREATE INDEX idx_clients_last_name ON clients (last_name);

-- Change CREATE_INDEX: idx_client_contacts_client_id
CREATE INDEX idx_client_contacts_client_id ON client_contacts (client_id);

-- Change CREATE_INDEX: idx_clients_email
CREATE INDEX idx_clients_email ON clients (email);


-- +migrate Down

-- Undo CREATE_INDEX: clients@idx_clients_email
DROP INDEX clients@idx_clients_email;

-- Undo CREATE_INDEX: client_contacts@idx_client_contacts_client_id
DROP INDEX client_contacts@idx_client_contacts_client_id;

-- Undo CREATE_INDEX: clients@idx_clients_last_name
DROP INDEX clients@idx_clients_last_name;

-- Undo CREATE_INDEX: clients@idx_clients_first_name
DROP INDEX clients@idx_clients_first_name;

-- Undo CREATE_INDEX: clients@idx_clients_phone_number
DROP INDEX clients@idx_clients_phone_number;

-- Undo ADD_COLUMN: source
ALTER TABLE messages DROP COLUMN IF EXISTS source;

-- Undo CREATE_TABLE: client_contacts
DROP TABLE IF EXISTS client_contacts CASCADE;

-- Undo ADD_COLUMN: pin
ALTER TABLE clients DROP COLUMN IF EXISTS pin;

-- Undo ADD_COLUMN: passport_id
ALTER TABLE clients DROP COLUMN IF EXISTS passport_id;

-- Undo ADD_COLUMN: phone
ALTER TABLE users DROP COLUMN IF EXISTS phone;

-- Undo CREATE_TABLE: passports
DROP TABLE IF EXISTS passports CASCADE;

