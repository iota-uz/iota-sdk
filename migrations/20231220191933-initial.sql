-- +migrate Up
CREATE EXTENSION vector;

CREATE TABLE money
(
    id          SERIAL PRIMARY KEY,
    value       NUMERIC(9, 2) NOT NULL,
    currency_id INT           REFERENCES currencies (id) ON DELETE SET NULL
);

CREATE TABLE inventory
(
    id             SERIAL PRIMARY KEY,
    name           VARCHAR(255) NOT NULL,
    description    TEXT,
    price_money_id INT          REFERENCES money (id) ON DELETE SET NULL,
    quantity       INT          NOT NULL,
    created_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE positions
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE currencies
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL, -- Russian Ruble
    code       VARCHAR(3)   NOT NULL, -- RUB
    symbol     VARCHAR(3)   NOT NULL, -- ₽
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE difficulty_levels
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    coefficient FLOAT        NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE task_types
(
    id          SERIAL PRIMARY KEY,
    icon        VARCHAR(255),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE settings
(
    id             SERIAL PRIMARY KEY,
    default_risks  FLOAT NOT NULL,
    default_margin FLOAT NOT NULL,
    ndfl           FLOAT NOT NULL,
    esn            FLOAT NOT NULL,
    updated_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

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
    password    VARCHAR(255) NOT NULL,
    avatar_id   INT          REFERENCES uploads (id) ON DELETE SET NULL,
    last_login  TIMESTAMP    NULL,
    last_ip     VARCHAR(255) NULL,
    last_action TIMESTAMP    NULL,
    employee_id INT          REFERENCES employees (id) ON DELETE SET NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE telegram_sessions
(
    user_id    INT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    session    TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE user_roles
(
    user_id    INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    INT NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (user_id, role_id)
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
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    amount_money_id INT          REFERENCES money (id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE expenses
(
    id              SERIAL PRIMARY KEY,
    amount_money_id INT  REFERENCES money (id) ON DELETE SET NULL,
    category_id     INT  NOT NULL REFERENCES expense_categories (id) ON DELETE CASCADE,
    date            DATE NOT NULL               DEFAULT CURRENT_DATE,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    id              SERIAL PRIMARY KEY,
    first_name      VARCHAR(255) NOT NULL,
    last_name       VARCHAR(255) NOT NULL,
    middle_name     VARCHAR(255) NULL,
    email           VARCHAR(255) NOT NULL UNIQUE,
    phone           VARCHAR(255),
    salary_money_id INT          REFERENCES money (id) ON DELETE SET NULL,
    hourly_rate     FLOAT        NOT NULL,
    coefficient     FLOAT        NOT NULL,
    position_id     INT          NOT NULL REFERENCES positions (id) ON DELETE CASCADE,
    avatar_id       INT          REFERENCES uploads (id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    updated_at         TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE employee_contacts
(
    id          SERIAL PRIMARY KEY,
    employee_id INT          NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    type        VARCHAR(255) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE customer_contacts
(
    id          SERIAL PRIMARY KEY,
    customer_id INT          NOT NULL REFERENCES customers (id) ON DELETE CASCADE,
    type        VARCHAR(255) NOT NULL,
    value       VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE projects
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE project_stages
(
    id         SERIAL PRIMARY KEY,
    project_id INT                         NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       VARCHAR(255)                NOT NULL,
    margin     FLOAT                       NOT NULL,
    risks      FLOAT                       NOT NULL,
    start_date TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    end_date   TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE estimates
(
    id          SERIAL PRIMARY KEY,
    task_id     INT   NOT NULL REFERENCES project_tasks (id) ON DELETE CASCADE,
    employee_id INT   NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    hours       FLOAT NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE payments
(
    id              SERIAL PRIMARY KEY,
    amount_money_id INT NOT NULL REFERENCES money (id) ON DELETE SET NULL,
    customer_id     INT REFERENCES customers (id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at      TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE folders
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    icon_id    INT          REFERENCES uploads (id) ON DELETE SET NULL,
    parent_id  INT REFERENCES folders (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE embeddings
(
    id         SERIAL PRIMARY KEY,
    embedding  VECTOR(384) NOT NULL,
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
    id         SERIAL PRIMARY KEY,
    user_id    INT          NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    label      VARCHAR(255) NOT NULL,
    messages   JSON         NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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

CREATE TABLE vacancies
(
    id         SERIAL PRIMARY KEY,
    url        VARCHAR(255) NOT NULL,
    title      VARCHAR(255) NOT NULL,
    body       TEXT,
    hidden     BOOLEAN      NOT NULL       DEFAULT FALSE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE salary_range
(
    min_salary_money_id INT             REFERENCES money (id) ON DELETE SET NULL,
    max_salary_money_id INT             REFERENCES money (id) ON DELETE SET NULL,
    vacancy_id          INT PRIMARY KEY NOT NULL REFERENCES vacancies (id) ON DELETE CASCADE
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
    created_at          TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE skills
(
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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
    created_at   TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE applications
(
    id           SERIAL PRIMARY KEY,
    applicant_id INT NOT NULL REFERENCES applicants (id) ON DELETE CASCADE,
    vacancy_id   INT NOT NULL REFERENCES vacancies (id) ON DELETE CASCADE,
    created_at   TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interview_questions
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    description TEXT,
    type        VARCHAR(255) NOT NULL,
    language    VARCHAR(255) NOT NULL,
    difficulty  VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interviews
(
    id             SERIAL PRIMARY KEY,
    application_id INT                         NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
    interviewer_id INT                         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    date           TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE interview_ratings
(
    id             SERIAL PRIMARY KEY,
    interview_id   INT NOT NULL REFERENCES interviews (id) ON DELETE CASCADE,
    interviewer_id INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    question_id    INT NOT NULL REFERENCES interview_questions (id) ON DELETE CASCADE,
    rating         INT NOT NULL,
    comment        TEXT,
    created_at     TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE contact_form_submissions
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL,
    phone      VARCHAR(255),
    company    VARCHAR(255),
    message    TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE blog_posts
(
    id         SERIAL PRIMARY KEY,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL,
    author_id  INT          REFERENCES users (id) ON DELETE SET NULL,
    picture_id INT          REFERENCES uploads (id) ON DELETE SET NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE blog_post_tags
(
    id         SERIAL PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE blog_post_tag_relations
(
    post_id INT NOT NULL REFERENCES blog_posts (id) ON DELETE CASCADE,
    tag_id  INT NOT NULL REFERENCES blog_post_tags (id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);

CREATE TABLE blog_comments
(
    id         SERIAL PRIMARY KEY,
    post_id    INT  NOT NULL REFERENCES blog_posts (id) ON DELETE CASCADE,
    content    TEXT NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE blog_likes
(
    id         SERIAL PRIMARY KEY,
    post_id    INT NOT NULL REFERENCES blog_posts (id) ON DELETE CASCADE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE website_pages
(
    id         SERIAL PRIMARY KEY,
    path       VARCHAR(255) NOT NULL,
    seo_title  VARCHAR(255),
    seo_desc   TEXT,
    seo_keys   TEXT,
    seo_h1     VARCHAR(255),
    seo_h2     VARCHAR(255),
    seo_img    VARCHAR(255),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE website_page_views
(
    id         SERIAL PRIMARY KEY,
    page_id    INT NOT NULL REFERENCES website_pages (id) ON DELETE CASCADE,
    user_agent VARCHAR(255),
    ip         VARCHAR(255),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT current_timestamp
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

CREATE INDEX embeddings_article_id_idx ON embeddings (article_id);

CREATE INDEX uploaded_images_upload_id_idx ON uploaded_images (upload_id);

CREATE INDEX action_log_user_id_idx ON action_log (user_id);

CREATE INDEX dialogues_user_id_idx ON dialogues (user_id);

CREATE INDEX expenses_category_id_idx ON expenses (category_id);

CREATE INDEX employees_position_id_idx ON employees (position_id);
CREATE INDEX employees_avatar_id_idx ON employees (avatar_id);

CREATE INDEX uploads_uploader_id_idx ON uploads (uploader_id);

CREATE INDEX folders_parent_id_idx ON folders (parent_id);

CREATE INDEX project_stages_project_id_idx ON project_stages (project_id);

CREATE INDEX project_tasks_stage_id_idx ON project_tasks (stage_id);
CREATE INDEX project_tasks_type_id_idx ON project_tasks (type_id);
CREATE INDEX project_tasks_level_id_idx ON project_tasks (level_id);
CREATE INDEX project_tasks_parent_id_idx ON project_tasks (parent_id);

CREATE INDEX estimates_task_id_idx ON estimates (task_id);
CREATE INDEX estimates_employee_id_idx ON estimates (employee_id);

CREATE INDEX payments_customer_id_idx ON payments (customer_id);

CREATE INDEX customers_company_id_idx ON customers (company_id);

CREATE INDEX customer_contacts_customer_id_idx ON customer_contacts (customer_id);

CREATE INDEX vacancies_salary_range_id_idx ON vacancies (id);

CREATE INDEX applicants_vacancy_id_idx ON applicants (vacancy_id);

CREATE INDEX applicant_comments_applicant_id_idx ON applicant_comments (applicant_id);

CREATE INDEX applications_applicant_id_idx ON applications (applicant_id);

CREATE INDEX interview_ratings_interview_id_idx ON interview_ratings (interview_id);
CREATE INDEX interviews_application_id_idx ON interviews (application_id);
CREATE INDEX interviews_interviewer_id_idx ON interviews (interviewer_id);

CREATE INDEX blog_posts_author_id_idx ON blog_posts (author_id);
CREATE INDEX blog_posts_picture_id_idx ON blog_posts (picture_id);

CREATE INDEX blog_post_tag_relations_tag_id_idx ON blog_post_tag_relations (tag_id);

CREATE INDEX blog_comments_post_id_idx ON blog_comments (post_id);

CREATE INDEX blog_likes_post_id_idx ON blog_likes (post_id);

CREATE INDEX website_page_views_page_id_idx ON website_page_views (page_id);

-- +migrate Down
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS money;
DROP TABLE IF EXISTS website_pages;
DROP TABLE IF EXISTS blog_likes;
DROP TABLE IF EXISTS blog_comments;
DROP TABLE IF EXISTS blog_post_tag_relations;
DROP TABLE IF EXISTS blog_post_tags;
DROP TABLE IF EXISTS blog_posts;
DROP TABLE IF EXISTS contact_form_submissions;
DROP TABLE IF EXISTS interview_ratings;
DROP TABLE IF EXISTS interviews;
DROP TABLE IF EXISTS interview_questions;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS applicant_comments;
DROP TABLE IF EXISTS applicants;
DROP TABLE IF EXISTS salary_range;
DROP TABLE IF EXISTS vacancies;
DROP TABLE IF EXISTS authentication_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS dialogues;
DROP TABLE IF EXISTS action_log;
DROP TABLE IF EXISTS uploaded_images;
DROP TABLE IF EXISTS likes;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS folders;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS difficulty_levels;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS task_types;
DROP TABLE IF EXISTS estimates;
DROP TABLE IF EXISTS project_tasks;
DROP TABLE IF EXISTS project_stages;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS customer_contacts;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS employee_meta;
DROP TABLE IF EXISTS employee_contacts;
DROP TABLE IF EXISTS employees;
DROP TABLE IF EXISTS uploads;
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS expense_categories;
DROP TABLE IF EXISTS prompts;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS companies;
DROP TABLE IF EXISTS website_page_views;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS inventory;

DROP EXTENSION IF EXISTS vector;