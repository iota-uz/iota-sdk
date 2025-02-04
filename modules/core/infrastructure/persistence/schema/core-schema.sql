-- +migrate Up
CREATE TABLE uploads
(
    id         SERIAL PRIMARY KEY,
    hash       VARCHAR(255)  NOT NULL UNIQUE,
    path       VARCHAR(1024) NOT NULL   DEFAULT '',
    size       INT           NOT NULL   DEFAULT 0,
    mimetype   VARCHAR(255)  NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE positions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE settings
(
    id              SERIAL PRIMARY KEY,
    default_risks   FLOAT NOT NULL,
    default_margin  FLOAT NOT NULL,
    income_tax_rate FLOAT NOT NULL,
    social_tax_rate FLOAT NOT NULL,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE currencies
(
    code       VARCHAR(3)   NOT NULL PRIMARY KEY, -- RUB
    name       VARCHAR(255) NOT NULL,             -- Russian Ruble
    symbol     VARCHAR(3)   NOT NULL,             -- ₽
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employees
(
    id                 SERIAL PRIMARY KEY,
    first_name         VARCHAR(255)  NOT NULL,
    last_name          VARCHAR(255)  NOT NULL,
    middle_name        VARCHAR(255),
    email              VARCHAR(255)  NOT NULL UNIQUE,
    phone              VARCHAR(255),
    salary             NUMERIC(9, 2) NOT NULL,
    salary_currency_id VARCHAR(3)    REFERENCES currencies (code) ON DELETE SET NULL,
    hourly_rate        NUMERIC(9, 2) NOT NULL,
    coefficient        FLOAT         NOT NULL,
    avatar_id          INT           REFERENCES uploads (id) ON DELETE SET NULL,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employee_positions
(
    employee_id INT NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    position_id INT NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
    PRIMARY KEY (employee_id, position_id)
);

CREATE TABLE employee_meta
(
    employee_id        INT PRIMARY KEY NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    primary_language   VARCHAR(255),
    secondary_language VARCHAR(255),
    tin                VARCHAR(255),
    pin              VARCHAR(255),
    notes            TEXT,
    birth_date         DATE,
    hire_date        DATE,
    resignation_date DATE
);

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
    employee_id INT                      REFERENCES employees (id) ON DELETE SET NULL,
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

CREATE TABLE prompts
(
    id          VARCHAR(30) PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT         NOT NULL,
    prompt      TEXT         NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employee_contacts
(
    id          SERIAL PRIMARY KEY,
    employee_id INT          NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    type        VARCHAR(255) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

CREATE TABLE action_logs
(
    id         SERIAL PRIMARY KEY,
    method     VARCHAR(255) NOT NULL,
    path       VARCHAR(255) NOT NULL,
    user_id    INT          REFERENCES users (id) ON DELETE SET NULL,
    after      JSON,
    before     JSON,
    user_agent VARCHAR(255) NOT NULL,
    ip         VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE dialogues
(
    id         SERIAL PRIMARY KEY,
    user_id    INT          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label      VARCHAR(255) NOT NULL,
    messages   JSON         NOT NULL,
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

CREATE TABLE authentication_logs
(
    id         SERIAL PRIMARY KEY,
    user_id    INTEGER                  NOT NULL
        CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
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

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);
CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

CREATE INDEX employees_avatar_id_idx ON employees (avatar_id);

-- +migrate Down
DROP TABLE IF EXISTS action_log CASCADE;
DROP TABLE IF EXISTS authentication_logs CASCADE;
DROP TABLE IF EXISTS companies CASCADE;
DROP TABLE IF EXISTS currencies CASCADE;
DROP TABLE IF EXISTS dialogues CASCADE;
DROP TABLE IF EXISTS employee_contacts CASCADE;
DROP TABLE IF EXISTS employee_meta CASCADE;
DROP TABLE IF EXISTS employees CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS positions CASCADE;
DROP TABLE IF EXISTS prompts CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS settings CASCADE;
DROP TABLE IF EXISTS telegram_sessions CASCADE;
DROP TABLE IF EXISTS uploaded_images CASCADE;
DROP TABLE IF EXISTS uploads CASCADE;
DROP TABLE IF EXISTS employee_positions CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS action_logs CASCADE;
DROP TABLE IF EXISTS tabs CASCADE;
