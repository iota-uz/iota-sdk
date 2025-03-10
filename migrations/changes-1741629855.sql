-- +migrate Up

-- Change CREATE_TABLE: user_groups
CREATE TABLE user_groups (
	id          SERIAL8 PRIMARY KEY,
	name        VARCHAR(255) NOT NULL UNIQUE,
	description TEXT,
	created_at  TIMESTAMP DEFAULT now(),
	updated_at  TIMESTAMP DEFAULT now()
);

-- Change CREATE_TABLE: group_roles
CREATE TABLE group_roles (
	group_id   INT8 REFERENCES user_groups (id) ON DELETE CASCADE,
	role_id    INT8 REFERENCES roles (id) ON DELETE CASCADE,
	created_at TIMESTAMP DEFAULT now(),
	PRIMARY KEY (group_id, role_id)
);

-- Change CREATE_TABLE: group_users
CREATE TABLE group_users (
	group_id   INT8 REFERENCES user_groups (id) ON DELETE CASCADE,
	user_id    INT8 REFERENCES users (id) ON DELETE CASCADE,
	created_at TIMESTAMP DEFAULT now(),
	PRIMARY KEY (group_id, user_id)
);


-- +migrate Down

-- Undo CREATE_TABLE: group_users
DROP TABLE IF EXISTS group_users CASCADE;

-- Undo CREATE_TABLE: group_roles
DROP TABLE IF EXISTS group_roles CASCADE;

-- Undo CREATE_TABLE: user_groups
DROP TABLE IF EXISTS user_groups CASCADE;

