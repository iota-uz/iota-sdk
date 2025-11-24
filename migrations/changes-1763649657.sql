-- +migrate Up
ALTER TABLE expense_categories
ADD COLUMN is_cogs BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX idx_expense_categories_cogs
ON expense_categories(is_cogs)
WHERE is_cogs = TRUE;

COMMENT ON COLUMN expense_categories.is_cogs IS
'Flag indicating if expenses in this category are Cost of Goods Sold (direct production costs)';

-- +migrate Down
DROP INDEX IF EXISTS idx_expense_categories_cogs;
ALTER TABLE expense_categories
DROP COLUMN is_cogs;
