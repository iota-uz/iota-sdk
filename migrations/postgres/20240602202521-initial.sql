-- +migrate Up
BEGIN;

CREATE TABLE uploads
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    path       VARCHAR(255) NOT NULL,
--     uploader_id INT          REFERENCES users (id) ON DELETE SET NULL,
    mimetype   VARCHAR(255) NOT NULL,
    size       FLOAT        NOT NULL,
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

CREATE TABLE inventory
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255)  NOT NULL,
    description TEXT,
    currency_id VARCHAR(3)    REFERENCES currencies (code) ON DELETE SET NULL,
    price       NUMERIC(9, 2) NOT NULL,
    quantity    INT           NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE positions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_units
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL, -- Kilogram, Piece, etc.
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_positions
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    barcode     VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    unit_id     INT          REFERENCES warehouse_units (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_position_images
(
    warehouse_position_id INT REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    upload_id             INT REFERENCES uploads (id) ON DELETE CASCADE,
    PRIMARY KEY (upload_id, warehouse_position_id)
);

CREATE TABLE warehouse_products
(
    id          SERIAL PRIMARY KEY,
    position_id INT          NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    rfid        VARCHAR(255) NOT NULL UNIQUE,
    status      VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE inventory_checks
(
    id         SERIAL PRIMARY KEY,
    status     VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE inventory_check_results
(
    id                 SERIAL PRIMARY KEY,
    inventory_check_id INT NOT NULL REFERENCES inventory_checks (id) ON DELETE CASCADE,
    position_id        INT NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    expected_quantity  INT NOT NULL,
    actual_quantity    INT NOT NULL,
    difference         INT NOT NULL,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_orders
(
    id         SERIAL PRIMARY KEY,
    type       VARCHAR(255) NOT NULL,
    status     VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_order_items
(
    warehouse_order_id INT NOT NULL REFERENCES warehouse_orders (id) ON DELETE CASCADE,
    product_id         INT NOT NULL REFERENCES warehouse_products (id) ON DELETE CASCADE,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (warehouse_order_id, product_id)
);

CREATE TABLE difficulty_levels
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    coefficient FLOAT        NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE task_types
(
    id          SERIAL PRIMARY KEY,
    icon        VARCHAR(255),
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
    position_id        INT           NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
    avatar_id          INT           REFERENCES uploads (id) ON DELETE SET NULL,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employee_meta
(
    employee_id        INT PRIMARY KEY NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    primary_language   VARCHAR(255),
    secondary_language VARCHAR(255),
    tin                VARCHAR(255),
    general_info       TEXT,
    yt_profile_id      VARCHAR(255)    NOT NULL,
    birth_date         DATE,
    join_date          DATE,
    leave_date         DATE,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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
    avatar_id   INT                      REFERENCES uploads (id) ON DELETE SET NULL,
    last_login  TIMESTAMP                NULL,
    last_ip     VARCHAR(255)             NULL,
    last_action TIMESTAMP WITH TIME ZONE NULL,
    employee_id INT                      REFERENCES employees (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE telegram_sessions
(
    user_id    INT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    session    TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

CREATE TABLE expense_categories
(
    id                 SERIAL PRIMARY KEY,
    name               VARCHAR(255)  NOT NULL,
    description        TEXT,
    amount             NUMERIC(9, 2) NOT NULL,
    amount_currency_id VARCHAR(3)    NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

CREATE TABLE customers
(
    id          SERIAL PRIMARY KEY,
    first_name  VARCHAR(255) NOT NULL,
    last_name   VARCHAR(255) NOT NULL,
    middle_name VARCHAR(255) NULL,
    email       VARCHAR(255),
    phone       VARCHAR(255),
    company_id  INT          REFERENCES companies (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE customer_contacts
(
    id          SERIAL PRIMARY KEY,
    customer_id INT          NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    type        VARCHAR(255) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE projects
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE project_stages
(
    id         SERIAL PRIMARY KEY,
    project_id INT                      NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       VARCHAR(255)             NOT NULL,
    margin     FLOAT                    NOT NULL,
    risks      FLOAT                    NOT NULL,
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date   TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE project_tasks
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    stage_id    INT          NOT NULL REFERENCES project_stages (id) ON DELETE CASCADE,
    type_id     INT          NOT NULL REFERENCES task_types (id) ON DELETE CASCADE,
    level_id    INT          NOT NULL REFERENCES difficulty_levels (id) ON DELETE CASCADE,
    parent_id   INT REFERENCES project_tasks (id) ON DELETE CASCADE,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE estimates
(
    id          SERIAL PRIMARY KEY,
    task_id     INT   NOT NULL REFERENCES project_tasks (id) ON DELETE CASCADE,
    employee_id INT   NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    hours       FLOAT NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE money_accounts
(
    id                  SERIAL PRIMARY KEY,
    name                VARCHAR(255)  NOT NULL,
    account_number      VARCHAR(255)  NOT NULL,
    description         TEXT,
    balance             NUMERIC(9, 2) NOT NULL,
    balance_currency_id VARCHAR(3)    NOT NULL REFERENCES currencies (code) ON DELETE CASCADE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at          TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE transactions
(
    id                 SERIAL PRIMARY KEY,
    amount             NUMERIC(9, 2) NOT NULL,
    amount_currency_id VARCHAR(3)    NOT NULL REFERENCES currencies (code) ON DELETE RESTRICT,
    money_account_id   INT           NOT NULL REFERENCES money_accounts (id) ON DELETE RESTRICT,
    transaction_date   DATE          NOT NULL   DEFAULT CURRENT_DATE,
    accounting_period  DATE          NOT NULL   DEFAULT CURRENT_DATE,
    transaction_type   VARCHAR(255)  NOT NULL, -- income, expense, transfer
    comment            TEXT,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expenses
(
    id             SERIAL PRIMARY KEY,
    transaction_id INT NOT NULL REFERENCES transactions (id) ON DELETE CASCADE,
    category_id    INT NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE payments
(
    id             SERIAL PRIMARY KEY,
    stage_id       INT NOT NULL REFERENCES project_stages (id) ON DELETE RESTRICT,
    transaction_id INT NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE folders
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    icon_id    INT          REFERENCES uploads (id) ON DELETE SET NULL,
    parent_id  INT REFERENCES folders (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE articles
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    content     TEXT         NOT NULL,
    title_emoji VARCHAR(255),
    author_id   INT          REFERENCES users (id) ON DELETE SET NULL,
    picture_id  INT          REFERENCES uploads (id) ON DELETE SET NULL,
    folder_id   INT          REFERENCES folders (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE comments
(
    id         SERIAL PRIMARY KEY,
    article_id INT  NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    user_id    INT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE likes
(
    id         SERIAL PRIMARY KEY,
    article_id INT NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL UNIQUE,
    resource    VARCHAR(255) NOT NULL,
    module      VARCHAR(255) NOT NULL,
    modifier    VARCHAR(255) NOT NULL,
    description TEXT
);

CREATE TABLE role_permissions
(
    role_id       INT NOT NULL,
    permission_id INT NOT NULL,
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

CREATE TABLE vacancies
(
    id         SERIAL PRIMARY KEY,
    url        VARCHAR(255) NOT NULL,
    title      VARCHAR(255) NOT NULL,
    body       TEXT,
    hidden     BOOLEAN      NOT NULL    DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE salary_range
(
    min_salary             NUMERIC(9, 2)   NOT NULL,
    max_salary             NUMERIC(9, 2)   NOT NULL,
    min_salary_currency_id VARCHAR(3)      REFERENCES currencies (code) ON DELETE SET NULL,
    max_salary_currency_id VARCHAR(3)      REFERENCES currencies (code) ON DELETE SET NULL,
    vacancy_id             INT PRIMARY KEY NOT NULL REFERENCES vacancies (id) ON DELETE CASCADE
);

CREATE TABLE applicants
(
    id                  SERIAL PRIMARY KEY,
    first_name          VARCHAR(255) NOT NULL,
    last_name           VARCHAR(255) NOT NULL,
    middle_name         VARCHAR(255),
    primary_language    VARCHAR(255),
    secondary_language  VARCHAR(255),
    email               VARCHAR(255) NOT NULL,
    phone               VARCHAR(255) NOT NULL,
    experience_in_month INT          NOT NULL,
    vacancy_id          INT          NOT NULL REFERENCES vacancies (id) ON DELETE CASCADE,
    created_at          TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE skills
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employee_skills
(
    employee_id INT NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    skill_id    INT NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (employee_id, skill_id)
);

CREATE TABLE applicant_skills
(
    applicant_id INT NOT NULL REFERENCES applicants (id) ON DELETE CASCADE,
    skill_id     INT NOT NULL REFERENCES skills (id) ON DELETE CASCADE,
    PRIMARY KEY (applicant_id, skill_id)
);

CREATE TABLE applicant_comments
(
    id           SERIAL PRIMARY KEY,
    applicant_id INT  NOT NULL REFERENCES applicants (id) ON DELETE CASCADE,
    user_id      INT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content      TEXT NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE applications
(
    id           SERIAL PRIMARY KEY,
    applicant_id INT NOT NULL REFERENCES applicants (id) ON DELETE CASCADE,
    vacancy_id   INT NOT NULL REFERENCES vacancies (id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interview_questions
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    type        VARCHAR(255) NOT NULL,
    language    VARCHAR(255) NOT NULL,
    difficulty  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interviews
(
    id             SERIAL PRIMARY KEY,
    application_id INT                      NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    interviewer_id INT                      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    date           TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interview_ratings
(
    id             SERIAL PRIMARY KEY,
    interview_id   INT NOT NULL REFERENCES interviews (id) ON DELETE CASCADE,
    interviewer_id INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    question_id    INT NOT NULL REFERENCES interview_questions (id) ON DELETE CASCADE,
    rating         INT NOT NULL,
    comment        TEXT,
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE contact_form_submissions
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL,
    phone      VARCHAR(255),
    company    VARCHAR(255),
    message    TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE INDEX users_first_name_idx ON users (first_name);
CREATE INDEX users_last_name_idx ON users (last_name);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);
CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

CREATE INDEX role_permissions_role_id_idx ON role_permissions (role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE INDEX articles_folder_id_idx ON articles (folder_id);

CREATE INDEX comments_article_id_idx ON comments (article_id);
CREATE INDEX comments_user_id_idx ON comments (user_id);

CREATE INDEX likes_article_id_idx ON likes (article_id);
CREATE INDEX likes_user_id_idx ON likes (user_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

CREATE INDEX expenses_category_id_idx ON expenses (category_id);

CREATE INDEX employees_position_id_idx ON employees (position_id);
CREATE INDEX employees_avatar_id_idx ON employees (avatar_id);

CREATE INDEX folders_parent_id_idx ON folders (parent_id);

CREATE INDEX project_stages_project_id_idx ON project_stages (project_id);

CREATE INDEX project_tasks_stage_id_idx ON project_tasks (stage_id);
CREATE INDEX project_tasks_type_id_idx ON project_tasks (type_id);
CREATE INDEX project_tasks_level_id_idx ON project_tasks (level_id);
CREATE INDEX project_tasks_parent_id_idx ON project_tasks (parent_id);

CREATE INDEX estimates_task_id_idx ON estimates (task_id);
CREATE INDEX estimates_employee_id_idx ON estimates (employee_id);

CREATE INDEX payments_staged_id_idx ON payments (stage_id);

CREATE INDEX customers_company_id_idx ON customers (company_id);

CREATE INDEX customer_contacts_customer_id_idx ON customer_contacts (customer_id);

CREATE INDEX vacancies_salary_range_id_idx ON vacancies (id);

CREATE INDEX applicants_vacancy_id_idx ON applicants (vacancy_id);

CREATE INDEX applicant_comments_applicant_id_idx ON applicant_comments (applicant_id);

CREATE INDEX applications_applicant_id_idx ON applications (applicant_id);

CREATE INDEX interview_ratings_interview_id_idx ON interview_ratings (interview_id);
CREATE INDEX interviews_application_id_idx ON interviews (application_id);
CREATE INDEX interviews_interviewer_id_idx ON interviews (interviewer_id);


-- +migrate Down
DROP TABLE IF EXISTS action_log CASCADE;
DROP TABLE IF EXISTS applicant_comments CASCADE;
DROP TABLE IF EXISTS applicant_skills CASCADE;
DROP TABLE IF EXISTS applicants CASCADE;
DROP TABLE IF EXISTS applications CASCADE;
DROP TABLE IF EXISTS articles CASCADE;
DROP TABLE IF EXISTS authentication_logs CASCADE;
DROP TABLE IF EXISTS companies CASCADE;
DROP TABLE IF EXISTS comments CASCADE;
DROP TABLE IF EXISTS contact_form_submissions CASCADE;
DROP TABLE IF EXISTS currencies CASCADE;
DROP TABLE IF EXISTS customer_contacts CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
DROP TABLE IF EXISTS dialogues CASCADE;
DROP TABLE IF EXISTS difficulty_levels CASCADE;
DROP TABLE IF EXISTS employee_contacts CASCADE;
DROP TABLE IF EXISTS employee_meta CASCADE;
DROP TABLE IF EXISTS employee_skills CASCADE;
DROP TABLE IF EXISTS employees CASCADE;
DROP TABLE IF EXISTS estimates CASCADE;
DROP TABLE IF EXISTS expense_categories CASCADE;
DROP TABLE IF EXISTS expenses CASCADE;
DROP TABLE IF EXISTS folders CASCADE;
DROP TABLE IF EXISTS interview_questions CASCADE;
DROP TABLE IF EXISTS interview_ratings CASCADE;
DROP TABLE IF EXISTS interviews CASCADE;
DROP TABLE IF EXISTS inventory CASCADE;
DROP TABLE IF EXISTS likes CASCADE;
DROP TABLE IF EXISTS permissions CASCADE;
DROP TABLE IF EXISTS positions CASCADE;
DROP TABLE IF EXISTS warehouse_position_images CASCADE;
DROP TABLE IF EXISTS warehouse_positions CASCADE;
DROP TABLE IF EXISTS warehouse_products CASCADE;
DROP TABLE IF EXISTS inventory_checks CASCADE;
DROP TABLE IF EXISTS inventory_check_results CASCADE;
DROP TABLE IF EXISTS warehouse_orders CASCADE;
DROP TABLE IF EXISTS warehouse_order_items CASCADE;
DROP TABLE IF EXISTS warehouse_units CASCADE;
DROP TABLE IF EXISTS salary_range CASCADE;
DROP TABLE IF EXISTS money_accounts CASCADE;
DROP TABLE IF EXISTS prompts CASCADE;
DROP TABLE IF EXISTS project_stages CASCADE;
DROP TABLE IF EXISTS project_tasks CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS role_permissions CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS salary_range CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS settings CASCADE;
DROP TABLE IF EXISTS skills CASCADE;
DROP TABLE IF EXISTS task_types CASCADE;
DROP TABLE IF EXISTS telegram_sessions CASCADE;
DROP TABLE IF EXISTS uploaded_images CASCADE;
DROP TABLE IF EXISTS uploads CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS vacancies CASCADE;
DROP TABLE IF EXISTS payments CASCADE;

COMMIT;