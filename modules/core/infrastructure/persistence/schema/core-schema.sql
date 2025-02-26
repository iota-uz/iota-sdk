-- +migrate Up
CREATE TABLE companies
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    about      TEXT,
    address    VARCHAR(255),
    phone      VARCHAR(255),
    logo_id    INT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE uploads
(
    id         SERIAL PRIMARY KEY,
    hash       VARCHAR(255)  NOT NULL UNIQUE, -- md5 hash of the file
    path       VARCHAR(1024) NOT NULL   DEFAULT '', -- relative path to the file
    size       INT           NOT NULL   DEFAULT 0, -- in bytes
    mimetype   VARCHAR(255)  NOT NULL, -- image/jpeg, application/pdf, etc.
    type       VARCHAR(255)  NOT NULL, -- image, document, etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE currencies
(
    code       VARCHAR(3)   NOT NULL PRIMARY KEY, -- RUB
    name       VARCHAR(255) NOT NULL,             -- Russian Ruble
    symbol     VARCHAR(3)   NOT NULL,             -- ₽
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE roles
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE users
(
    id          SERIAL PRIMARY KEY,
    first_name  VARCHAR(255)             NOT NULL,
    last_name   VARCHAR(255)             NOT NULL,
    middle_name VARCHAR(255),
    email       VARCHAR(255)             NOT NULL UNIQUE,
    password    VARCHAR(255),
    ui_language VARCHAR(3)               NOT NULL,
    avatar_id   INT                      REFERENCES uploads (id) ON DELETE SET NULL,
    last_login  TIMESTAMP                NULL,
    last_ip     VARCHAR(255)             NULL,
    last_action TIMESTAMP WITH TIME ZONE NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_roles
(
    user_id    INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    INT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE uploaded_images
(
    id         SERIAL PRIMARY KEY,
    upload_id  INT          NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    type       VARCHAR(255) NOT NULL,
    size       FLOAT        NOT NULL,
    width      INT          NOT NULL,
    height     INT          NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE permissions
(
    id       uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
    name     VARCHAR(255)                               NOT NULL UNIQUE,
    resource VARCHAR(255)                               NOT NULL, -- roles, users, etc.
    action   VARCHAR(255)                               NOT NULL, -- create, read, update, delete
    modifier VARCHAR(255)                               NOT NULL, -- all / own
    description TEXT
);

CREATE TABLE role_permissions
(
    role_id       INT  NOT NULL,
    permission_id uuid NOT NULL,
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
    token      VARCHAR(255)             NOT NULL UNIQUE PRIMARY KEY,
    user_id    INTEGER                  NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ip         VARCHAR(255)             NOT NULL,
    user_agent VARCHAR(255)             NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tabs
(
    id       SERIAL PRIMARY KEY,
    href     VARCHAR(255) NOT NULL,
    user_id  INT          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    position INT          NOT NULL DEFAULT 0,
    UNIQUE (href, user_id)
);

CREATE INDEX users_first_name_idx ON users (first_name);
CREATE INDEX users_last_name_idx ON users (last_name);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

