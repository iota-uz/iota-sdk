CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    name varchar(255) NOT NULL UNIQUE,
    domain varchar(255),
    phone varchar(255),
    email varchar(255),
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
    ui_language varchar(16) NOT NULL,
    phone varchar(255),
    avatar_id int REFERENCES uploads (id) ON DELETE SET NULL,
    last_login timestamp NULL,
    last_ip varchar(255) NULL,
    last_action timestamp with time zone NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email),
    UNIQUE (tenant_id, phone),
    -- Tenant-qualified FK target so core.user_positions can reference a user by
    -- (tenant_id, id) and never bridge tenants.
    CONSTRAINT users_tenant_id_id_key UNIQUE (tenant_id, id)
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
    UNIQUE (tenant_id, name),
    CONSTRAINT user_groups_tenant_id_id_key UNIQUE (tenant_id, id)
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

CREATE SCHEMA IF NOT EXISTS core;

CREATE TABLE core.departments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    parent_id uuid NULL,
    code varchar NOT NULL,
    name jsonb NOT NULL,
    "order" int NOT NULL DEFAULT 0,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE (tenant_id, code),
    CONSTRAINT departments_tenant_id_id_key UNIQUE (tenant_id, id),
    CONSTRAINT departments_parent_tenant_fkey FOREIGN KEY (tenant_id, parent_id) REFERENCES core.departments (tenant_id, id) ON DELETE SET NULL (parent_id)
);

CREATE INDEX departments_tenant_id_idx ON core.departments (tenant_id);

CREATE INDEX departments_tenant_id_parent_id_idx ON core.departments (tenant_id, parent_id);

CREATE TABLE core.user_positions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    department_id uuid NOT NULL,
    title jsonb NOT NULL,
    is_manager boolean NOT NULL DEFAULT FALSE,
    is_primary boolean NOT NULL DEFAULT FALSE,
    status varchar NOT NULL DEFAULT 'active',
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT user_positions_department_tenant_fkey FOREIGN KEY (tenant_id, department_id) REFERENCES core.departments (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT user_positions_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE
);

CREATE INDEX user_positions_tenant_id_user_id_idx ON core.user_positions (tenant_id, user_id);

CREATE INDEX user_positions_tenant_id_department_id_idx ON core.user_positions (tenant_id, department_id);

CREATE TABLE core.notifications (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    event_key varchar(200) NOT NULL DEFAULT '',
    level text NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'success', 'warning', 'error')),
    title varchar(300) NOT NULL,
    body text NOT NULL DEFAULT '',
    action_url varchar(2000) NOT NULL DEFAULT '',
    dedupe_key varchar(300),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    read_at timestamptz,
    CONSTRAINT notifications_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    UNIQUE (tenant_id, user_id, dedupe_key)
);

CREATE INDEX notifications_recipient_idx ON core.notifications (tenant_id, user_id, created_at DESC, id DESC);

CREATE INDEX notifications_unread_idx ON core.notifications (tenant_id, user_id)
WHERE
    read_at IS NULL;

CREATE TABLE core.notification_rules (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    enabled boolean NOT NULL DEFAULT FALSE,
    user_ids jsonb NOT NULL DEFAULT '[]',
    group_ids jsonb NOT NULL DEFAULT '[]',
    role_ids jsonb NOT NULL DEFAULT '[]',
    recipient_keys jsonb NOT NULL DEFAULT '[]',
    level text NOT NULL DEFAULT '' CHECK (level IN ('',
    'info',
    'success',
    'warning',
    'error')),
channels jsonb NOT NULL DEFAULT '["in_app"]',
created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
PRIMARY KEY (tenant_id,
    event_key));

CREATE TABLE core.notification_notices (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    event_version integer NOT NULL DEFAULT 1 CHECK (event_version >= 1),
    snapshot_ref text NOT NULL DEFAULT '',
    event_id text NOT NULL,
    mode text NOT NULL CHECK (mode IN ('rule', 'explicit')),
    subject_entity text NOT NULL DEFAULT '',
    subject_key text NOT NULL DEFAULT '',
    producer_invocation_id text NOT NULL DEFAULT '',
    producer_event_id text NOT NULL DEFAULT '',
    producer_instance_id text NOT NULL DEFAULT '',
    producer_occurrence text NOT NULL DEFAULT '',
    event jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, id),
    UNIQUE (tenant_id, event_key, event_id),
    CHECK ((subject_entity = '') = (subject_key = ''))
);

CREATE INDEX notification_notices_subject_idx ON core.notification_notices (tenant_id, subject_entity, subject_key)
WHERE
    subject_entity <> '';

