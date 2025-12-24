-- +migrate Up
-- Add UserUpdateBlockStatus permission
INSERT INTO permissions (id, name, resource, action, modifier, description)
VALUES (
    '6aec630b-be56-4a34-ae65-7958d693ecb9',
    'User.UpdateBlockStatus',
    'user',
    'update',
    'all',
    'Permission to block and unblock users'
)
ON CONFLICT (name) DO NOTHING;

-- +migrate Down
-- Remove UserUpdateBlockStatus permission
DELETE FROM permissions WHERE id = '6aec630b-be56-4a34-ae65-7958d693ecb9';
