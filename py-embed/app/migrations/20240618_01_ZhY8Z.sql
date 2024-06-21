-- 
-- depends:
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE embeddings
(
    uuid         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    embedding    VECTOR(512)  NOT NULL,
    meta         JSONB        NOT NULL,
    reference_id VARCHAR(255) NOT NULL,
    text         TEXT         NOT NULL,
    language     VARCHAR(10)  NOT NULL
);

CREATE INDEX embeddings_embedding_idx ON embeddings USING ivfflat (embedding);
CREATE INDEX embeddings_reference_id_idx ON embeddings (reference_id);
CREATE INDEX embeddings_text_idx ON embeddings USING gin (to_tsvector('english', language));
CREATE INDEX embeddings_language_idx ON embeddings (language);
