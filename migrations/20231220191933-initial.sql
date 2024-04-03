-- +migrate Up
CREATE TABLE users
(
    id          SERIAL PRIMARY KEY,
    first_name  VARCHAR(255) NOT NULL,
    last_name   VARCHAR(255) NOT NULL,
    middle_name VARCHAR(255) NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    last_login  TIMESTAMP    NULL,
    last_ip     VARCHAR(255) NULL,
    last_action TIMESTAMP    NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE roles
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE permissions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE user_roles
(
    user_id    INT NOT NULL,
    role_id    INT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE CASCADE,
    CONSTRAINT fk_role
        FOREIGN KEY (role_id)
            REFERENCES roles (id)
            ON DELETE CASCADE
);

CREATE TABLE role_permissions
(
    role_id       INT NOT NULL,
    permission_id INT NOT NULL,
    created_at    TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_role_permissions_role
        FOREIGN KEY (role_id)
            REFERENCES roles (id)
            ON DELETE CASCADE,
    CONSTRAINT fk_permission
        FOREIGN KEY (permission_id)
            REFERENCES permissions (id)
            ON DELETE CASCADE
);

CREATE TABLE sessions
(
    token      VARCHAR(255) NOT NULL UNIQUE PRIMARY KEY,
    user_id    INTEGER      NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMP    NOT NULL,
    ip         VARCHAR(255) NOT NULL,
    user_agent VARCHAR(255) NOT NULL,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE authentication_logs
(
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER      NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    ip         VARCHAR(255) NOT NULL,
    user_agent VARCHAR(255) NOT NULL,
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE INDEX users_first_name_idx ON users (first_name);
CREATE INDEX users_last_name_idx ON users (last_name);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);
CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

CREATE INDEX user_roles_user_id_idx ON user_roles (user_id);
CREATE INDEX user_roles_role_id_idx ON user_roles (role_id);
CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

-- +migrate Down
-- Drop indexes first (if they were explicitly created; otherwise, this step is not necessary as dropping tables will also drop associated indexes)
DROP INDEX IF EXISTS authentication_logs_created_at_idx;
DROP INDEX IF EXISTS authentication_logs_user_id_idx;

DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;

DROP INDEX IF EXISTS users_last_name_idx;
DROP INDEX IF EXISTS users_first_name_idx;

-- Then drop the tables in reverse order of their creation
DROP TABLE IF EXISTS authentication_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS roles;
