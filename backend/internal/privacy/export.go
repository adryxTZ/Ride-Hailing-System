package privacy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
)

// exportSections are the parts of a person's export: each query returns one
// JSON value for the user ($1). Only rows about this person are read; a
// driver's trips leave out the stops (those are the riders' places).
var exportSections = []struct{ name, sql string }{
	{"profile", `SELECT jsonb_build_object('id', id, 'phone', phone, 'email', email, 'full_name', full_name, 'locale', locale,
		'status', status, 'created_at', created_at) FROM users WHERE id=$1`},
	{"consents", `SELECT coalesce(jsonb_agg(jsonb_build_object('app', app, 'version', version, 'accepted_at', accepted_at) ORDER BY accepted_at), '[]')
		FROM user_consents WHERE user_id=$1`},
	{"saved_places", `SELECT coalesce(jsonb_agg(jsonb_build_object('label', label, 'kind', kind, 'address', address,
		'lat', ST_Y(location::geometry), 'lng', ST_X(location::geometry), 'created_at', created_at)), '[]') FROM saved_places WHERE user_id=$1`},
	{"trusted_contacts", `SELECT coalesce(jsonb_agg(jsonb_build_object('name', name, 'phone', phone, 'created_at', created_at)), '[]')
		FROM trusted_contacts WHERE user_id=$1`},
	{"trips", `SELECT coalesce(jsonb_agg(jsonb_build_object('code', t.code, 'role', CASE WHEN t.rider_id=$1 THEN 'RIDER' ELSE 'DRIVER' END,
		'service', t.service_key, 'vehicle_class', t.vehicle_class_key, 'status', t.status, 'requested_at', t.requested_at, 'completed_at', t.completed_at,
		'fare', coalesce(t.final_fare, t.estimated_fare), 'currency', t.currency, 'payment_method', t.payment_method,
		'stops', CASE WHEN t.rider_id<>$1 THEN NULL ELSE (SELECT jsonb_agg(jsonb_build_object('kind', s.kind, 'address', s.address, 'lat', ST_Y(s.location::geometry), 'lng', ST_X(s.location::geometry)) ORDER BY s.seq)
		          FROM trip_stops s WHERE s.trip_id=t.id) END) ORDER BY t.requested_at), '[]')
		FROM trips t WHERE t.rider_id=$1 OR t.driver_id=$1`},
	{"payments", `SELECT coalesce(jsonb_agg(jsonb_build_object('purpose', purpose, 'amount', amount, 'currency', currency, 'gateway', gateway,
		'status', status, 'created_at', created_at) ORDER BY created_at), '[]') FROM gateway_transactions WHERE payer_id=$1`},
	{"receipts", `SELECT coalesce(jsonb_agg(jsonb_build_object('number', number, 'issued_at', issued_at, 'total', total, 'currency', currency) ORDER BY issued_at), '[]')
		FROM receipts WHERE rider_id=$1 OR driver_id=$1`},
	{"ratings_given", `SELECT coalesce(jsonb_agg(jsonb_build_object('score', score, 'comment', comment, 'aspects', aspects, 'created_at', created_at)), '[]')
		FROM trip_ratings WHERE rater_id=$1`},
	{"ratings_received", `SELECT coalesce(jsonb_agg(jsonb_build_object('score', score, 'aspects', aspects, 'created_at', created_at)), '[]')
		FROM trip_ratings WHERE ratee_id=$1`},
	{"messages_sent", `SELECT coalesce(jsonb_agg(jsonb_build_object('body', body, 'created_at', created_at) ORDER BY created_at), '[]')
		FROM trip_messages WHERE sender_id=$1`},
	{"documents", `SELECT coalesce(jsonb_agg(jsonb_build_object('kind', kind, 'status', status, 'expires_on', expires_on, 'created_at', created_at)), '[]')
		FROM documents WHERE owner_id=$1`},
	{"holds", `SELECT coalesce(jsonb_agg(jsonb_build_object('kind', kind, 'reason', reason, 'placed_at', placed_at, 'released_at', released_at)), '[]')
		FROM driver_holds WHERE driver_id=$1`},
	{"payouts", `SELECT coalesce(jsonb_agg(jsonb_build_object('amount', amount, 'currency', currency, 'status', status, 'created_at', created_at)), '[]')
		FROM driver_payouts WHERE driver_id=$1`},
}

// exportData builds the export document for a person.
func (s *Service) exportData(ctx context.Context, userID string) (map[string]json.RawMessage, error) {
	out := map[string]json.RawMessage{}
	for _, sec := range exportSections {
		var raw []byte
		if err := s.store.pool.QueryRow(ctx, sec.sql, userID).Scan(&raw); err != nil {
			return nil, fmt.Errorf("export %s: %w", sec.name, err)
		}
		out[sec.name] = raw
	}
	return out, nil
}

// BuildExport writes the file for an approved export, marks it READY for
// ExportLinkTTL and texts the person (the River job).
func (s *Service) BuildExport(ctx context.Context, requestID string) error {
	r, err := s.store.request(ctx, nil, requestID, false)
	if err != nil {
		return err
	}
	if r.Status != StatusInProgress {
		return nil // cancelled or already built
	}
	data, err := s.exportData(ctx, r.UserID)
	if err != nil {
		return err
	}
	doc, err := json.MarshalIndent(map[string]any{"generated_at": s.now(), "brand": s.d.Brand.DisplayName, "data": data}, "", "  ")
	if err != nil {
		return err
	}
	key := fmt.Sprintf("exports/%s/%s.json", r.UserID, uuid.NewString())
	if err := s.d.Writer.Put(ctx, key, bytes.NewReader(doc), ExportMaxBytes); err != nil {
		return err
	}
	expires := s.now().Add(ExportLinkTTL)
	if err := s.store.markReady(ctx, requestID, key, expires); err != nil {
		return err
	}
	if s.d.Notify != nil && r.UserPhone != "" {
		body := fmt.Sprintf("%s: your data is ready to download in the app until %s.", s.d.Brand.DisplayName, expires.In(s.d.Location).Format("2 Jan 15:04"))
		if err := s.d.Notify(ctx, r.UserPhone, body); err != nil {
			slog.Warn("privacy: export SMS", "err", err)
		}
	}
	return nil
}
