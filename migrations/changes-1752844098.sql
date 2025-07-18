-- +migrate Up
-- Add GIN index for full text search on clients table
-- Create a computed column for full text search combining first_name, last_name, middle_name, and phone_number

-- Add computed column for full text search
ALTER TABLE clients 
ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('english', coalesce(first_name, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(last_name, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(middle_name, '')), 'B') ||
    setweight(to_tsvector('english', coalesce(phone_number, '')), 'C')
) STORED;

-- Create GIN index for efficient full text search
CREATE INDEX idx_clients_search_vector ON clients USING gin(search_vector);

-- +migrate Down
-- Remove the GIN index and computed column
DROP INDEX IF EXISTS idx_clients_search_vector;
ALTER TABLE clients DROP COLUMN IF EXISTS search_vector;