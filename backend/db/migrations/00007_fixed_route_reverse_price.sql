-- +goose Up
-- Same corridor, different price per direction (demand and driver return legs):
-- price applies origin→destination; reverse_price applies destination→origin when set.
ALTER TABLE fixed_routes ADD COLUMN reverse_price numeric(14,2);

-- +goose Down
ALTER TABLE fixed_routes DROP COLUMN reverse_price;
