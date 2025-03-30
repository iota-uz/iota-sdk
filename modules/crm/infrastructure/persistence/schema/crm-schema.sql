CREATE TABLE clients (
    id serial PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
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
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, phone_number),
    UNIQUE (tenant_id, email)
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
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    created_at timestamp(3) DEFAULT now() NOT NULL,
    client_id int NOT NULL REFERENCES clients (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    last_message_at timestamp(3) DEFAULT now()
);

CREATE TABLE messages (
    id serial PRIMARY KEY,
    created_at timestamp(3) DEFAULT now() NOT NULL,
    chat_id int NOT NULL REFERENCES chats (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    message text NOT NULL,
    source varchar(20) NOT NULL,
    sender_user_id int REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    sender_client_id int REFERENCES clients (id) ON DELETE SET NULL ON UPDATE CASCADE,
    is_read boolean DEFAULT FALSE NOT NULL,
    read_at timestamp(3)
);

CREATE TABLE message_media (
    message_id int NOT NULL REFERENCES messages (id) ON DELETE CASCADE ON UPDATE CASCADE,
    upload_id int NOT NULL REFERENCES uploads (id) ON DELETE CASCADE ON UPDATE CASCADE,
    PRIMARY KEY (message_id, upload_id)
);

CREATE TABLE message_templates (
    id serial PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
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

CREATE INDEX idx_clients_tenant_id ON clients (tenant_id);

CREATE INDEX idx_client_contacts_client_id ON client_contacts (client_id);

CREATE INDEX idx_chats_tenant_id ON chats (tenant_id);

CREATE INDEX idx_message_templates_tenant_id ON message_templates (tenant_id);
