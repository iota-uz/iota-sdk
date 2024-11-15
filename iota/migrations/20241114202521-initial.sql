-- +migrate Up
BEGIN;

CREATE TABLE task_types
(
    id          SERIAL PRIMARY KEY,
    icon        VARCHAR(255),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE projects
(
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    counterparty_id INT          NOT NULL REFERENCES counterparty (id) ON DELETE CASCADE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
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

CREATE INDEX project_stages_project_id_idx ON project_stages (project_id);

CREATE INDEX project_tasks_stage_id_idx ON project_tasks (stage_id);
CREATE INDEX project_tasks_type_id_idx ON project_tasks (type_id);
CREATE INDEX project_tasks_level_id_idx ON project_tasks (level_id);
CREATE INDEX project_tasks_parent_id_idx ON project_tasks (parent_id);

CREATE INDEX estimates_task_id_idx ON estimates (task_id);
CREATE INDEX estimates_employee_id_idx ON estimates (employee_id);

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
DROP TABLE IF EXISTS counterparty_contacts CASCADE;
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
DROP TABLE IF EXISTS transactions CASCADE;
DROP TABLE IF EXISTS action_logs CASCADE;

COMMIT;