package fleet

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/brand"
)

// Service is the fleet use cases for one brand.
type Service struct {
	areas DriverAreas // nil until SetAreas
	// countries returns a country's rules (licence pattern and classes); nil = no checks.
	countries func(code string) *brand.Country
	// onApprovedTx runs inside the transaction that approves a driver and vehicle (regulator registration).
	onApprovedTx ApprovedTxHook
	// gate may refuse going online (regulator registration pending); nil = no gate.
	gate RegulatorGate
	// docTypes caches each country's document types.
	docTypes docTypeCache
	// onDocuments runs after a driver's documents changed (expiry holds are re-evaluated).
	onDocuments func(ctx context.Context, driverID string)
	store       store
	classes     Classes
	files       Files
	offline     Offline
	now         func() time.Time
}

// Upload and view links live this long.
const (
	uploadTTL = 15 * time.Minute
	viewTTL   = 10 * time.Minute
)

// New wires the service; offline may be nil (no presence).
func New(pool *pgxpool.Pool, brandID string, c Classes, f Files, off Offline) *Service {
	return &Service{store: store{pool: pool, brandID: brandID}, classes: c, files: f, offline: off, now: time.Now}
}

// ApprovedTxHook runs inside the approving transaction with the driver's country.
type ApprovedTxHook func(ctx context.Context, tx pgx.Tx, country, driverID, vehicleID string) error

// RegulatorGate refuses going online until the regulator has accepted the driver
// and vehicle, when the country runs fail-closed.
type RegulatorGate func(ctx context.Context, country, driverID, vehicleID string) error

// OnApprovedTx registers the approval hook.
func (s *Service) OnApprovedTx(h ApprovedTxHook) { s.onApprovedTx = h }

// SetRegulatorGate registers the going-online gate.
func (s *Service) SetRegulatorGate(g RegulatorGate) { s.gate = g }

// approvedTx runs the approval hook for a driver.
func (s *Service) approvedTx(ctx context.Context, tx pgx.Tx, driverID, vehicleID string) error {
	if s.onApprovedTx == nil {
		return nil
	}
	country, err := s.store.countryOf(ctx, driverID)
	if err != nil {
		return err
	}
	return s.onApprovedTx(ctx, tx, country, driverID, vehicleID)
}

// OnDocumentsChanged registers what runs after a driver's documents change (approval of a renewal, staff edits).
func (s *Service) OnDocumentsChanged(f func(ctx context.Context, driverID string)) { s.onDocuments = f }

// SetCountries plugs in the brand's country rules (licence number pattern and classes).
func (s *Service) SetCountries(f func(code string) *brand.Country) { s.countries = f }

// SetAreas plugs in the operating-areas module (eligibility and sign-up area).
func (s *Service) SetAreas(a DriverAreas) { s.areas = a }
