-- +migrate Up
ALTER TABLE clients ALTER phone_number DROP NOT NULL;

-- +migrate Down
UPDATE clients SET phone_number = '' WHERE phone_number = NULL; 
ALTER TABLE clients ALTER phone_number SET NOT NULL;
