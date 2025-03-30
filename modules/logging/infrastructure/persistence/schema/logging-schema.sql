CREATE TABLE authentication_logs (
    id serial PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id integer NOT NULL CONSTRAINT fk_user_id REFERENCES users (id) ON DELETE CASCADE,
    ip varchar(255) NOT NULL,
    user_agent varchar(255) NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE TABLE action_logs (
    id serial PRIMARY KEY,
    tenant_id int NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    method varchar(255) NOT NULL,
    path varchar(255) NOT NULL,
    user_id int REFERENCES users (id) ON DELETE SET NULL,
    after JSON,
    before JSON,
    user_agent varchar(255) NOT NULL,
    ip varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE INDEX action_logs_tenant_id_idx ON action_logs (tenant_id);

CREATE INDEX action_log_user_id_idx ON action_logs (user_id);

CREATE INDEX authentication_logs_tenant_id_idx ON authentication_logs (tenant_id);

CREATE INDEX authentication_logs_user_id_idx ON authentication_logs (user_id);

CREATE INDEX authentication_logs_created_at_idx ON authentication_logs (created_at);

