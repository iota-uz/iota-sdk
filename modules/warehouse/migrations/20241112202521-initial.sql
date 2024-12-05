-- +migrate Up
CREATE TABLE warehouse_units
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL, -- Kilogram, Piece, etc.
    short_title VARCHAR(255) NOT NULL, -- kg, pcs, etc.
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_positions
(
    id          SERIAL PRIMARY KEY,
    title       VARCHAR(255) NOT NULL,
    barcode     VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    unit_id     INT          REFERENCES warehouse_units (id) ON DELETE SET NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_position_images
(
    warehouse_position_id INT NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    upload_id             INT NOT NULL REFERENCES uploads (id) ON DELETE CASCADE,
    PRIMARY KEY (upload_id, warehouse_position_id)
);

CREATE TABLE warehouse_products
(
    id          SERIAL PRIMARY KEY,
    position_id INT          NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    rfid        VARCHAR(255) NOT NULL UNIQUE,
    status      VARCHAR(255) NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    updated_at  TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_orders
(
    id         SERIAL PRIMARY KEY,
    type       VARCHAR(255) NOT NULL,
    status     VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE warehouse_order_items
(
    warehouse_order_id INT NOT NULL REFERENCES warehouse_orders (id) ON DELETE CASCADE,
    product_id         INT NOT NULL REFERENCES warehouse_products (id) ON DELETE CASCADE,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (warehouse_order_id, product_id)
);

CREATE TABLE inventory_checks
(
    id         SERIAL PRIMARY KEY,
    status     VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

CREATE TABLE inventory_check_results
(
    id                 SERIAL PRIMARY KEY,
    inventory_check_id INT NOT NULL REFERENCES inventory_checks (id) ON DELETE CASCADE,
    position_id        INT NOT NULL REFERENCES warehouse_positions (id) ON DELETE CASCADE,
    expected_quantity  INT NOT NULL,
    actual_quantity    INT NOT NULL,
    difference         INT NOT NULL,
    created_at         TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp
);

-- +migrate Down
DROP TABLE warehouse_order_items;
DROP TABLE warehouse_orders;
DROP TABLE inventory_check_results;
DROP TABLE inventory_checks;
DROP TABLE warehouse_products;
DROP TABLE warehouse_position_images;
DROP TABLE warehouse_positions;
DROP TABLE warehouse_units;
