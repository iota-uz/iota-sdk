-- name: select
SELECT wp.id,
       wp.title,
       wp.barcode,
       wp.unit_id,
       wp.created_at,
       wp.updated_at,
       wu.id,
       wu.title,
       wu.short_title,
       wu.created_at,
       wu.updated_at
FROM warehouse_positions wp
         LEFT JOIN warehouse_units wu ON wp.unit_id = wu.id;

-- name: select_id_only
SELECT id
FROM warehouse_positions;

-- name: count
SELECT COUNT(*) as count
FROM warehouse_positions;

-- name: insert
INSERT INTO warehouse_positions (title, barcode, unit_id)
VALUES ($1, $2, $3)
RETURNING id;

-- name: insert_image
INSERT INTO warehouse_position_images (warehouse_position_id, upload_id)
VALUES ($1, $2);

-- name: update
UPDATE warehouse_positions wp
SET title   = COALESCE(NULLIF($1, ''), wp.title),
    barcode = COALESCE(NULLIF($2, ''), wp.barcode),
    unit_id = COALESCE(NULLIF($3, 0), wp.unit_id)
WHERE id = $4;

-- name: delete
DELETE
FROM warehouse_positions
WHERE id = $1;

-- name: delete_images
DELETE
from warehouse_position_images
WHERE warehouse_position_id = $1
