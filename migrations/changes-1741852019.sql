-- +migrate Up
ALTER TABLE passports
DROP CONSTRAINT passports_passport_number_key;

ALTER TABLE passports
ADD CONSTRAINT passports_passport_number_series_key UNIQUE (passport_number, series);

-- +migrate Down
ALTER TABLE passports
DROP CONSTRAINT passports_passport_number_series_key;

ALTER TABLE passports
ADD CONSTRAINT passports_passport_number_key UNIQUE (passport_number);
