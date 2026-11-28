-- +migrate Up

-- Change CREATE_TABLE: user_permissions
CREATE TABLE user_permissions (
	user_id       INT8 NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	permission_id UUID NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
	PRIMARY KEY (user_id, permission_id)
);


-- +migrate Down

-- Undo CREATE_TABLE: user_permissions
DROP TABLE IF EXISTS user_permissions CASCADE;

