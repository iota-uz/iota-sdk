-- +migrate Up
CREATE EXTENSION vector;

CREATE TABLE companies
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    about      TEXT,
    address    VARCHAR(255),
    phone      VARCHAR(255),
    logo_id    INT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE roles
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE users
(
    id          SERIAL PRIMARY KEY,
    first_name  VARCHAR(255) NOT NULL,
    last_name   VARCHAR(255) NOT NULL,
    middle_name VARCHAR(255) NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    company_id  INT REFERENCES companies (id) ON DELETE CASCADE,
    role_id     INT          REFERENCES roles (id) ON DELETE SET NULL,
    password    VARCHAR(255) NOT NULL,
    last_login  TIMESTAMP    NULL,
    last_ip     VARCHAR(255) NULL,
    last_action TIMESTAMP    NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE prompts
(
    id          VARCHAR(30) PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL,
    prompt      TEXT         NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expense_categories
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    amount      FLOAT        NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expenses
(
    id          SERIAL PRIMARY KEY,
    amount      FLOAT NOT NULL,
    category_id INT   NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE uploads
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    path        VARCHAR(255) NOT NULL,
    uploader_id INT          REFERENCES users (id) ON DELETE SET NULL,
    mimetype    VARCHAR(255) NOT NULL,
    size        FLOAT        NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employees
(
    id          SERIAL PRIMARY KEY,
    first_name  VARCHAR(255) NOT NULL,
    last_name   VARCHAR(255) NOT NULL,
    middle_name VARCHAR(255) NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    salary      FLOAT        NOT NULL,
    avatar_id   INT          REFERENCES uploads (id) ON DELETE SET NULL,
    company_id  INT          NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE folders
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    parent_id  INT REFERENCES folders (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE articles
(
    id         SERIAL PRIMARY KEY,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL,
    folder_id  INT          REFERENCES folders (id) ON DELETE SET NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE embeddings
(
    id         SERIAL PRIMARY KEY,
    embedding  VECTOR(512) NOT NULL,
    article_id INT         NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    text       TEXT        NOT NULL
);

CREATE TABLE comments
(
    id         SERIAL PRIMARY KEY,
    article_id INT  NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    user_id    INT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE likes
(
    id         SERIAL PRIMARY KEY,
    article_id INT NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE uploaded_images
(
    id         SERIAL PRIMARY KEY,
    upload_id  INT          NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    type       VARCHAR(255) NOT NULL,
    size       FLOAT        NOT NULL,
    width      INT          NOT NULL,
    height     INT          NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE action_log
(
    id         SERIAL PRIMARY KEY,
    method     VARCHAR(255) NOT NULL,
    path       VARCHAR(255) NOT NULL,
    user_id    INT          REFERENCES users (id) ON DELETE SET NULL,
    after      JSON,
    before     JSON,
    user_agent VARCHAR(255) NOT NULL,
    ip         VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE dialogues
(
    id      SERIAL PRIMARY KEY,
    user_id INT          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label   VARCHAR(255) NOT NULL
);

CREATE TABLE messages
(
    id          SERIAL PRIMARY KEY,
    dialogue_id INT  NOT NULL REFERENCES dialogues (id) ON DELETE CASCADE,
    content     TEXT NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE permissions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

-- +migrate Down
DROP INDEX IF EXISTS user_roles_role_id_idx;
DROP INDEX IF EXISTS user_roles_user_id_idx;
DROP INDEX IF EXISTS role_permissions_permission_id_idx;
DROP INDEX IF EXISTS role_permissions_role_id_idx;
DROP INDEX IF EXISTS authentication_logs_created_at_idx;
DROP INDEX IF EXISTS authentication_logs_user_id_idx;
DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;
DROP INDEX IF EXISTS users_last_name_idx;
DROP INDEX IF EXISTS users_first_name_idx;

DROP TABLE IF EXISTS prompts CASCADE;
DROP TABLE IF EXISTS expenses CASCADE;
DROP TABLE IF EXISTS expense_categories CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS messages CASCADE;
DROP TABLE IF EXISTS dialogues CASCADE;
DROP TABLE IF EXISTS authentication_logs CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS uploaded_images CASCADE;
DROP TABLE IF EXISTS uploads CASCADE;
DROP TABLE IF EXISTS action_log CASCADE;
DROP TABLE IF EXISTS likes CASCADE;
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS articles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS companies CASCADE;
DROP TABLE IF EXISTS folders CASCADE;
