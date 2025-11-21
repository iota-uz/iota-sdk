-- +migrate Up
-- Map LLC-like types to LLC
UPDATE counterparty
SET legal_type = 'LLC'
WHERE legal_type IN ('LTD', 'GMBH', 'SARL', 'EURL', 'SRL', 'LTDA', 'BV', 'SPZOO', 'OU', 'UAB')
AND tenant_id IS NOT NULL;

-- Map JSC-like types to JSC
UPDATE counterparty
SET legal_type = 'JSC'
WHERE legal_type IN ('INC', 'PLC', 'CCORP', 'SCORP', 'SA', 'AG', 'AB', 'AS', 'PTYLTD', 'KK', 'SAO')
AND tenant_id IS NOT NULL;

-- Map partnerships and generic to Sole Proprietorship
UPDATE counterparty
SET legal_type = 'SOLE_PROPRIETORSHIP'
WHERE legal_type IN ('SP', 'LLP', 'LLLP', 'SC', 'LEGAL_ENTITY')
AND tenant_id IS NOT NULL;

-- Verify all records have valid types
-- +migrate StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM counterparty
        WHERE legal_type NOT IN ('INDIVIDUAL', 'LLC', 'JSC', 'SOLE_PROPRIETORSHIP')
        AND tenant_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Invalid legal_type values found after migration';
    END IF;
END $$;
-- +migrate StatementEnd

-- +migrate Down
-- Map back from 4 types to original 27 types
-- Note: This is a one-way migration; we cannot perfectly reverse the mapping
-- The following mapping reverses the consolidation:

-- Reverse LLC mappings (use LTD as the base since it's most common)
UPDATE counterparty
SET legal_type = 'LTD'
WHERE legal_type = 'LLC'
AND tenant_id IS NOT NULL;

-- Reverse JSC mappings (use INC as the base)
UPDATE counterparty
SET legal_type = 'INC'
WHERE legal_type = 'JSC'
AND tenant_id IS NOT NULL;

-- Reverse Sole Proprietorship mappings (use SP as the base)
UPDATE counterparty
SET legal_type = 'SP'
WHERE legal_type = 'SOLE_PROPRIETORSHIP'
AND tenant_id IS NOT NULL;
