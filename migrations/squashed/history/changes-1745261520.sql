-- +migrate Up

-- Change ADD_COLUMN: comments
ALTER TABLE clients ADD COLUMN comments TEXT;


-- +migrate Down

-- Undo ADD_COLUMN: comments
ALTER TABLE clients DROP COLUMN IF EXISTS comments;

