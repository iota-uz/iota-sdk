CREATE TABLE clients (
    id serial PRIMARY KEY,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    middle_name varchar(255),
    phone_number varchar(255),
    address text,
    email varchar(255),
    date_of_birth date,
    gender varchar(15),
    passport_id uuid REFERENCES passports (id) ON DELETE SET NULL ON UPDATE CASCADE,
    pin varchar(128), -- Personal Identification Number
    comments text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE client_contacts (
    id serial PRIMARY KEY,
    client_id int NOT NULL REFERENCES clients (id) ON DELETE CASCADE ON UPDATE CASCADE,
    contact_type varchar(20) NOT NULL, -- telegram, whatsapp, viber, phone, email
    contact_value varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE chats (
    id serial PRIMARY KEY,
    client_id int NOT NULL UNIQUE REFERENCES clients (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    last_message_at timestamp(3) DEFAULT now(),
    created_at timestamp(3) DEFAULT now() NOT NULL
);

CREATE TABLE chat_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    chat_id int NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    -- Whether user_id is not client_id, both can not be set
    user_id int REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    client_id int REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE,
    client_contact_id int UNIQUE REFERENCES client_contacts (id) ON DELETE SET NULL ON UPDATE CASCADE,
    transport varchar(20) NOT NULL,
    transport_meta jsonb,
    created_at timestamp(3) DEFAULT now() NOT NULL,
    updated_at timestamp(3) DEFAULT now() NOT NULL
);

CREATE TABLE messages (
    id serial PRIMARY KEY,
    created_at timestamp(3) DEFAULT now() NOT NULL,
    chat_id int NOT NULL REFERENCES chats (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    sender_id uuid NOT NULL REFERENCES chat_members (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    message text NOT NULL,
    sent_at timestamp(3),
    read_at timestamp(3)
);

CREATE TABLE message_media (
    message_id int NOT NULL REFERENCES messages (id) ON DELETE CASCADE ON UPDATE CASCADE,
    upload_id int NOT NULL REFERENCES uploads (id) ON DELETE CASCADE ON UPDATE CASCADE,
    PRIMARY KEY (message_id, upload_id)
);

CREATE TABLE message_templates (
    id serial PRIMARY KEY,
    template TEXT NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE INDEX idx_chats_client_id ON chats (client_id);

CREATE INDEX idx_messages_chat_id ON messages (chat_id);

CREATE INDEX idx_messages_sender_user_id ON messages (sender_user_id);

CREATE INDEX idx_messages_sender_client_id ON messages (sender_client_id);

CREATE INDEX idx_clients_first_name ON clients (first_name);

CREATE INDEX idx_clients_last_name ON clients (last_name);

CREATE INDEX idx_clients_phone_number ON clients (phone_number);

CREATE INDEX idx_clients_email ON clients (email);

CREATE INDEX idx_client_contacts_client_id ON client_contacts (client_id);

