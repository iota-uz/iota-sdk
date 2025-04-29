-- +migrate Up

-- Change CREATE_TABLE: chat_members
CREATE TABLE chat_members (
	id                UUID DEFAULT gen_random_uuid() PRIMARY KEY,
	chat_id           INT8 NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
	user_id           INT8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
	client_id         INT8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE,
	client_contact_id INT8 UNIQUE REFERENCES client_contacts (id) ON DELETE SET NULL ON UPDATE CASCADE,
	transport         VARCHAR(20) NOT NULL,
	transport_meta    JSONB
);

-- Change ADD_COLUMN: sender_id
ALTER TABLE messages ADD COLUMN sender_id UUID REFERENCES chat_members (id) ON DELETE RESTRICT ON UPDATE CASCADE;

ALTER TABLE messages DROP COLUMN IF EXISTS sender_user_id;

ALTER TABLE messages DROP COLUMN IF EXISTS sender_client_id;

ALTER TABLE messages DROP COLUMN IF EXISTS source;


-- +migrate Down

ALTER TABLE messages ADD COLUMN source VARCHAR(20) NOT NULL;

ALTER TABLE messages ADD COLUMN sender_client_id INT8 REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE;

ALTER TABLE messages ADD COLUMN sender_user_id INT8 REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE;

-- Undo ADD_COLUMN: sender_id
ALTER TABLE messages DROP COLUMN IF EXISTS sender_id;

-- Undo CREATE_TABLE: chat_members
DROP TABLE IF EXISTS chat_members CASCADE;

