CREATE TABLE positions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

CREATE TABLE employee_contacts
(
    id          SERIAL PRIMARY KEY,
    employee_id INT          NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    type        VARCHAR(255) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);


