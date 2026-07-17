package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var plainChineseName = regexp.MustCompile(`^[\p{Han}]{2,4}$`)
var personalIdentifier = regexp.MustCompile(`^(?:1[3-9]\d{9}|\d{15}|\d{17}[\dXx])$`)

// CareService owns validation, idempotency and lifecycle rules for LegalFlow.
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

func (s *CareService) CreateMatter(ctx context.Context, input CreateMatterInput, key string) (Matter, error) {
	if strings.TrimSpace(key) == "" {
		return Matter{}, ErrMissingIdempotencyKey
	}
	input.SubjectAlias = strings.TrimSpace(input.SubjectAlias)
	input.CaseType = strings.TrimSpace(input.CaseType)
	input.Priority = strings.TrimSpace(input.Priority)
	input.Deadline = strings.TrimSpace(input.Deadline)
	if input.SubjectAlias == "" || plainChineseName.MatchString(input.SubjectAlias) || personalIdentifier.MatchString(input.SubjectAlias) {
		return Matter{}, fmt.Errorf("%w: subjectAlias must be an alias", ErrInvalidInput)
	}
	if input.CaseType == "" || input.Priority == "" || input.Deadline == "" {
		return Matter{}, fmt.Errorf("%w: caseType, priority and deadline are required", ErrInvalidInput)
	}
	if !validDeadline(input.Deadline) {
		return Matter{}, fmt.Errorf("%w: deadline must be a valid date", ErrInvalidInput)
	}
	resourceKey := "matter:create:" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Matter{}, err
	} else if ok {
		return s.store.GetMatter(ctx, existing)
	}
	release, err := s.idem.Lock(ctx, "matter:create-lock", 10*time.Second)
	if err != nil {
		return Matter{}, err
	}
	defer release()
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Matter{}, err
	} else if ok {
		return s.store.GetMatter(ctx, existing)
	}
	m, err := s.store.CreateMatter(ctx, Matter{SubjectAlias: input.SubjectAlias, CaseType: input.CaseType, Priority: input.Priority, Deadline: input.Deadline, Status: MatterPending})
	if err != nil {
		return Matter{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, m.ID, 24*time.Hour); err != nil {
		return Matter{}, err
	}
	return m, nil
}

func (s *CareService) AssignMatter(ctx context.Context, id string, input AssignMatterInput, key string) (Matter, MatterTask, error) {
	if strings.TrimSpace(key) == "" {
		return Matter{}, MatterTask{}, ErrMissingIdempotencyKey
	}
	input.Assignee = strings.TrimSpace(input.Assignee)
	input.Actor = strings.TrimSpace(input.Actor)
	if input.Assignee == "" {
		return Matter{}, MatterTask{}, fmt.Errorf("%w: assignee is required", ErrInvalidInput)
	}
	if input.Actor == "" {
		input.Actor = "运营人员"
	}
	resourceKey := "matter:assign:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Matter{}, MatterTask{}, err
	} else if ok {
		m, err := s.store.GetMatter(ctx, existing)
		if err != nil {
			return Matter{}, MatterTask{}, err
		}
		if len(m.Tasks) == 0 {
			return Matter{}, MatterTask{}, ErrNotFound
		}
		return m, m.Tasks[0], nil
	}
	release, err := s.idem.Lock(ctx, "matter:lock:"+id, 10*time.Second)
	if err != nil {
		return Matter{}, MatterTask{}, err
	}
	defer release()
	m, task, _, err := s.store.AssignMatter(ctx, id, input.Assignee, input.Actor)
	if err != nil {
		return Matter{}, MatterTask{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, m.ID, 24*time.Hour); err != nil {
		return Matter{}, MatterTask{}, err
	}
	return m, task, nil
}

