package fleet

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kalen-limited/hauler-rider/backend/internal/kernel/apperr"
	"github.com/kalen-limited/hauler-rider/backend/internal/providers/storage"
)

// Onboarding returns the driver's sign-up checklist, creating it on first use.
func (s *Service) Onboarding(ctx context.Context, userID string) (*Onboarding, error) {
	if err := s.store.ensureProfile(ctx, userID); err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}

// SaveProfile stores the driver's name, licence and ID while onboarding.
func (s *Service) SaveProfile(ctx context.Context, userID string, in ProfileInput) (*Onboarding, error) {
	in.FullName, in.LicenceNumber, in.NationalID = strings.TrimSpace(in.FullName), strings.ToUpper(strings.TrimSpace(in.LicenceNumber)), strings.TrimSpace(in.NationalID)
	in.LicenceClass = strings.ToUpper(strings.TrimSpace(in.LicenceClass))
	if err := validateProfile(in); err != nil {
		return nil, err
	}
	if err := s.requireEditable(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.checkLicence(ctx, userID, in); err != nil {
		return nil, err
	}
	if in.AreaID != "" && s.areas != nil {
		if err := s.areas.SetSignupArea(ctx, userID, in.AreaID); err != nil {
			return nil, err
		}
	}
	if err := s.store.saveProfile(ctx, userID, in); err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}

// SaveVehicle registers or edits the driver's vehicle (any edit needs a new review).
func (s *Service) SaveVehicle(ctx context.Context, userID string, in VehicleInput) (*Onboarding, error) {
	in.Plate = normalisePlate(in.Plate)
	if in.Plate == "" || in.ServiceKey == "" || in.ClassKey == "" {
		return nil, fieldError("plate", "REQUIRED")
	}
	ok, err := s.classes.ClassExists(ctx, in.ServiceKey, in.ClassKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrUnknownClass
	}
	if err := s.requireEditable(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := s.store.saveVehicle(ctx, userID, in); err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}

// UploadURL issues a signed URL for one document file.
func (s *Service) UploadURL(ctx context.Context, userID, contentType string) (*UploadTicket, error) {
	ext, ok := storage.AllowedTypes[contentType]
	if !ok {
		return nil, fieldError("content_type", "UNSUPPORTED")
	}
	key := "documents/" + userID + "/" + uuid.NewString() + "." + ext
	url, err := s.files.UploadURL(ctx, key, contentType, uploadTTL)
	if err != nil {
		return nil, err
	}
	return &UploadTicket{FileKey: key, UploadURL: url, ExpiresAt: s.now().Add(uploadTTL)}, nil
}

// AddDocument attaches an uploaded file as the driver's (or their vehicle's) document.
func (s *Service) AddDocument(ctx context.Context, userID string, in DocumentInput) (*Onboarding, error) {
	t, err := s.uploadType(ctx, userID, &in)
	if err != nil {
		return nil, err
	}
	owner := t.Owner
	// The key embeds the uploader: nobody can attach someone else's file.
	if !strings.HasPrefix(in.FileKey, "documents/"+userID+"/") {
		return nil, ErrUploadMissing
	}
	size, err := s.files.Size(ctx, in.FileKey)
	if err != nil {
		return nil, ErrUploadMissing
	}
	in.sizeBytes = size
	if err := s.requireEditable(ctx, userID); err != nil {
		return nil, err
	}
	ownerID := userID
	if owner == OwnerVehicle {
		v, err := s.store.vehicleOf(ctx, userID)
		if err != nil {
			return nil, err
		}
		if v == nil {
			return nil, fieldError("vehicle", "REQUIRED")
		}
		ownerID = v.ID
	}
	if err := s.store.putDocument(ctx, owner, ownerID, in); err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}

// Submit sends a complete application for review.
func (s *Service) Submit(ctx context.Context, userID string) (*Onboarding, error) {
	ob, err := s.assemble(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ob.CanEdit {
		return nil, ErrNotEditable
	}
	if !ob.CanSubmit {
		return nil, ErrIncomplete.WithDetails(map[string]any{"missing": ob.Missing})
	}
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.store.setStatus(ctx, tx, userID, StatusPendingReview, nil, nil); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.assemble(ctx, userID)
}

// assemble loads profile, vehicle and documents and computes the checklist.
func (s *Service) assemble(ctx context.Context, userID string) (*Onboarding, error) {
	p, err := s.store.profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrDriverNotFound
	}
	v, err := s.store.vehicleOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	owners := []string{userID}
	if v != nil {
		owners = append(owners, v.ID)
	}
	docs, err := s.store.documents(ctx, owners)
	if err != nil {
		return nil, err
	}
	types, err := s.DocumentTypes(ctx, deref(p.Country))
	if err != nil {
		return nil, err
	}
	s.withExpiry(types, docs, deref(p.Country))
	reqs, missing := checklist(types, *p, v, docs)
	ob := &Onboarding{Profile: *p, Vehicle: v, Documents: docs, Requirements: reqs, Missing: missing, CanEdit: editable(p.Status)}
	ob.CanSubmit = ob.CanEdit && len(missing) == 0
	if s.countries != nil {
		if c := s.countries(deref(p.Country)); c != nil {
			ob.LicenceClasses, ob.LicencePattern = c.LicenceClasses, c.LicencePattern
		}
	}
	return ob, nil
}

func (s *Service) requireEditable(ctx context.Context, userID string) error {
	if err := s.store.ensureProfile(ctx, userID); err != nil {
		return err
	}
	p, err := s.store.profile(ctx, userID)
	if err != nil {
		return err
	}
	if p == nil || !editable(p.Status) {
		return ErrNotEditable
	}
	return nil
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const minNameLen = 3

func validateProfile(in ProfileInput) error {
	var fields []map[string]string
	if len([]rune(in.FullName)) < minNameLen {
		fields = append(fields, map[string]string{"path": "full_name", "code": "REQUIRED"})
	}
	if in.LicenceNumber == "" {
		fields = append(fields, map[string]string{"path": "licence_number", "code": "REQUIRED"})
	}
	if in.NationalID == "" {
		fields = append(fields, map[string]string{"path": "national_id", "code": "REQUIRED"})
	}
	if in.LicenceExpiry != "" && !datePattern.MatchString(in.LicenceExpiry) {
		fields = append(fields, map[string]string{"path": "licence_expiry", "code": "INVALID"})
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	return nil
}

// normalisePlate: "t 123 abc" → "T123ABC".
func normalisePlate(p string) string {
	return strings.ToUpper(strings.Join(strings.Fields(p), ""))
}

func fieldError(path, code string) error {
	return apperr.Validation.WithDetails(map[string]any{"fields": []map[string]string{{"path": path, "code": code}}})
}

// checkLicence applies the driver's country rules: the number must match the
// country's pattern (10 digits in Tanzania) and the class must be one it lists,
// so registrations are not refused by the regulator later.
func (s *Service) checkLicence(ctx context.Context, userID string, in ProfileInput) error {
	if s.countries == nil {
		return nil
	}
	code, err := s.store.countryOf(ctx, userID)
	if err != nil {
		return err
	}
	c := s.countries(code)
	if c == nil {
		return nil
	}
	var fields []map[string]string
	if !c.LicenceOK(in.LicenceNumber) {
		fields = append(fields, map[string]string{"path": "licence_number", "code": "FORMAT"})
	}
	switch {
	case len(c.LicenceClasses) > 0 && in.LicenceClass == "":
		fields = append(fields, map[string]string{"path": "licence_class", "code": "REQUIRED"})
	case !c.LicenceClassOK(in.LicenceClass):
		fields = append(fields, map[string]string{"path": "licence_class", "code": "UNKNOWN"})
	}
	if len(fields) > 0 {
		return apperr.Validation.WithDetails(map[string]any{"fields": fields})
	}
	return nil
}

// uploadType checks an upload against the driver's country: the kind must be one
// it asks for (not OFF), and the expiry date must follow the type's rules.
func (s *Service) uploadType(ctx context.Context, userID string, in *DocumentInput) (*DocumentType, error) {
	country, err := s.store.countryOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	types, err := s.DocumentTypes(ctx, country)
	if err != nil {
		return nil, err
	}
	t := typeOf(types, in.Kind)
	if t == nil || t.Requirement == RequireOff {
		return nil, ErrUnknownDocument
	}
	if in.ExpiresOn, err = checkExpiry(t, in.ExpiresOn, s.localToday(country)); err != nil {
		return nil, err
	}
	return t, nil
}

// localToday is today's date in a country (documents are valid through their local expiry date).
func (s *Service) localToday(country string) time.Time {
	loc := time.UTC
	if s.countries != nil {
		if c := s.countries(country); c != nil {
			if l, err := time.LoadLocation(c.Timezone); err == nil {
				loc = l
			}
		}
	}
	n := s.now().In(loc)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// withExpiry fills each document's (and renewal's) expiry state from its type.
func (s *Service) withExpiry(types []DocumentType, docs []Document, country string) {
	today := s.localToday(country)
	for i := range docs {
		t := typeOf(types, docs[i].Kind)
		tracked := t != nil && t.TracksExpiry
		docs[i].ExpiryState, docs[i].DaysLeft = ExpiryState(tracked, docs[i].ExpiresOn, today)
		if r := docs[i].Renewal; r != nil {
			r.ExpiryState, r.DaysLeft = ExpiryState(tracked, r.ExpiresOn, today)
		}
	}
}
