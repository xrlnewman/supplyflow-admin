package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CareService owns validation, idempotency and lifecycle rules for SupplyFlow.
type CareService struct {
	store CareStore
	idem  idempotencyStore
}

func NewCareService(store CareStore, idem idempotencyStore) *CareService {
	return &CareService{store: store, idem: idem}
}

func (s *CareService) CreateAppointment(ctx context.Context, input CreateAppointmentInput, key string) (Appointment, error) {
	if strings.TrimSpace(key) == "" {
		return Appointment{}, ErrMissingIdempotencyKey
	}
	if strings.TrimSpace(input.Patient) == "" && strings.TrimSpace(input.PatientID) == "" {
		return Appointment{}, fmt.Errorf("%w: patient is required", ErrInvalidInput)
	}
	resourceKey := "appointment:create:" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Appointment{}, err
	} else if ok {
		return s.store.GetAppointment(ctx, existing)
	}
	release, err := s.idem.Lock(ctx, "appointment:create-lock", 10*time.Second)
	if err != nil {
		return Appointment{}, err
	}
	defer release()
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Appointment{}, err
	} else if ok {
		return s.store.GetAppointment(ctx, existing)
	}
	a, err := s.store.CreateAppointment(ctx, Appointment{PatientID: input.PatientID, Patient: input.Patient, Department: input.Department, Doctor: input.Doctor, ScheduledAt: input.ScheduledAt, Status: AppointmentPending})
	if err != nil {
		return Appointment{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, a.ID, 24*time.Hour); err != nil {
		return Appointment{}, err
	}
	return a, nil
}

func (s *CareService) CheckinAppointment(ctx context.Context, id, key string) (Appointment, error) {
	return s.UpdateAppointmentStatus(ctx, id, AppointmentChecked, "前台", key)
}

func (s *CareService) CreateFollowup(ctx context.Context, input CreateFollowupInput, key string) (Followup, error) {
	if strings.TrimSpace(key) == "" {
		return Followup{}, ErrMissingIdempotencyKey
	}
	if strings.TrimSpace(input.Patient) == "" && strings.TrimSpace(input.PatientID) == "" {
		return Followup{}, fmt.Errorf("%w: patient is required", ErrInvalidInput)
	}
	resourceKey := "followup:create:" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Followup{}, err
	} else if ok {
		return findFollowup(ctx, s.store, existing)
	}
	release, err := s.idem.Lock(ctx, "followup:create-lock", 10*time.Second)
	if err != nil {
		return Followup{}, err
	}
	defer release()
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Followup{}, err
	} else if ok {
		return findFollowup(ctx, s.store, existing)
	}
	f, err := s.store.CreateFollowup(ctx, Followup{PatientID: input.PatientID, Patient: input.Patient, Summary: input.Summary, DueAt: input.DueAt, Status: FollowupPending})
	if err != nil {
		return Followup{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, f.ID, 24*time.Hour); err != nil {
		return Followup{}, err
	}
	return f, nil
}

func (s *CareService) UpdateAppointmentStatus(ctx context.Context, id, status string, args ...string) (Appointment, error) {
	actor, key := "运营人员", ""
	if len(args) == 1 {
		key = args[0]
	}
	if len(args) >= 2 {
		actor, key = args[0], args[1]
	}
	if strings.TrimSpace(key) == "" {
		return Appointment{}, ErrMissingIdempotencyKey
	}
	status = strings.TrimSpace(status)
	if !validAppointmentStatus(status) {
		return Appointment{}, fmt.Errorf("%w: unknown status", ErrInvalidInput)
	}
	resourceKey := "appointment:status:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Appointment{}, err
	} else if ok {
		return s.store.GetAppointment(ctx, existing)
	}
	release, err := s.idem.Lock(ctx, "appointment:status-lock:"+id, 10*time.Second)
	if err != nil {
		return Appointment{}, err
	}
	defer release()
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Appointment{}, err
	} else if ok {
		return s.store.GetAppointment(ctx, existing)
	}
	if actor == "" {
		actor = "运营人员"
	}
	a, _, err := s.store.UpdateAppointmentStatus(ctx, id, status, actor)
	if err != nil {
		return Appointment{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, a.ID, 24*time.Hour); err != nil {
		return Appointment{}, err
	}
	return a, nil
}

func (s *CareService) CompleteFollowup(ctx context.Context, id, key string) (Followup, error) {
	if strings.TrimSpace(key) == "" {
		return Followup{}, ErrMissingIdempotencyKey
	}
	resourceKey := "followup:complete:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Followup{}, err
	} else if ok {
		return findFollowup(ctx, s.store, existing)
	}
	release, err := s.idem.Lock(ctx, "followup:complete-lock:"+id, 10*time.Second)
	if err != nil {
		return Followup{}, err
	}
	defer release()
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Followup{}, err
	} else if ok {
		return findFollowup(ctx, s.store, existing)
	}
	f, err := s.store.CompleteFollowup(ctx, id)
	if err != nil {
		return Followup{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, f.ID, 24*time.Hour); err != nil {
		return Followup{}, err
	}
	return f, nil
}

func findFollowup(ctx context.Context, store CareStore, id string) (Followup, error) {
	list, _, err := store.ListFollowups(ctx, 1, 100, "")
	if err != nil {
		return Followup{}, err
	}
	for _, f := range list {
		if f.ID == id {
			return f, nil
		}
	}
	return Followup{}, ErrNotFound
}

func validAppointmentStatus(status string) bool {
	switch status {
	case AppointmentPending, AppointmentChecked, AppointmentWaiting, AppointmentServing, AppointmentCompleted, AppointmentCancelled:
		return true
	}
	return false
}

func httpStatusForError(err error) int {
	switch {
	case errors.Is(err, ErrMissingIdempotencyKey), errors.Is(err, ErrInvalidInput):
		return 400
	case errors.Is(err, ErrNotFound):
		return 404
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrIdempotencyBusy), errors.Is(err, ErrSupplyInvalidTransition):
		return 409
	default:
		return 500
	}
}
