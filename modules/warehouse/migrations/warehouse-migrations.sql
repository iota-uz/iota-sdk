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
    rfid        VARCHAR(255) NULL UNIQUE,
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
    warehouse_order_id   INT NOT NULL REFERENCES warehouse_orders (id) ON DELETE CASCADE,
    warehouse_product_id INT NOT NULL REFERENCES warehouse_products (id) ON DELETE CASCADE,
    created_at           TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    PRIMARY KEY (warehouse_order_id, warehouse_product_id)
);

CREATE TABLE inventory_checks
(
    id              SERIAL PRIMARY KEY,
    status          VARCHAR(255) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(255) NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT current_timestamp,
    finished_at     TIMESTAMP WITH TIME ZONE,
    created_by_id   INT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    finished_by_id  INT REFERENCES users (id) ON DELETE CASCADE
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
DROP TABLE IF EXISTS inventory_check_results CASCADE;
DROP TABLE IF EXISTS warehouse_order_items CASCADE;
DROP TABLE IF EXISTS warehouse_orders CASCADE;
DROP TABLE IF EXISTS inventory_checks CASCADE;
DROP TABLE IF EXISTS warehouse_products CASCADE;
DROP TABLE IF EXISTS warehouse_positions CASCADE;
DROP TABLE IF EXISTS warehouse_position_images CASCADE;
DROP TABLE IF EXISTS warehouse_units CASCADE;
