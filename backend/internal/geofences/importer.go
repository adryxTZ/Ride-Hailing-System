package geofences

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/kalen-limited/hauler-rider/backend/internal/audit"
	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
)

// ImportInput is a GeoJSON FeatureCollection of one kind for one country.
type ImportInput struct {
	Country  string          `json:"country"`
	Kind     string          `json:"kind"`
	Features json.RawMessage `json:"features"`
	DryRun   bool            `json:"dry_run"`
	Reason   string          `json:"reason"`
}

// ImportResult is what happened (or would happen) to one feature.
type ImportResult struct {
	Code    string `json:"code"`
	Outcome string `json:"outcome"` // CREATE | UPDATE | INVALID
	Error   string `json:"error,omitempty"`
}

type feature struct {
	Properties struct {
		Name    string `json:"name"`
		Code    string `json:"code"`
		RadiusM int    `json:"radius_m"`
	} `json:"properties"`
	Geometry json.RawMessage `json:"geometry"`
}

// errDryRun rolls a dry run's transaction back after every check ran.
var errDryRun = errors.New("geofences: dry run")

// Import checks every feature; a dry run reports per feature, an apply saves
// all valid features in one transaction or none when any is invalid.
func (s *Service) Import(ctx context.Context, staffID string, in ImportInput) ([]ImportResult, error) {
	in.Country, in.Kind = strings.ToUpper(in.Country), strings.ToUpper(in.Kind)
	if len(in.Features) > ImportMaxBytes {
		return nil, ErrImportInvalid.WithDetails(map[string]any{"reason": "too large"})
	}
	features, err := parseFeatures(in.Features)
	if err != nil {
		return nil, err
	}
	if !in.DryRun && strings.TrimSpace(in.Reason) == "" {
		return nil, ErrReason
	}
	var results []ImportResult
	err = pgx.BeginFunc(ctx, s.store.pool, func(tx pgx.Tx) error {
		results = s.importAll(ctx, tx, staffID, in, features)
		if in.DryRun {
			return errDryRun
		}
		for _, r := range results {
			if r.Outcome == "INVALID" {
				return ErrImportInvalid.WithDetails(map[string]any{"results": results})
			}
		}
		if err := audit.Record(ctx, tx, s.store.brandID, staffID, "geofence.import", "geofences:"+in.Country, in.Reason, map[string]any{"count": len(results)}); err != nil {
			return err
		}
		return bump(ctx, tx)
	})
	if errors.Is(err, errDryRun) {
		return results, nil
	}
	return results, err
}

func parseFeatures(raw json.RawMessage) ([]feature, error) {
	var fc struct {
		Features []feature `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, ErrImportInvalid.WithDetails(map[string]any{"reason": "not a FeatureCollection"})
	}
	if len(fc.Features) == 0 || len(fc.Features) > ImportMaxFeatures {
		return nil, ErrImportInvalid.WithDetails(map[string]any{"reason": "1 to 200 features"})
	}
	return fc.Features, nil
}

// importAll saves each feature under a savepoint so one failure does not end the transaction.
func (s *Service) importAll(ctx context.Context, tx pgx.Tx, staffID string, in ImportInput, features []feature) []ImportResult {
	out := make([]ImportResult, 0, len(features))
	for _, f := range features {
		r := ImportResult{Code: f.Properties.Code}
		sp, err := tx.Begin(ctx)
		if err == nil {
			r.Outcome, err = s.importOne(ctx, sp, staffID, in, f)
		}
		if err != nil {
			r.Outcome, r.Error = "INVALID", describe(err)
			_ = sp.Rollback(ctx)
		} else {
			_ = sp.Commit(ctx)
		}
		out = append(out, r)
	}
	return out
}

func (s *Service) importOne(ctx context.Context, tx pgx.Tx, staffID string, in ImportInput, f feature) (string, error) {
	shapeIn := ShapeInput{Geometry: f.Geometry, RadiusM: f.Properties.RadiusM}
	if in.Kind == KindPlace {
		var pt struct {
			Coordinates [2]float64 `json:"coordinates"`
		}
		if err := json.Unmarshal(f.Geometry, &pt); err != nil {
			return "", invalid("WRONG_TYPE", "a place needs a Point")
		}
		shapeIn = ShapeInput{Center: &Point{Lng: pt.Coordinates[0], Lat: pt.Coordinates[1]}, RadiusM: f.Properties.RadiusM}
	}
	c := CreateInput{Country: in.Country, Kind: in.Kind, Code: f.Properties.Code, Name: strings.TrimSpace(f.Properties.Name)}
	if err := checkNaming(c); err != nil {
		return "", err
	}
	sh, err := s.shape(ctx, tx, in.Country, in.Kind, shapeIn)
	if err != nil {
		return "", err
	}
	existing, err := s.store.list(ctx, tx, `g.country_code=$2 AND g.code=$3`, in.Country, c.Code)
	if err != nil {
		return "", err
	}
	if len(existing) == 0 {
		_, err := s.store.insert(ctx, tx, staffID, Geofence{Country: c.Country, Kind: c.Kind, Code: c.Code, Name: c.Name}, sh, in.Reason)
		return "CREATE", err
	}
	if existing[0].Kind != in.Kind {
		return "", invalid("WRONG_TYPE", "that code belongs to another kind")
	}
	if _, err := tx.Exec(ctx, `UPDATE geofences SET name=$2 WHERE id=$1`, existing[0].ID, c.Name); err != nil {
		return "", err
	}
	return "UPDATE", s.store.reshape(ctx, tx, staffID, existing[0], sh, in.Reason)
}

// describe is an error as a short reason for the import table.
func describe(err error) string {
	var ae *apperr.E
	if errors.As(err, &ae) {
		if r, ok := ae.Details["reason"].(string); ok {
			return ae.Code + ": " + r
		}
		return ae.Code
	}
	return err.Error()
}
