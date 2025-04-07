-- +migrate Up

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_tenant_name_key;

ALTER TABLE inventory ADD UNIQUE (tenant_id, name);

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_name_key;

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_tenant_name_key;

ALTER TABLE user_groups ADD UNIQUE (tenant_id, name);

ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tenant_tin_key;

ALTER TABLE counterparty ADD UNIQUE (tenant_id, tin);

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_key;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_name_key;

ALTER TABLE roles ADD UNIQUE (tenant_id, name);

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_hash_key;

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_tenant_hash_key;

ALTER TABLE uploads ADD UNIQUE (tenant_id, hash);

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_phone_key;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_email_key;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_email_key;

ALTER TABLE employees ADD UNIQUE (tenant_id, email);

ALTER TABLE employees ADD UNIQUE (tenant_id, phone);

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_title_key;

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_short_title_key;

ALTER TABLE warehouse_units ADD UNIQUE (tenant_id, title);

ALTER TABLE warehouse_units ADD UNIQUE (tenant_id, short_title);

ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_tenant_account_number_key;

ALTER TABLE money_accounts ADD UNIQUE (tenant_id, account_number);

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_tenant_passport_number_series_key;

ALTER TABLE passports ADD UNIQUE (tenant_id, passport_number, series);

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_email_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_phone_key;

ALTER TABLE users ADD UNIQUE (tenant_id, email);

ALTER TABLE users ADD UNIQUE (tenant_id, phone);

ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_tenant_name_key;

ALTER TABLE positions ADD UNIQUE (tenant_id, name);

ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_tenant_name_key;

ALTER TABLE expense_categories ADD UNIQUE (tenant_id, name);

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_name_key;

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_name_key;

ALTER TABLE permissions ADD UNIQUE (tenant_id, name);

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_phone_number_key;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_email_key;

ALTER TABLE clients ADD UNIQUE (tenant_id, phone_number);

ALTER TABLE clients ADD UNIQUE (tenant_id, email);

ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_barcode_key;

ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_tenant_barcode_key;

ALTER TABLE warehouse_positions ADD UNIQUE (tenant_id, barcode);

ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_rfid_key;

ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_tenant_rfid_key;

ALTER TABLE warehouse_products ADD UNIQUE (tenant_id, rfid);

ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_tenant_name_key;

ALTER TABLE companies ADD UNIQUE (tenant_id, name);

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_href_user_id_key;

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_tenant_href_user_id_key;

ALTER TABLE tabs ADD UNIQUE (tenant_id, href, user_id);


-- +migrate Down

ALTER TABLE tabs DROP CONSTRAINT IF EXISTS tabs_tenant_id_href_user_id_key;

ALTER TABLE tabs ADD CONSTRAINT tabs_tenant_href_user_id_key UNIQUE (tenant_id, href, user_id);

ALTER TABLE tabs ADD UNIQUE (href, user_id);

ALTER TABLE companies DROP CONSTRAINT IF EXISTS companies_tenant_id_name_key;

ALTER TABLE companies ADD CONSTRAINT companies_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE warehouse_products DROP CONSTRAINT IF EXISTS warehouse_products_tenant_id_rfid_key;

ALTER TABLE warehouse_products ADD CONSTRAINT warehouse_products_tenant_rfid_key UNIQUE (tenant_id, rfid);

ALTER TABLE warehouse_products ADD CONSTRAINT warehouse_products_rfid_key UNIQUE (rfid);

ALTER TABLE warehouse_positions DROP CONSTRAINT IF EXISTS warehouse_positions_tenant_id_barcode_key;

ALTER TABLE warehouse_positions ADD CONSTRAINT warehouse_positions_tenant_barcode_key UNIQUE (tenant_id, barcode);

ALTER TABLE warehouse_positions ADD CONSTRAINT warehouse_positions_barcode_key UNIQUE (barcode);

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_id_email_key;

ALTER TABLE clients DROP CONSTRAINT IF EXISTS clients_tenant_id_phone_number_key;

ALTER TABLE clients ADD CONSTRAINT clients_tenant_email_key UNIQUE (tenant_id, email);

ALTER TABLE clients ADD CONSTRAINT clients_tenant_phone_number_key UNIQUE (tenant_id, phone_number);

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS permissions_tenant_id_name_key;

ALTER TABLE permissions ADD CONSTRAINT permissions_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE permissions ADD CONSTRAINT permissions_name_key UNIQUE (name);

ALTER TABLE expense_categories DROP CONSTRAINT IF EXISTS expense_categories_tenant_id_name_key;

ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE positions DROP CONSTRAINT IF EXISTS positions_tenant_id_name_key;

ALTER TABLE positions ADD CONSTRAINT positions_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_phone_key;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_email_key;

ALTER TABLE users ADD CONSTRAINT users_tenant_phone_key UNIQUE (tenant_id, phone);

ALTER TABLE users ADD CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email);

ALTER TABLE users ADD CONSTRAINT users_phone_key UNIQUE (phone);

ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE passports DROP CONSTRAINT IF EXISTS passports_tenant_id_passport_number_series_key;

ALTER TABLE passports
	ADD CONSTRAINT passports_tenant_passport_number_series_key UNIQUE (tenant_id, passport_number, series);

ALTER TABLE money_accounts DROP CONSTRAINT IF EXISTS money_accounts_tenant_id_account_number_key;

ALTER TABLE money_accounts ADD CONSTRAINT money_accounts_tenant_account_number_key UNIQUE (tenant_id, account_number);

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_id_short_title_key;

ALTER TABLE warehouse_units DROP CONSTRAINT IF EXISTS warehouse_units_tenant_id_title_key;

ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_tenant_short_title_key UNIQUE (tenant_id, short_title);

ALTER TABLE warehouse_units ADD CONSTRAINT warehouse_units_tenant_title_key UNIQUE (tenant_id, title);

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_id_phone_key;

ALTER TABLE employees DROP CONSTRAINT IF EXISTS employees_tenant_id_email_key;

ALTER TABLE employees ADD CONSTRAINT employees_tenant_email_key UNIQUE (tenant_id, email);

ALTER TABLE employees ADD CONSTRAINT employees_email_key UNIQUE (email);

ALTER TABLE employees ADD CONSTRAINT employees_tenant_phone_key UNIQUE (tenant_id, phone);

ALTER TABLE uploads DROP CONSTRAINT IF EXISTS uploads_tenant_id_hash_key;

ALTER TABLE uploads ADD CONSTRAINT uploads_tenant_hash_key UNIQUE (tenant_id, hash);

ALTER TABLE uploads ADD CONSTRAINT uploads_hash_key UNIQUE (hash);

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_tenant_id_name_key;

ALTER TABLE roles ADD CONSTRAINT roles_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);

ALTER TABLE counterparty DROP CONSTRAINT IF EXISTS counterparty_tenant_id_tin_key;

ALTER TABLE counterparty ADD CONSTRAINT counterparty_tenant_tin_key UNIQUE (tenant_id, tin);

ALTER TABLE user_groups DROP CONSTRAINT IF EXISTS user_groups_tenant_id_name_key;

ALTER TABLE user_groups ADD CONSTRAINT user_groups_tenant_name_key UNIQUE (tenant_id, name);

ALTER TABLE user_groups ADD CONSTRAINT user_groups_name_key UNIQUE (name);

ALTER TABLE inventory DROP CONSTRAINT IF EXISTS inventory_tenant_id_name_key;

ALTER TABLE inventory ADD CONSTRAINT inventory_tenant_name_key UNIQUE (tenant_id, name);

