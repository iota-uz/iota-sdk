-- +migrate Up
-- Change CREATE_TABLE: chat_members
CREATE TABLE chat_members (
    id uuid DEFAULT gen_random_uuid () PRIMARY KEY,
    chat_id int8 NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id int8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    client_id int8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE,
    client_contact_id int8 UNIQUE REFERENCES client_contacts (id) ON DELETE SET NULL ON UPDATE CASCADE,
    transport varchar(20) NOT NULL,
    transport_meta jsonb,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    created_at timestamp(3) DEFAULT now() NOT NULL,
    updated_at timestamp(3) DEFAULT now() NOT NULL
);

-- Change ADD_COLUMN: sender_id
ALTER TABLE messages
    ADD COLUMN sender_id UUID NOT NULL REFERENCES chat_members (id) ON DELETE RESTRICT ON UPDATE CASCADE;

-- Change ADD_COLUMN: sent_at
ALTER TABLE messages
    ADD COLUMN sent_at TIMESTAMP(3);

ALTER TABLE messages
    DROP COLUMN IF EXISTS sender_user_id;

ALTER TABLE messages
    DROP COLUMN IF EXISTS sender_client_id;

ALTER TABLE messages
    DROP COLUMN IF EXISTS is_read;

ALTER TABLE messages
    DROP COLUMN IF EXISTS source;

-- +migrate Down
ALTER TABLE messages
    ADD COLUMN source VARCHAR(20) NOT NULL;

ALTER TABLE messages
    ADD COLUMN is_read BOOL DEFAULT FALSE NOT NULL;

ALTER TABLE messages
    ADD COLUMN sender_client_id INT8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE messages
    ADD COLUMN sender_user_id INT8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE;

-- Undo ADD_COLUMN: sent_at
ALTER TABLE messages
    DROP COLUMN IF EXISTS sent_at;

-- Undo ADD_COLUMN: sender_id
ALTER TABLE messages
    DROP COLUMN IF EXISTS sender_id;

-- Undo CREATE_TABLE: chat_members
DROP TABLE IF EXISTS chat_members CASCADE;

