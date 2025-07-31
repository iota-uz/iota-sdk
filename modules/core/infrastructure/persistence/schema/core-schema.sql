CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    name varchar(255) NOT NULL UNIQUE,
    domain varchar(255),
    is_active boolean NOT NULL DEFAULT TRUE,
    logo_id int REFERENCES uploads (id) ON DELETE SET NULL,
    logo_compact_id int REFERENCES uploads (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

CREATE TABLE uploads (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL, -- original file name
    hash VARCHAR(255) NOT NULL, -- md5 hash of the file
    path varchar(1024) NOT NULL DEFAULT '', -- relative path to the file
    slug varchar(255) NOT NULL,
    size int NOT NULL DEFAULT 0, -- in bytes
    mimetype varchar(255) NOT NULL, -- image/jpeg, application/pdf, etc.
    type VARCHAR(255) NOT NULL, -- image, document, etc.
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, hash),
    UNIQUE (tenant_id, slug)
);

CREATE TABLE passports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    first_name varchar(255),
    last_name varchar(255),
    middle_name varchar(255),
    gender varchar(10),
    birth_date date,
    birth_place varchar(255),
    nationality varchar(100),
    passport_type varchar(20), -- Type of passport (e.g., personal, diplomatic).
    passport_number varchar(20),
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
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT passports_tenant_passport_number_series_key UNIQUE (tenant_id, passport_number, series)
);

CREATE TABLE companies (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    about text,
    address varchar(255),
    phone varchar(255),
    logo_id int REFERENCES uploads (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, name)
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
    type varchar(50) NOT NULL CHECK (type IN ('system', 'user')),
    name varchar(255) NOT NULL UNIQUE,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE users (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    type varchar(50) NOT NULL CHECK (type IN ('system', 'user')),
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255),
    email varchar(255) NOT NULL,
    password VARCHAR(255),
    ui_language varchar(3) NOT NULL,
    phone varchar(255),
    avatar_id int REFERENCES uploads (id) ON DELETE SET NULL,
    last_login timestamp NULL,
    last_ip varchar(255) NULL,
    last_action timestamp with time zone NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email),
    UNIQUE (tenant_id, phone)
);

CREATE TABLE user_roles (
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id int NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE user_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    type varchar(50) NOT NULL CHECK (type IN ('system', 'user')),
    name varchar(255) UNIQUE NOT NULL,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    description text,
    created_at timestamp DEFAULT now(),
    updated_at timestamp DEFAULT now(),
    UNIQUE (tenant_id, name)
);

CREATE TABLE group_users (
    group_id uuid REFERENCES user_groups (id) ON DELETE CASCADE,
    user_id integer REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamp DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE group_roles (
    group_id uuid REFERENCES user_groups (id) ON DELETE CASCADE,
    role_id integer REFERENCES roles (id) ON DELETE CASCADE,
    created_at timestamp DEFAULT now(),
    PRIMARY KEY (group_id, role_id)
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
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    resource varchar(255) NOT NULL, -- roles, users, etc.
    action varchar(255) NOT NULL, -- create, read, update, delete
    modifier varchar(255) NOT NULL, -- all / own
    description text,
    UNIQUE (tenant_id, name)
);

CREATE TABLE role_permissions (
    role_id int NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_permissions (
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, permission_id)
);

CREATE TABLE sessions (
    token varchar(255) NOT NULL PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamp with time zone NOT NULL,
    ip varchar(255) NOT NULL,
    user_agent varchar(255) NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE tabs (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    href varchar(255) NOT NULL,
    user_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    position int NOT NULL DEFAULT 0,
    UNIQUE (tenant_id, href, user_id)
);

CREATE INDEX users_tenant_id_idx ON users (tenant_id);

CREATE INDEX users_first_name_idx ON users (first_name);

CREATE INDEX users_last_name_idx ON users (last_name);

CREATE INDEX sessions_tenant_id_idx ON sessions (tenant_id);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);

CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

CREATE INDEX uploads_tenant_id_idx ON uploads (tenant_id);

CREATE INDEX roles_tenant_id_idx ON roles (tenant_id);

CREATE INDEX user_groups_tenant_id_idx ON user_groups (tenant_id);

CREATE INDEX permissions_tenant_id_idx ON permissions (tenant_id);

CREATE INDEX tabs_tenant_id_idx ON tabs (tenant_id);

