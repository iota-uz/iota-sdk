-- +migrate Up
CREATE TABLE clients (
    id            SERIAL PRIMARY KEY,
    first_name    VARCHAR(255) NOT NULL,
    last_name     VARCHAR(255),
    middle_name   VARCHAR(255),
    phone_number  VARCHAR(255) NOT NULL,
    address       TEXT,
    email         VARCHAR(255),
    hourly_rate   FLOAT,
    date_of_birth DATE,
    gender        VARCHAR(15),
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at    TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE chats (
    id          SERIAL PRIMARY KEY,
    created_at  TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP NOT NULL,
    client_id   INT NOT NULL REFERENCES clients(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    last_message_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE messages (
    id               SERIAL PRIMARY KEY,
    created_at       TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP NOT NULL,
    chat_id          INT NOT NULL REFERENCES chats(id) ON DELETE RESTRICT ON UPDATE CASCADE,
    message          TEXT NOT NULL,
    sender_user_id   INT REFERENCES users(id) ON DELETE SET NULL ON UPDATE CASCADE,
    sender_client_id INT REFERENCES clients(id) ON DELETE SET NULL ON UPDATE CASCADE,
    is_read          BOOLEAN DEFAULT false NOT NULL,
    read_at          TIMESTAMP(3)
);

CREATE TABLE message_media (
    message_id  INT NOT NULL REFERENCES messages(id) ON DELETE CASCADE ON UPDATE CASCADE,
    upload_id   INT NOT NULL REFERENCES uploads(id) ON DELETE CASCADE ON UPDATE CASCADE,
    PRIMARY KEY (message_id, upload_id)
);

CREATE TABLE message_templates (
    id          SERIAL PRIMARY KEY,
    template    TEXT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE INDEX idx_chats_client_id ON chats (client_id);

CREATE INDEX idx_messages_chat_id ON messages (chat_id);
CREATE INDEX idx_messages_sender_user_id ON messages (sender_user_id);
CREATE INDEX idx_messages_sender_client_id ON messages (sender_client_id);

CREATE INDEX idx_customers_first_name ON clients (first_name);
CREATE INDEX idx_customers_last_name ON clients (last_name);
CREATE INDEX idx_customers_phone_number ON clients (phone_number);

-- +migrate Down
DROP TABLE IF EXISTS message_templates;
DROP TABLE IF EXISTS message_media;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chats;
DROP TABLE IF EXISTS customers;

