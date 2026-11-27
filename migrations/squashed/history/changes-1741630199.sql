-- +migrate Up
-- Change CREATE_TABLE: user_groups
CREATE TABLE user_groups (
    id uuid DEFAULT gen_random_uuid () PRIMARY KEY,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp DEFAULT now(),
    updated_at timestamp DEFAULT now(),
    CONSTRAINT user_groups_name_key UNIQUE (name)
);

-- Change CREATE_TABLE: group_roles
CREATE TABLE group_roles (
    group_id uuid REFERENCES user_groups (id) ON DELETE CASCADE,
    role_id int8 REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamp DEFAULT now(),
    PRIMARY KEY (group_id, role_id)
);

-- Change CREATE_TABLE: group_users
CREATE TABLE group_users (
    group_id uuid REFERENCES user_groups (id) ON DELETE CASCADE,
    user_id int8 REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamp DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

-- +migrate Down
-- Undo CREATE_TABLE: group_users
DROP TABLE IF EXISTS group_users CASCADE;

-- Undo CREATE_TABLE: group_roles
DROP TABLE IF EXISTS group_roles CASCADE;

-- Undo CREATE_TABLE: user_groups
DROP TABLE IF EXISTS user_groups CASCADE;

