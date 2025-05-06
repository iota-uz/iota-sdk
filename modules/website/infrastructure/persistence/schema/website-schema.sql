CREATE TABLE IF NOT EXISTS ai_chat_configs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid (),
    model_name varchar(255) NOT NULL,
    model_type varchar(50) NOT NULL,
    system_prompt text NOT NULL,
    temperature real NOT NULL DEFAULT 0.7,
    max_tokens integer NOT NULL DEFAULT 1024,
    base_url varchar(255) NOT NULL,
    access_token varchar(1024),
    is_default boolean NOT NULL DEFAULT FALSE,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);

-- Add partial unique index to ensure only one row can have is_default=true
CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_chat_configs_unique_default ON ai_chat_configs (is_default) WHERE (is_default = true);


