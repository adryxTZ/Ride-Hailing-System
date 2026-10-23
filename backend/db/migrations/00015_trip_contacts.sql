-- +goose Up
-- People on the trip other than the account holder: who is picked up / hands
-- over the goods (pickup) and who receives them (dropoff): {"pickup":{"name","phone"},"dropoff":{…}}.
ALTER TABLE trips ADD COLUMN contacts jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE trips DROP COLUMN contacts;