func (s *CareService) UpdateMatterStatus(ctx context.Context, id, status, actor, key string) (Matter, error) {
	if strings.TrimSpace(key) == "" {
		return Matter{}, ErrMissingIdempotencyKey
	}
	status = strings.TrimSpace(status)
	if !validMatterStatus(status) {
		return Matter{}, fmt.Errorf("%w: unknown matter status", ErrInvalidInput)
	}
	if strings.TrimSpace(actor) == "" {
		actor = "运营人员"
	}
	resourceKey := "matter:status:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Matter{}, err
	} else if ok {
		return s.store.GetMatter(ctx, existing)
	}
	release, err := s.idem.Lock(ctx, "matter:lock:"+id, 10*time.Second)
	if err != nil {
		return Matter{}, err
	}
	defer release()
	m, _, err := s.store.UpdateMatterStatus(ctx, id, status, actor)
	if err != nil {
		return Matter{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, m.ID, 24*time.Hour); err != nil {
		return Matter{}, err
	}
	return m, nil
}

func (s *CareService) AddMatterDocument(ctx context.Context, id string, input AddMatterFileInput, key string) (MatterDocument, error) {
	if strings.TrimSpace(key) == "" {
		return MatterDocument{}, ErrMissingIdempotencyKey
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Checksum = strings.TrimSpace(input.Checksum)
	if input.Name == "" || input.Kind == "" || input.Checksum == "" {
		return MatterDocument{}, fmt.Errorf("%w: name, kind and checksum are required", ErrInvalidInput)
	}
	resourceKey := "matter:file:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return MatterDocument{}, err
	} else if ok {
		return findMatterDocument(ctx, s.store, existing)
	}
	release, err := s.idem.Lock(ctx, "matter:lock:"+id, 10*time.Second)
	if err != nil {
		return MatterDocument{}, err
	}
	defer release()
	doc, _, err := s.store.AddMatterDocument(ctx, id, MatterDocument{Name: input.Name, Kind: input.Kind, Checksum: input.Checksum}, "运营人员")
	if err != nil {
		return MatterDocument{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, doc.ID, 24*time.Hour); err != nil {
		return MatterDocument{}, err
	}
	return doc, nil
}

func (s *CareService) CloseMatter(ctx context.Context, id string, input CloseMatterInput, key string) (Matter, MatterEvent, error) {
	if strings.TrimSpace(key) == "" {
		return Matter{}, MatterEvent{}, ErrMissingIdempotencyKey
	}
	input.Result = strings.TrimSpace(input.Result)
	input.Actor = strings.TrimSpace(input.Actor)
	if input.Result == "" {
		return Matter{}, MatterEvent{}, fmt.Errorf("%w: result is required", ErrInvalidInput)
	}
	if input.Actor == "" {
		input.Actor = "运营人员"
	}
	resourceKey := "matter:close:" + id + ":" + key
	if existing, ok, err := s.idem.Get(ctx, resourceKey); err != nil {
		return Matter{}, MatterEvent{}, err
	} else if ok {
		m, err := s.store.GetMatter(ctx, existing)
		return m, MatterEvent{}, err
	}
	release, err := s.idem.Lock(ctx, "matter:lock:"+id, 10*time.Second)
	if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	defer release()
	m, event, err := s.store.CloseMatter(ctx, id, input.Result, input.Actor)
	if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	if err := s.idem.Set(ctx, resourceKey, m.ID, 24*time.Hour); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	return m, event, nil
}

func findMatterDocument(ctx context.Context, store CareStore, id string) (MatterDocument, error) {
	matter, err := store.GetMatter(ctx, id)
	if err != nil {
		return MatterDocument{}, err
	}
	for _, doc := range matter.Documents {
		if doc.ID == id {
			return doc, nil
		}
	}
	return MatterDocument{}, ErrNotFound
}

func validMatterStatus(status string) bool {
	switch status {
	case MatterPending, MatterFiled, MatterCollaborating, MatterPendingClose, MatterClosed:
		return true
	}
	return false
}

func validDeadline(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.After(time.Now().Add(-24 * time.Hour))
		}
	}
	return false
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
	case errors.Is(err, ErrInvalidTransition), errors.Is(err, ErrIdempotencyBusy):
		return 409
	default:
		return 500
	}
}