CREATE TABLE core.notification_deliveries (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    notice_id uuid NOT NULL,
    event_key text NOT NULL,
    event_id text NOT NULL,
    content_version integer NOT NULL DEFAULT 1,
    recipient_kind text NOT NULL DEFAULT 'user' CHECK (recipient_kind IN ('user', 'record', 'address')),
    recipient_user_id integer,
    recipient_ref_entity text,
    recipient_ref_key text,
    recipient_key text NOT NULL DEFAULT '',
    channel text NOT NULL CHECK (channel IN ('in_app', 'email', 'sms', 'push')),
    target_key text NOT NULL DEFAULT '',
    target_address text NOT NULL DEFAULT '',
    target_device_id text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    language text
    NOT NULL DEFAULT 'en',
    level text NOT NULL DEFAULT 'info' CHECK (level IN ('info', 'success', 'warning', 'error')),
    title varchar(300) NOT NULL,
    body text NOT NULL DEFAULT '',
    action_url varchar(2000) NOT NULL DEFAULT '',
    content jsonb NOT NULL DEFAULT '{}',
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    attempt_limit integer NOT NULL DEFAULT 5 CHECK (attempt_limit >= 1),
    error_code text NOT NULL DEFAULT '',
    provider_receipt text NOT NULL DEFAULT '',
    inbox_notification_id uuid REFERENCES core.notifications (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at timestamptz,
    CONSTRAINT notification_deliveries_user_tenant_fkey FOREIGN KEY (tenant_id, recipient_user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT notification_deliveries_notice_fkey FOREIGN KEY (tenant_id, notice_id) REFERENCES core.notification_notices (tenant_id, id) ON DELETE CASCADE,
    CONSTRAINT notification_deliveries_status_check CHECK (status IN ('unresolved', 'pending', 'sent', 'failed', 'ambiguous', 'no_address', 'skipped')),
    CONSTRAINT notification_deliveries_recipient_check CHECK ((recipient_kind = 'user' AND recipient_user_id IS NOT NULL AND recipient_ref_entity IS NULL AND recipient_ref_key IS NULL) OR (recipient_kind = 'record' AND recipient_user_id IS NULL AND recipient_ref_entity IS NOT NULL AND recipient_ref_key IS NOT NULL) OR (recipient_kind = 'address' AND recipient_user_id IS NULL AND recipient_ref_entity IS NULL AND recipient_ref_key IS NULL AND (target_address <> '' OR status = 'no_address'))),
    CONSTRAINT notification_deliveries_in_app_check CHECK (channel <> 'in_app' OR recipient_kind = 'user'),
    CONSTRAINT notification_deliveries_inbox_check CHECK (inbox_notification_id IS NULL OR channel = 'in_app'),
    CONSTRAINT notification_deliveries_target_key UNIQUE (tenant_id, notice_id, recipient_key, channel, target_key)
);

CREATE INDEX notification_deliveries_notice_idx ON core.notification_deliveries (tenant_id, event_key, event_id);

CREATE INDEX notification_deliveries_due_idx ON core.notification_deliveries (tenant_id, available_at, created_at)
WHERE
    status IN ('pending', 'unresolved') AND channel <> 'in_app';

CREATE TABLE core.notification_delivery_attempts (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    delivery_id uuid NOT NULL REFERENCES core.notification_deliveries (id) ON DELETE CASCADE,
    attempt integer NOT NULL CHECK (attempt >= 1),
    started_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at timestamptz,
    outcome text CHECK (outcome IN ('sent', 'failed', 'permanent', 'ambiguous')),
    provider_receipt text NOT NULL DEFAULT '',
    error_code text NOT NULL DEFAULT '',
    PRIMARY KEY (delivery_id, attempt),
    CHECK ((outcome IS NULL) = (finished_at IS NULL))
);

CREATE INDEX notification_delivery_attempts_tenant_idx ON core.notification_delivery_attempts (tenant_id, delivery_id);

CREATE TABLE core.notification_push_devices (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    device_id text NOT NULL CHECK (device_id <> ''),
    token text NOT NULL CHECK (token <> ''),
    platform text NOT NULL CHECK (platform IN ('ios', 'android', 'web')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at timestamptz,
    CONSTRAINT notification_push_devices_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, user_id, device_id)
);

CREATE UNIQUE INDEX notification_push_devices_token_idx ON core.notification_push_devices (tenant_id, token)
WHERE
    revoked_at IS NULL;

CREATE TABLE core.notification_channel_preferences (
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL,
    channel text NOT NULL CHECK (channel IN ('in_app', 'email', 'sms', 'push')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT notification_channel_preferences_user_tenant_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE,
    PRIMARY KEY (tenant_id, user_id, channel)
);

CREATE TABLE core.notification_dispatches (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    event_key text NOT NULL,
    event_id text NOT NULL,
    event jsonb NOT NULL,
    rule jsonb NOT NULL,
    recipient_ids jsonb NOT NULL,
    mode text NOT NULL DEFAULT 'rule' CHECK (mode IN ('rule', 'explicit')),
    addressees jsonb NOT NULL DEFAULT '[]',
    cursor integer NOT NULL DEFAULT 0 CHECK (CURSOR >= 0),
    delivered integer NOT NULL DEFAULT 0 CHECK (delivered >= 0),
    attempts integer NOT NULL DEFAULT 0,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at timestamptz,
    UNIQUE (tenant_id, event_key, event_id)
);

CREATE INDEX notification_dispatches_ready_idx ON core.notification_dispatches (tenant_id, available_at, created_at)
WHERE
    completed_at IS NULL;

