CREATE TABLE positions (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT positions_tenant_name_key UNIQUE (tenant_id, name)
);

CREATE TABLE employees (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255),
    email varchar(255) NOT NULL,
    phone varchar(255),
    salary numeric(9, 2) NOT NULL,
    salary_currency_id varchar(3) REFERENCES currencies (code) ON DELETE SET NULL,
    hourly_rate numeric(9, 2) NOT NULL,
    coefficient float NOT NULL,
    avatar_id int REFERENCES uploads (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT employees_tenant_email_key UNIQUE (tenant_id, email),
    CONSTRAINT employees_tenant_phone_key UNIQUE (tenant_id, phone)
);

CREATE TABLE employee_positions (
    employee_id int NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    position_id int NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
    PRIMARY KEY (employee_id, position_id)
);

CREATE TABLE employee_meta (
    employee_id int PRIMARY KEY NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    primary_language varchar(255),
    secondary_language varchar(255),
    tin varchar(255),
    pin varchar(255),
    notes text,
    birth_date date,
    hire_date date,
    resignation_date date
);

CREATE TABLE employee_contacts (
    id serial PRIMARY KEY,
    employee_id int NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    type VARCHAR(255) NOT NULL,
    value varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
);

