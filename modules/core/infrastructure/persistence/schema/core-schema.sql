CREATE TABLE uploads (
    id serial PRIMARY KEY,
    name varchar(255) NOT NULL, -- original file name
    hash VARCHAR(255) NOT NULL UNIQUE, -- md5 hash of the file
    path varchar(1024) NOT NULL DEFAULT '', -- relative path to the file
    size int NOT NULL DEFAULT 0, -- in bytes
    mimetype varchar(255) NOT NULL, -- image/jpeg, application/pdf, etc.
    type VARCHAR(255) NOT NULL, -- image, document, etc.
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE passports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    first_name varchar(255),
    last_name varchar(255),
    middle_name varchar(255),
    gender varchar(10),
    birth_date date,
    birth_place varchar(255),
    nationality varchar(100),
    passport_type varchar(20), -- Type of passport (e.g., personal, diplomatic).
    passport_number varchar(20) UNIQUE,
    series varchar(20), -- Some countries use a prefix before the passport number.
    issuing_country varchar(100),
    issued_at date,
    issued_by varchar(255), -- Name of the authority that issued the passport.
    expires_at date,
    machine_readable_zone varchar(88), -- MRZ string found on passport data pages.
    biometric_data jsonb, -- Stores biometric details like fingerprints, iris scans.
    signature_image bytea, -- Digital signature of the passport holder.
    remarks text, -- Additional notes (e.g., travel restrictions, visa endorsements).
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE companies (
    id serial PRIMARY KEY,
    name varchar(255) NOT NULL,
    about text,
    address varchar(255),
    phone varchar(255),
    logo_id int REFERENCES uploads (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE currencies (
    code varchar(3) NOT NULL PRIMARY KEY, -- RUB
    name varchar(255) NOT NULL, -- Russian Ruble
    symbol varchar(3) NOT NULL, -- ₽
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE roles (
    id serial PRIMARY KEY,
    name varchar(255) NOT NULL UNIQUE,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE users (
    id serial PRIMARY KEY,
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255),
    email varchar(255) NOT NULL UNIQUE,
    password VARCHAR(255),
    ui_language varchar(3) NOT NULL,
    phone varchar(255) UNIQUE,
    avatar_id int REFERENCES uploads (id) ON DELETE SET NULL,
    last_login timestamp NULL,
    last_ip varchar(255) NULL,
    last_action timestamp with time zone NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE user_roles (
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id int NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE uploaded_images (
    id serial PRIMARY KEY,
    upload_id int NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    type VARCHAR(255) NOT NULL,
    size float NOT NULL,
    width int NOT NULL,
    height int NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid () NOT NULL,
    name varchar(255) NOT NULL UNIQUE,
    resource varchar(255) NOT NULL, -- roles, users, etc.
    action varchar(255) NOT NULL, -- create, read, update, delete
    modifier varchar(255) NOT NULL, -- all / own
    description text
);

CREATE TABLE role_permissions (
    role_id int NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE sessions (
    token varchar(255) NOT NULL PRIMARY KEY,
    user_id integer NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamp with time zone NOT NULL,
    ip varchar(255) NOT NULL,
    user_agent varchar(255) NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE tabs (
    id serial PRIMARY KEY,
    href varchar(255) NOT NULL,
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    position int NOT NULL DEFAULT 0,
    UNIQUE (href, user_id)
);

CREATE INDEX users_first_name_idx ON users (first_name);

CREATE INDEX users_last_name_idx ON users (last_name);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);

CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

