CREATE TABLE prompts (
    id varchar(30) PRIMARY KEY,
    title varchar(255) NOT NULL,
    description text NOT NULL,
    prompt text NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE dialogues (
    id serial PRIMARY KEY,
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label varchar(255) NOT NULL,
    messages json NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

