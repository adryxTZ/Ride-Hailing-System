package fleet

import "context"

// Eligibility returns what the driver may go online with, or why not.
// The vehicle decides the class; the driver serves every service offering that
// class (a Boda gets rides and packages). The app cannot claim others.
func (s *Service) Eligibility(ctx context.Context, userID string) (*Eligibility, error) {
	p, err := s.store.profile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.Status != StatusApproved {
		return nil, ErrDriverNotApproved
	}
	v, err := s.store.vehicleOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	if v == nil || v.Status != ReviewApproved {
		return nil, ErrVehicleNotApproved
	}
	if s.gate != nil {
		if err := s.gate(ctx, deref(p.Country), userID, v.ID); err != nil {
			return nil, err
		}
	}
	services, err := s.classes.ServicesForClass(ctx, v.ClassKey)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		services = []string{v.ServiceKey}
	}
	el := &Eligibility{VehicleID: v.ID, Services: services, Classes: []string{v.ClassKey}, Plate: v.Plate, Areas: []string{}}
	if s.areas != nil {
		if el.Areas, err = s.areas.AreaIDs(ctx, userID); err != nil {
			return nil, err
		}
	}
	return el, nil
}

// VehicleOf returns the vehicle a driver works with (nil when none is registered).
func (s *Service) VehicleOf(ctx context.Context, driverID string) (*Vehicle, error) {
	return s.store.vehicleOf(ctx, driverID)
}
