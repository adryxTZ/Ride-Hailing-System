-- +goose Up
-- Pricing history (spec 2026-10-23-pricing-profiles-geofences.md, S3): which
-- profile applied where and when, read from the append-only assignments and
-- events. No new tables: an index for a country's events over a time range,
-- one for "the shape a geofence had at T", and a view naming every assignment.

CREATE INDEX IF NOT EXISTS pricing_profile_events_time_idx ON pricing_profile_events (brand_id, at DESC);
CREATE INDEX IF NOT EXISTS geofence_versions_time_idx ON geofence_versions (geofence_id, created_at);

-- Every assignment, current and closed, with its profile and scope names.
CREATE VIEW pricing_assignment_timeline AS
SELECT a.id AS assignment_id, a.brand_id, a.country_code, a.profile_id, p.name AS profile_name, p.status AS profile_status,
       a.scope_kind, a.scope_id,
       coalesce(CASE a.scope_kind
                  WHEN 'COUNTRY' THEN (SELECT c.name->>'en' FROM countries c WHERE c.brand_id = a.brand_id AND c.code = a.scope_id)
                  WHEN 'AREA' THEN (SELECT ar.name FROM areas ar WHERE ar.id::text = a.scope_id)
                  ELSE (SELECT g.name FROM geofences g WHERE g.id::text = a.scope_id)
                END, a.scope_id) AS scope_name,
       a.priority, a.valid_from, a.valid_to, a.reason, a.close_reason,
       ab.full_name AS assigned_by_name, cb.full_name AS closed_by_name
FROM pricing_profile_assignments a
JOIN pricing_profiles p ON p.id = a.profile_id
LEFT JOIN users ab ON ab.id = a.assigned_by
LEFT JOIN users cb ON cb.id = a.closed_by;

-- +goose Down
DROP VIEW IF EXISTS pricing_assignment_timeline;
DROP INDEX IF EXISTS pricing_profile_events_time_idx;
