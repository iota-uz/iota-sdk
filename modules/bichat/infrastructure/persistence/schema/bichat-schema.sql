CREATE TABLE prompts (
    id varchar(30) PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL,
    description text NOT NULL,
    prompt text NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE dialogues (
    id serial PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label varchar(255) NOT NULL,
    messages json NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

CREATE INDEX dialogues_tenant_id_idx ON dialogues (tenant_id);

CREATE INDEX prompts_tenant_id_idx ON prompts (tenant_id);

