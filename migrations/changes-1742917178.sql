-- +migrate Up
ALTER TABLE clients ALTER phone_number DROP NOT NULL;

-- +migrate Down
ALTER TABLE clients ALTER phone_number SET DEFAULT '';
ALTER TABLE clients ALTER phone_number SET NOT NULL;

