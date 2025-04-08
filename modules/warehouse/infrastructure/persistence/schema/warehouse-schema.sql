CREATE TABLE warehouse_units (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL, -- Kilogram, Piece, etc.
    short_title varchar(255) NOT NULL, -- kg, pcs, etc.
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now()
    CONSTRAINT warehouse_units_tenant_title_key UNIQUE (tenant_id, title),
    CONSTRAINT warehouse_units_tenant_short_title_key UNIQUE (tenant_id, short_title)
);

CREATE TABLE warehouse_positions (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    title varchar(255) NOT NULL,
    barcode varchar(255) NOT NULL,
    description text,
    unit_id int REFERENCES warehouse_units (id) ON DELETE SET NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT warehouse_positions_tenant_barcode_key UNIQUE (tenant_id, barcode)
);

CREATE TABLE warehouse_position_images (
    warehouse_position_id int NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    upload_id int NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    PRIMARY KEY (upload_id, warehouse_position_id)
);

CREATE TABLE warehouse_products (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    position_id int NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    rfid varchar(255) NULL,
    status varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT warehouse_products_tenant_rfid_key UNIQUE (tenant_id, rfid)
);

CREATE TABLE warehouse_orders (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    type VARCHAR(255) NOT NULL,
    status varchar(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE warehouse_order_items (
    warehouse_order_id int NOT NULL REFERENCES warehouse_orders (id) ON DELETE CASCADE,
    warehouse_product_id int NOT NULL REFERENCES warehouse_products (id) ON DELETE CASCADE,
    PRIMARY KEY (warehouse_order_id, warehouse_product_id)
);

CREATE TABLE inventory_checks (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    status varchar(255) NOT NULL,
    name varchar(255) NOT NULL,
    type VARCHAR(255) NOT NULL,
    created_at timestamp with time zone DEFAULT now(),
    finished_at timestamp with time zone,
    created_by_id int NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    finished_by_id int REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE inventory_check_results (
    id serial PRIMARY KEY,
    tenant_id uuid REFERENCES tenants (id) ON DELETE CASCADE,
    inventory_check_id int NOT NULL REFERENCES inventory_checks (id) ON DELETE CASCADE,
    position_id int NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    expected_quantity int NOT NULL,
    actual_quantity int NOT NULL,
    difference int NOT NULL,
    created_at timestamp with time zone DEFAULT now()
);

CREATE INDEX warehouse_orders_tenant_id_idx ON warehouse_orders (tenant_id);

CREATE INDEX inventory_checks_tenant_id_idx ON inventory_checks (tenant_id);

CREATE INDEX inventory_check_results_tenant_id_idx ON inventory_check_results (tenant_id);
