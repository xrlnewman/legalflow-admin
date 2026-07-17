package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var (
	ErrNotFound              = errors.New("resource not found")
	ErrInvalidTransition     = errors.New("invalid appointment status transition")
	ErrMissingIdempotencyKey = errors.New("Idempotency-Key is required")
	ErrInvalidInput          = errors.New("invalid input")
	ErrIdempotencyBusy       = errors.New("request with the same Idempotency-Key is in progress")
)

type CareStore interface {
	Dashboard(context.Context) (Dashboard, error)
	ListDepartments(context.Context) ([]Department, error)
	ListDoctors(context.Context) ([]Doctor, error)
	ListPatients(context.Context, int, int) ([]Patient, int, error)
	ListAppointments(context.Context, int, int, string) ([]Appointment, int, error)
	GetAppointment(context.Context, string) (Appointment, error)
	CreateAppointment(context.Context, Appointment) (Appointment, error)
	UpdateAppointmentStatus(context.Context, string, string, string) (Appointment, AppointmentEvent, error)
	ListAppointmentEvents(context.Context, string) ([]AppointmentEvent, error)
	ListFollowups(context.Context, int, int, string) ([]Followup, int, error)
	CreateFollowup(context.Context, Followup) (Followup, error)
	CompleteFollowup(context.Context, string) (Followup, error)
	ListMatters(context.Context, int, int, string, string) ([]Matter, int, error)
	GetMatter(context.Context, string) (Matter, error)
	CreateMatter(context.Context, Matter) (Matter, error)
	AssignMatter(context.Context, string, string, string) (Matter, MatterTask, MatterEvent, error)
	UpdateMatterStatus(context.Context, string, string, string) (Matter, MatterEvent, error)
	AddMatterDocument(context.Context, string, MatterDocument, string) (MatterDocument, MatterEvent, error)
	CloseMatter(context.Context, string, string, string) (Matter, MatterEvent, error)
	ListMatterEvents(context.Context, string) ([]MatterEvent, error)
}

// MemoryStore is deterministic and dependency-free for unit tests and demos.
type MemoryStore struct {
	mu           sync.RWMutex
	seq          atomic.Uint64
	appointments map[string]Appointment
	events       map[string][]AppointmentEvent
	followups    map[string]Followup
	departments  []Department
	doctors      []Doctor
	patients     []Patient
	matters      map[string]Matter
	matterEvents map[string][]MatterEvent
	matterTasks  map[string]MatterTask
	matterDocs   map[string]MatterDocument
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		appointments: map[string]Appointment{}, events: map[string][]AppointmentEvent{}, followups: map[string]Followup{},
		matters: map[string]Matter{}, matterEvents: map[string][]MatterEvent{}, matterTasks: map[string]MatterTask{}, matterDocs: map[string]MatterDocument{},
		departments: []Department{{ID: "practice-contract", Name: "合同纠纷"}, {ID: "practice-labor", Name: "劳动争议"}, {ID: "practice-ip", Name: "知识产权"}, {ID: "practice-corp", Name: "公司治理"}},
		doctors:     []Doctor{{ID: "lawyer-01", Name: "林律师", Department: "合同纠纷", Status: "办理中", TodayCount: 18}, {ID: "lawyer-02", Name: "沈律师", Department: "劳动争议", Status: "办理中", TodayCount: 16}, {ID: "lawyer-03", Name: "赵律师", Department: "知识产权", Status: "办理中", TodayCount: 12}, {ID: "lawyer-04", Name: "周律师", Department: "公司治理", Status: "休息中", TodayCount: 10}, {ID: "lawyer-05", Name: "陈律师", Department: "合同纠纷", Status: "办理中", TodayCount: 14}, {ID: "lawyer-06", Name: "王律师", Department: "劳动争议", Status: "办理中", TodayCount: 16}},
	}
	for i := 1; i <= 30; i++ {
		s.patients = append(s.patients, Patient{ID: fmt.Sprintf("PT-%03d", i), Name: fmt.Sprintf("演示当事人%02d", i), Phone: fmt.Sprintf("1380000%04d", i), LastVisit: "2026-07-15"})
	}
	statuses := []string{AppointmentCompleted, AppointmentServing, AppointmentWaiting, AppointmentChecked, AppointmentPending}
	for i := 1; i <= 20; i++ {
		status := statuses[(i-1)%len(statuses)]
		id := fmt.Sprintf("LG-0716-%03d", 80+i)
		s.appointments[id] = Appointment{ID: id, PatientID: fmt.Sprintf("PT-%03d", i), Patient: s.patients[i-1].Name, Department: s.departments[(i-1)%len(s.departments)].Name, Doctor: s.doctors[(i-1)%len(s.doctors)].Name, ScheduledAt: fmt.Sprintf("2026-07-16T%02d:00:00+08:00", 8+(i%10)), Status: status, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
		if status != AppointmentPending {
			s.events[id] = append(s.events[id], AppointmentEvent{ID: id + "-EV-1", AppointmentID: id, FromStatus: AppointmentPending, ToStatus: status, Actor: "seed", CreatedAt: nowUTC()})
		}
	}
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("LG-0716-%03d", i)
		s.followups[id] = Followup{ID: id, PatientID: fmt.Sprintf("PT-%03d", i), Patient: s.patients[i-1].Name, Summary: "证据材料与合规期限提醒", DueAt: "2026-07-17", Status: FollowupPending, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
	}
	s.seq.Store(1000)
	seedMatters(s)
	return s
}

func seedMatters(s *MemoryStore) {
	statuses := []string{MatterPending, MatterFiled, MatterCollaborating, MatterPendingClose, MatterClosed}
	for i := 1; i <= 18; i++ {
		id := fmt.Sprintf("LF-0720-%03d", i)
		status := statuses[(i-1)%len(statuses)]
		assignee := ""
		if status != MatterPending {
			assignee = []string{"林律师", "沈律师", "赵律师"}[(i-1)%3]
		}
		created := nowUTC()
		m := Matter{ID: id, SubjectAlias: fmt.Sprintf("演示案卷-%03d", i), CaseType: []string{"合同审查", "劳动争议", "知识产权"}[(i-1)%3], Priority: []string{"高", "中", "低"}[(i-1)%3], Deadline: fmt.Sprintf("2026-07-%02d", 20+(i%8)), Assignee: assignee, Status: status, CreatedAt: created, UpdatedAt: created}
		if status == MatterClosed {
			m.ClosureResult = "材料已归档，待客户确认"
		}
		s.matters[id] = m
		if assignee != "" {
			task := MatterTask{ID: id + "-TASK-1", MatterID: id, Title: "核对案件材料与下一个节点", Assignee: assignee, Status: MatterTaskPending, CreatedAt: created, UpdatedAt: created}
			s.matterTasks[id] = task
		}
		if status != MatterPending {
			from := MatterPending
			steps := []string{MatterFiled, MatterCollaborating, MatterPendingClose, MatterClosed}
			for _, to := range steps {
				if status == from {
					break
				}
				e := MatterEvent{ID: id + fmt.Sprintf("-EV-%d", len(s.matterEvents[id])+1), MatterID: id, Action: "推进状态", FromStatus: from, ToStatus: to, Actor: "演示运营", CreatedAt: nowUTC()}
				s.matterEvents[id] = append(s.matterEvents[id], e)
				from = to
			}
		}
	}
}

func (s *MemoryStore) next(prefix string) string { return fmt.Sprintf("%s-%d", prefix, s.seq.Add(1)) }

func (s *MemoryStore) Dashboard(_ context.Context) (Dashboard, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := Dashboard{AverageWaitMinutes: 12}
	for _, a := range s.appointments {
		d.TodayAppointments++
		switch a.Status {
		case AppointmentCompleted:
			d.Completed++
		case AppointmentChecked, AppointmentWaiting, AppointmentServing:
			d.CheckedIn++
		}
	}
	for _, f := range s.followups {
		if f.Status == FollowupPending {
			d.PendingFollowups++
		}
	}
	return d, nil
}
func (s *MemoryStore) ListDepartments(_ context.Context) ([]Department, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Department(nil), s.departments...), nil
}
func (s *MemoryStore) ListDoctors(_ context.Context) ([]Doctor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Doctor(nil), s.doctors...), nil
}
func (s *MemoryStore) ListPatients(_ context.Context, page, pageSize int) ([]Patient, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return paginate(s.patients, page, pageSize)
}
func (s *MemoryStore) ListAppointments(_ context.Context, page, pageSize int, status string) ([]Appointment, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]Appointment, 0, len(s.appointments))
	for _, a := range s.appointments {
		if status == "" || a.Status == status {
			all = append(all, a)
		}
	}
	return paginate(all, page, pageSize)
}
func (s *MemoryStore) GetAppointment(_ context.Context, id string) (Appointment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.appointments[id]
	if !ok {
		return Appointment{}, ErrNotFound
	}
	return a, nil
}
func (s *MemoryStore) CreateAppointment(_ context.Context, a Appointment) (Appointment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.ID == "" {
		a.ID = s.next("AP")
	}
	if a.Status == "" {
		a.Status = AppointmentPending
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowUTC()
	}
	a.UpdatedAt = a.CreatedAt
	s.appointments[a.ID] = a
	return a, nil
}
func (s *MemoryStore) UpdateAppointmentStatus(_ context.Context, id, status, actor string) (Appointment, AppointmentEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.appointments[id]
	if !ok {
		return Appointment{}, AppointmentEvent{}, ErrNotFound
	}
	if !appointmentTransitions[a.Status][status] {
		return Appointment{}, AppointmentEvent{}, ErrInvalidTransition
	}
	old := a.Status
	a.Status = status
	a.UpdatedAt = nowUTC()
	s.appointments[id] = a
	event := AppointmentEvent{ID: s.next("EV"), AppointmentID: id, FromStatus: old, ToStatus: status, Actor: actor, CreatedAt: nowUTC()}
	s.events[id] = append(s.events[id], event)
	return a, event, nil
}
func (s *MemoryStore) ListAppointmentEvents(_ context.Context, id string) ([]AppointmentEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.appointments[id]; !ok {
		return nil, ErrNotFound
	}
	return append([]AppointmentEvent(nil), s.events[id]...), nil
}
func (s *MemoryStore) ListFollowups(_ context.Context, page, pageSize int, status string) ([]Followup, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]Followup, 0, len(s.followups))
	for _, f := range s.followups {
		if status == "" || f.Status == status {
			all = append(all, f)
		}
	}
	return paginate(all, page, pageSize)
}
func (s *MemoryStore) CreateFollowup(_ context.Context, f Followup) (Followup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.ID == "" {
		f.ID = s.next("FW")
	}
	if f.Status == "" {
		f.Status = FollowupPending
	}
	if f.CreatedAt == "" {
		f.CreatedAt = nowUTC()
	}
	f.UpdatedAt = f.CreatedAt
	s.followups[f.ID] = f
	return f, nil
}
func (s *MemoryStore) CompleteFollowup(_ context.Context, id string) (Followup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.followups[id]
	if !ok {
		return Followup{}, ErrNotFound
	}
	if f.Status != FollowupPending {
		return Followup{}, ErrInvalidTransition
	}
	f.Status = FollowupCompleted
	f.UpdatedAt = nowUTC()
	s.followups[id] = f
	return f, nil
}

func (s *MemoryStore) ListMatters(_ context.Context, page, pageSize int, status, assignee string) ([]Matter, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := make([]Matter, 0, len(s.matters))
	for _, matter := range s.matters {
		if status != "" && matter.Status != status {
			continue
		}
		if assignee != "" && matter.Assignee != assignee {
			continue
		}
		all = append(all, matter)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Deadline < all[j].Deadline })
	return paginate(all, page, pageSize)
}

func (s *MemoryStore) GetMatter(_ context.Context, id string) (Matter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	matter, ok := s.matters[id]
	if !ok {
		return Matter{}, ErrNotFound
	}
	matter.Tasks = nil
	if task, ok := s.matterTasks[id]; ok {
		matter.Tasks = []MatterTask{task}
	}
	matter.Documents = make([]MatterDocument, 0)
	for _, doc := range s.matterDocs {
		if doc.MatterID == id {
			matter.Documents = append(matter.Documents, doc)
		}
	}
	matter.Events = append([]MatterEvent(nil), s.matterEvents[id]...)
	sort.SliceStable(matter.Documents, func(i, j int) bool { return matter.Documents[i].CreatedAt < matter.Documents[j].CreatedAt })
	return matter, nil
}

func (s *MemoryStore) CreateMatter(_ context.Context, matter Matter) (Matter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if matter.ID == "" {
		matter.ID = s.next("MT")
	}
	if matter.Status == "" {
		matter.Status = MatterPending
	}
	if matter.CreatedAt == "" {
		matter.CreatedAt = nowUTC()
	}
	matter.UpdatedAt = matter.CreatedAt
	s.matters[matter.ID] = matter
	s.matterEvents[matter.ID] = append(s.matterEvents[matter.ID], MatterEvent{ID: s.next("ME"), MatterID: matter.ID, Action: "创建案件", ToStatus: matter.Status, Actor: "系统", CreatedAt: nowUTC()})
	return matter, nil
}

func (s *MemoryStore) AssignMatter(_ context.Context, id, assignee, actor string) (Matter, MatterTask, MatterEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matter, ok := s.matters[id]
	if !ok {
		return Matter{}, MatterTask{}, MatterEvent{}, ErrNotFound
	}
	if matter.Status == MatterClosed {
		return Matter{}, MatterTask{}, MatterEvent{}, ErrInvalidTransition
	}
	oldStatus := matter.Status
	if matter.Status == MatterPending {
		matter.Status = MatterFiled
	}
	matter.Assignee = assignee
	matter.UpdatedAt = nowUTC()
	s.matters[id] = matter
	task := MatterTask{ID: s.next("MTASK"), MatterID: id, Title: "案件协同负责人", Assignee: assignee, Status: MatterTaskPending, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
	s.matterTasks[id] = task
	event := MatterEvent{ID: s.next("ME"), MatterID: id, Action: "分配负责人", FromStatus: oldStatus, ToStatus: matter.Status, Actor: actor, Detail: assignee, CreatedAt: nowUTC()}
	s.matterEvents[id] = append(s.matterEvents[id], event)
	return matter, task, event, nil
}

func (s *MemoryStore) UpdateMatterStatus(_ context.Context, id, status, actor string) (Matter, MatterEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matter, ok := s.matters[id]
	if !ok {
		return Matter{}, MatterEvent{}, ErrNotFound
	}
	if !matterTransitions[matter.Status][status] {
		return Matter{}, MatterEvent{}, ErrInvalidTransition
	}
	old := matter.Status
	matter.Status = status
	matter.UpdatedAt = nowUTC()
	s.matters[id] = matter
	event := MatterEvent{ID: s.next("ME"), MatterID: id, Action: "推进状态", FromStatus: old, ToStatus: status, Actor: actor, CreatedAt: nowUTC()}
	s.matterEvents[id] = append(s.matterEvents[id], event)
	return matter, event, nil
}

func (s *MemoryStore) AddMatterDocument(_ context.Context, id string, document MatterDocument, actor string) (MatterDocument, MatterEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.matters[id]; !ok {
		return MatterDocument{}, MatterEvent{}, ErrNotFound
	}
	for _, existing := range s.matterDocs {
		if existing.MatterID == id && existing.Checksum == document.Checksum {
			return existing, MatterEvent{}, nil
		}
	}
	document.ID = s.next("DOC")
	document.MatterID = id
	document.CreatedAt = nowUTC()
	document.CreatedBy = actor
	s.matterDocs[document.ID] = document
	event := MatterEvent{ID: s.next("ME"), MatterID: id, Action: "归档文档", Actor: actor, Detail: document.Name, CreatedAt: nowUTC()}
	s.matterEvents[id] = append(s.matterEvents[id], event)
	return document, event, nil
}

func (s *MemoryStore) CloseMatter(_ context.Context, id, result, actor string) (Matter, MatterEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matter, ok := s.matters[id]
	if !ok {
		return Matter{}, MatterEvent{}, ErrNotFound
	}
	if matter.Status != MatterPendingClose {
		return Matter{}, MatterEvent{}, ErrInvalidTransition
	}
	matter.Status = MatterClosed
	matter.ClosureResult = result
	matter.UpdatedAt = nowUTC()
	s.matters[id] = matter
	event := MatterEvent{ID: s.next("ME"), MatterID: id, Action: "提交结案", FromStatus: MatterPendingClose, ToStatus: MatterClosed, Actor: actor, Detail: result, CreatedAt: nowUTC()}
	s.matterEvents[id] = append(s.matterEvents[id], event)
	return matter, event, nil
}

func (s *MemoryStore) ListMatterEvents(_ context.Context, id string) ([]MatterEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.matters[id]; !ok {
		return nil, ErrNotFound
	}
	events := append([]MatterEvent(nil), s.matterEvents[id]...)
	sort.SliceStable(events, func(i, j int) bool { return events[i].CreatedAt < events[j].CreatedAt })
	return events, nil
}

func paginate[T any](all []T, page, pageSize int) ([]T, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	total := len(all)
	start := (page - 1) * pageSize
	if start >= total {
		return []T{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

// SQLStore persists the same workflow in MySQL 8.4. Schema and seed live in deploy/mysql/init.sql.
type SQLStore struct{ db *sql.DB }

func NewSQLStore(ctx context.Context, dsn string) (*SQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLStore{db: db}, nil
}
func (s *SQLStore) Dashboard(ctx context.Context) (Dashboard, error) {
	var d Dashboard
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(status='已结案'),0), COALESCE(SUM(status IN ('已立案','待办理','办理中')),0) FROM appointments`).Scan(&d.TodayAppointments, &d.Completed, &d.CheckedIn)
	if err != nil {
		return d, err
	}
	d.AverageWaitMinutes = 12
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM followups WHERE status='待完成'`).Scan(&d.PendingFollowups); err != nil {
		return d, err
	}
	return d, nil
}
func (s *SQLStore) ListDepartments(ctx context.Context) ([]Department, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name FROM departments ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Department{}
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *SQLStore) ListDoctors(ctx context.Context) ([]Doctor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,department,status,today_count FROM doctors ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Doctor{}
	for rows.Next() {
		var d Doctor
		if err := rows.Scan(&d.ID, &d.Name, &d.Department, &d.Status, &d.TodayCount); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
func (s *SQLStore) ListPatients(ctx context.Context, page, pageSize int) ([]Patient, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM patients`).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,phone,last_visit FROM patients ORDER BY created_at DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Patient{}
	for rows.Next() {
		var p Patient
		if err := rows.Scan(&p.ID, &p.Name, &p.Phone, &p.LastVisit); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}
func (s *SQLStore) ListAppointments(ctx context.Context, page, pageSize int, status string) ([]Appointment, int, error) {
	var total int
	args := []any{}
	count := "SELECT COUNT(*) FROM appointments"
	q := "SELECT id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at FROM appointments"
	if status != "" {
		count += " WHERE status=?"
		q += " WHERE status=?"
		args = append(args, status)
	}
	if err := s.db.QueryRowContext(ctx, count, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	q += " ORDER BY scheduled_at ASC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Appointment{}
	for rows.Next() {
		var a Appointment
		if err := rows.Scan(&a.ID, &a.PatientID, &a.Patient, &a.Department, &a.Doctor, &a.ScheduledAt, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}
func (s *SQLStore) GetAppointment(ctx context.Context, id string) (Appointment, error) {
	var a Appointment
	err := s.db.QueryRowContext(ctx, `SELECT id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at FROM appointments WHERE id=?`, id).Scan(&a.ID, &a.PatientID, &a.Patient, &a.Department, &a.Doctor, &a.ScheduledAt, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Appointment{}, ErrNotFound
	}
	return a, err
}
func (s *SQLStore) CreateAppointment(ctx context.Context, a Appointment) (Appointment, error) {
	if a.ID == "" {
		a.ID = fmt.Sprintf("LG-%d", time.Now().UnixNano())
	}
	if a.Status == "" {
		a.Status = AppointmentPending
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowUTC()
	}
	a.UpdatedAt = a.CreatedAt
	_, err := s.db.ExecContext(ctx, `INSERT INTO appointments (id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, a.ID, a.PatientID, a.Patient, a.Department, a.Doctor, a.ScheduledAt, a.Status, a.CreatedAt, a.UpdatedAt)
	return a, err
}
func (s *SQLStore) UpdateAppointmentStatus(ctx context.Context, id, status, actor string) (Appointment, AppointmentEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Appointment{}, AppointmentEvent{}, err
	}
	defer tx.Rollback()
	var a Appointment
	if err = tx.QueryRowContext(ctx, `SELECT id,patient_id,patient_name,department,doctor,scheduled_at,status,created_at,updated_at FROM appointments WHERE id=? FOR UPDATE`, id).Scan(&a.ID, &a.PatientID, &a.Patient, &a.Department, &a.Doctor, &a.ScheduledAt, &a.Status, &a.CreatedAt, &a.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return Appointment{}, AppointmentEvent{}, ErrNotFound
	} else if err != nil {
		return Appointment{}, AppointmentEvent{}, err
	}
	if !appointmentTransitions[a.Status][status] {
		return Appointment{}, AppointmentEvent{}, ErrInvalidTransition
	}
	old := a.Status
	a.Status = status
	a.UpdatedAt = nowUTC()
	if _, err = tx.ExecContext(ctx, `UPDATE appointments SET status=?,updated_at=? WHERE id=?`, status, a.UpdatedAt, id); err != nil {
		return Appointment{}, AppointmentEvent{}, err
	}
	event := AppointmentEvent{ID: fmt.Sprintf("EV-%d", time.Now().UnixNano()), AppointmentID: id, FromStatus: old, ToStatus: status, Actor: actor, CreatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO appointment_events (id,appointment_id,from_status,to_status,actor,created_at) VALUES (?,?,?,?,?,?)`, event.ID, id, event.FromStatus, event.ToStatus, event.Actor, event.CreatedAt); err != nil {
		return Appointment{}, AppointmentEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return Appointment{}, AppointmentEvent{}, err
	}
	return a, event, nil
}
func (s *SQLStore) ListAppointmentEvents(ctx context.Context, id string) ([]AppointmentEvent, error) {
	if _, err := s.GetAppointment(ctx, id); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,appointment_id,from_status,to_status,actor,created_at FROM appointment_events WHERE appointment_id=? ORDER BY created_at ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AppointmentEvent{}
	for rows.Next() {
		var e AppointmentEvent
		if err := rows.Scan(&e.ID, &e.AppointmentID, &e.FromStatus, &e.ToStatus, &e.Actor, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (s *SQLStore) ListFollowups(ctx context.Context, page, pageSize int, status string) ([]Followup, int, error) {
	var total int
	args := []any{}
	count := "SELECT COUNT(*) FROM followups"
	q := "SELECT id,patient_id,patient_name,summary,due_at,status,created_at,updated_at FROM followups"
	if status != "" {
		count += " WHERE status=?"
		q += " WHERE status=?"
		args = append(args, status)
	}
	if err := s.db.QueryRowContext(ctx, count, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	q += " ORDER BY due_at ASC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Followup{}
	for rows.Next() {
		var f Followup
		if err := rows.Scan(&f.ID, &f.PatientID, &f.Patient, &f.Summary, &f.DueAt, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, f)
	}
	return out, total, rows.Err()
}
func (s *SQLStore) CreateFollowup(ctx context.Context, f Followup) (Followup, error) {
	if f.ID == "" {
		f.ID = fmt.Sprintf("FW-%d", time.Now().UnixNano())
	}
	if f.Status == "" {
		f.Status = FollowupPending
	}
	if f.CreatedAt == "" {
		f.CreatedAt = nowUTC()
	}
	f.UpdatedAt = f.CreatedAt
	_, err := s.db.ExecContext(ctx, `INSERT INTO followups (id,patient_id,patient_name,summary,due_at,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)`, f.ID, f.PatientID, f.Patient, f.Summary, f.DueAt, f.Status, f.CreatedAt, f.UpdatedAt)
	return f, err
}
func (s *SQLStore) CompleteFollowup(ctx context.Context, id string) (Followup, error) {
	var f Followup
	err := s.db.QueryRowContext(ctx, `SELECT id,patient_id,patient_name,summary,due_at,status,created_at,updated_at FROM followups WHERE id=?`, id).Scan(&f.ID, &f.PatientID, &f.Patient, &f.Summary, &f.DueAt, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Followup{}, ErrNotFound
	}
	if err != nil {
		return Followup{}, err
	}
	if f.Status != FollowupPending {
		return Followup{}, ErrInvalidTransition
	}
	f.Status = FollowupCompleted
	f.UpdatedAt = nowUTC()
	_, err = s.db.ExecContext(ctx, `UPDATE followups SET status=?,updated_at=? WHERE id=?`, f.Status, f.UpdatedAt, id)
	return f, err
}

func (s *SQLStore) ListMatters(ctx context.Context, page, pageSize int, status, assignee string) ([]Matter, int, error) {
	args := []any{}
	where := []string{}
	if status != "" {
		where = append(where, "status=?")
		args = append(args, status)
	}
	if assignee != "" {
		where = append(where, "assignee=?")
		args = append(args, assignee)
	}
	condition := ""
	if len(where) > 0 {
		condition = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM matters"+condition, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page, pageSize = normalizePage(page, pageSize)
	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := s.db.QueryContext(ctx, "SELECT id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at FROM matters"+condition+" ORDER BY deadline ASC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Matter{}
	for rows.Next() {
		var m Matter
		if err := rows.Scan(&m.ID, &m.SubjectAlias, &m.CaseType, &m.Priority, &m.Deadline, &m.Assignee, &m.Status, &m.ClosureResult, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	return out, total, rows.Err()
}

func (s *SQLStore) GetMatter(ctx context.Context, id string) (Matter, error) {
	var m Matter
	err := s.db.QueryRowContext(ctx, `SELECT id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at FROM matters WHERE id=?`, id).Scan(&m.ID, &m.SubjectAlias, &m.CaseType, &m.Priority, &m.Deadline, &m.Assignee, &m.Status, &m.ClosureResult, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Matter{}, ErrNotFound
	}
	if err != nil {
		return Matter{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,matter_id,title,assignee,status,created_at,updated_at FROM matter_tasks WHERE matter_id=? ORDER BY created_at DESC`, id)
	if err != nil {
		return Matter{}, err
	}
	for rows.Next() {
		var task MatterTask
		if err := rows.Scan(&task.ID, &task.MatterID, &task.Title, &task.Assignee, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			rows.Close()
			return Matter{}, err
		}
		m.Tasks = append(m.Tasks, task)
	}
	if err := rows.Close(); err != nil {
		return Matter{}, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id,matter_id,name,kind,checksum,created_at,created_by FROM matter_documents WHERE matter_id=? ORDER BY created_at ASC`, id)
	if err != nil {
		return Matter{}, err
	}
	for rows.Next() {
		var doc MatterDocument
		if err := rows.Scan(&doc.ID, &doc.MatterID, &doc.Name, &doc.Kind, &doc.Checksum, &doc.CreatedAt, &doc.CreatedBy); err != nil {
			rows.Close()
			return Matter{}, err
		}
		m.Documents = append(m.Documents, doc)
	}
	if err := rows.Close(); err != nil {
		return Matter{}, err
	}
	m.Events, err = s.ListMatterEvents(ctx, id)
	if err != nil {
		return Matter{}, err
	}
	return m, nil
}

func (s *SQLStore) CreateMatter(ctx context.Context, m Matter) (Matter, error) {
	if m.ID == "" {
		m.ID = fmt.Sprintf("MT-%d", time.Now().UnixNano())
	}
	if m.Status == "" {
		m.Status = MatterPending
	}
	if m.CreatedAt == "" {
		m.CreatedAt = nowUTC()
	}
	m.UpdatedAt = m.CreatedAt
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Matter{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO matters (id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, m.ID, m.SubjectAlias, m.CaseType, m.Priority, m.Deadline, m.Assignee, m.Status, m.ClosureResult, m.CreatedAt, m.UpdatedAt); err != nil {
		return Matter{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_events (id,matter_id,action,from_status,to_status,actor,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`, fmt.Sprintf("ME-%d", time.Now().UnixNano()), m.ID, "创建案件", "", m.Status, "系统", "", nowUTC()); err != nil {
		return Matter{}, err
	}
	if err = tx.Commit(); err != nil {
		return Matter{}, err
	}
	return m, nil
}

func (s *SQLStore) AssignMatter(ctx context.Context, id, assignee, actor string) (Matter, MatterTask, MatterEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	defer tx.Rollback()
	var m Matter
	if err = tx.QueryRowContext(ctx, `SELECT id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at FROM matters WHERE id=? FOR UPDATE`, id).Scan(&m.ID, &m.SubjectAlias, &m.CaseType, &m.Priority, &m.Deadline, &m.Assignee, &m.Status, &m.ClosureResult, &m.CreatedAt, &m.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return Matter{}, MatterTask{}, MatterEvent{}, ErrNotFound
	} else if err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	if m.Status == MatterClosed {
		return Matter{}, MatterTask{}, MatterEvent{}, ErrInvalidTransition
	}
	old := m.Status
	if m.Status == MatterPending {
		m.Status = MatterFiled
	}
	m.Assignee = assignee
	m.UpdatedAt = nowUTC()
	if _, err = tx.ExecContext(ctx, `UPDATE matters SET assignee=?,status=?,updated_at=? WHERE id=?`, m.Assignee, m.Status, m.UpdatedAt, id); err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	task := MatterTask{ID: fmt.Sprintf("MTASK-%d", time.Now().UnixNano()), MatterID: id, Title: "案件协同负责人", Assignee: assignee, Status: MatterTaskPending, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_tasks (id,matter_id,title,assignee,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE assignee=VALUES(assignee),updated_at=VALUES(updated_at)`, task.ID, id, task.Title, task.Assignee, task.Status, task.CreatedAt, task.UpdatedAt); err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	e := MatterEvent{ID: fmt.Sprintf("ME-%d", time.Now().UnixNano()), MatterID: id, Action: "分配负责人", FromStatus: old, ToStatus: m.Status, Actor: actor, Detail: assignee, CreatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_events (id,matter_id,action,from_status,to_status,actor,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`, e.ID, id, e.Action, e.FromStatus, e.ToStatus, e.Actor, e.Detail, e.CreatedAt); err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return Matter{}, MatterTask{}, MatterEvent{}, err
	}
	return m, task, e, nil
}

func (s *SQLStore) UpdateMatterStatus(ctx context.Context, id, status, actor string) (Matter, MatterEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	defer tx.Rollback()
	var m Matter
	if err = tx.QueryRowContext(ctx, `SELECT id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at FROM matters WHERE id=? FOR UPDATE`, id).Scan(&m.ID, &m.SubjectAlias, &m.CaseType, &m.Priority, &m.Deadline, &m.Assignee, &m.Status, &m.ClosureResult, &m.CreatedAt, &m.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return Matter{}, MatterEvent{}, ErrNotFound
	} else if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	if !matterTransitions[m.Status][status] {
		return Matter{}, MatterEvent{}, ErrInvalidTransition
	}
	old := m.Status
	m.Status = status
	m.UpdatedAt = nowUTC()
	if _, err = tx.ExecContext(ctx, `UPDATE matters SET status=?,updated_at=? WHERE id=?`, status, m.UpdatedAt, id); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	e := MatterEvent{ID: fmt.Sprintf("ME-%d", time.Now().UnixNano()), MatterID: id, Action: "推进状态", FromStatus: old, ToStatus: status, Actor: actor, CreatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_events (id,matter_id,action,from_status,to_status,actor,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`, e.ID, id, e.Action, e.FromStatus, e.ToStatus, e.Actor, "", e.CreatedAt); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	return m, e, nil
}

func (s *SQLStore) AddMatterDocument(ctx context.Context, id string, doc MatterDocument, actor string) (MatterDocument, MatterEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MatterDocument{}, MatterEvent{}, err
	}
	defer tx.Rollback()
	var exists string
	err = tx.QueryRowContext(ctx, `SELECT id FROM matters WHERE id=? FOR UPDATE`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return MatterDocument{}, MatterEvent{}, ErrNotFound
	} else if err != nil {
		return MatterDocument{}, MatterEvent{}, err
	}
	var existing MatterDocument
	err = tx.QueryRowContext(ctx, `SELECT id,matter_id,name,kind,checksum,created_at,created_by FROM matter_documents WHERE matter_id=? AND checksum=?`, id, doc.Checksum).Scan(&existing.ID, &existing.MatterID, &existing.Name, &existing.Kind, &existing.Checksum, &existing.CreatedAt, &existing.CreatedBy)
	if err == nil {
		_ = tx.Rollback()
		return existing, MatterEvent{}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return MatterDocument{}, MatterEvent{}, err
	}
	doc.ID = fmt.Sprintf("DOC-%d", time.Now().UnixNano())
	doc.MatterID = id
	doc.CreatedAt = nowUTC()
	doc.CreatedBy = actor
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_documents (id,matter_id,name,kind,checksum,created_at,created_by) VALUES (?,?,?,?,?,?,?)`, doc.ID, id, doc.Name, doc.Kind, doc.Checksum, doc.CreatedAt, doc.CreatedBy); err != nil {
		return MatterDocument{}, MatterEvent{}, err
	}
	e := MatterEvent{ID: fmt.Sprintf("ME-%d", time.Now().UnixNano()), MatterID: id, Action: "归档文档", Actor: actor, Detail: doc.Name, CreatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_events (id,matter_id,action,from_status,to_status,actor,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`, e.ID, id, e.Action, "", "", e.Actor, e.Detail, e.CreatedAt); err != nil {
		return MatterDocument{}, MatterEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return MatterDocument{}, MatterEvent{}, err
	}
	return doc, e, nil
}

func (s *SQLStore) CloseMatter(ctx context.Context, id, result, actor string) (Matter, MatterEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	defer tx.Rollback()
	var m Matter
	if err = tx.QueryRowContext(ctx, `SELECT id,subject_alias,case_type,priority,deadline,assignee,status,closure_result,created_at,updated_at FROM matters WHERE id=? FOR UPDATE`, id).Scan(&m.ID, &m.SubjectAlias, &m.CaseType, &m.Priority, &m.Deadline, &m.Assignee, &m.Status, &m.ClosureResult, &m.CreatedAt, &m.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return Matter{}, MatterEvent{}, ErrNotFound
	} else if err != nil {
		return Matter{}, MatterEvent{}, err
	}
	if m.Status != MatterPendingClose {
		return Matter{}, MatterEvent{}, ErrInvalidTransition
	}
	m.Status = MatterClosed
	m.ClosureResult = result
	m.UpdatedAt = nowUTC()
	if _, err = tx.ExecContext(ctx, `UPDATE matters SET status=?,closure_result=?,updated_at=? WHERE id=?`, m.Status, m.ClosureResult, m.UpdatedAt, id); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	e := MatterEvent{ID: fmt.Sprintf("ME-%d", time.Now().UnixNano()), MatterID: id, Action: "提交结案", FromStatus: MatterPendingClose, ToStatus: MatterClosed, Actor: actor, Detail: result, CreatedAt: nowUTC()}
	if _, err = tx.ExecContext(ctx, `INSERT INTO matter_events (id,matter_id,action,from_status,to_status,actor,detail,created_at) VALUES (?,?,?,?,?,?,?,?)`, e.ID, id, e.Action, e.FromStatus, e.ToStatus, e.Actor, e.Detail, e.CreatedAt); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	if err = tx.Commit(); err != nil {
		return Matter{}, MatterEvent{}, err
	}
	return m, e, nil
}

func (s *SQLStore) ListMatterEvents(ctx context.Context, id string) ([]MatterEvent, error) {
	var exists string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM matters WHERE id=?`, id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,matter_id,action,from_status,to_status,actor,detail,created_at FROM matter_events WHERE matter_id=? ORDER BY created_at ASC,id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MatterEvent{}
	for rows.Next() {
		var e MatterEvent
		if err := rows.Scan(&e.ID, &e.MatterID, &e.Action, &e.FromStatus, &e.ToStatus, &e.Actor, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// NewStoreFromEnv selects MySQL when MYSQL_DSN is configured and otherwise uses the seedable memory store.
func NewStoreFromEnv(ctx context.Context) (CareStore, func() error, error) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		return NewMemoryStore(), func() error { return nil }, nil
	}
	store, err := NewSQLStore(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	return store, store.db.Close, nil
}

// idempotencyStore is intentionally tiny: Redis is the production implementation, memory keeps tests hermetic.
type idempotencyStore interface {
	Get(context.Context, string) (string, bool, error)
	Set(context.Context, string, string, time.Duration) error
	Lock(context.Context, string, time.Duration) (func(), error)
}
type NoopIdempotency struct{}

var noopIdempotencyValues sync.Map

func (n NoopIdempotency) Get(_ context.Context, key string) (string, bool, error) {
	v, ok := noopIdempotencyValues.Load(key)
	if !ok {
		return "", false, nil
	}
	return v.(string), true, nil
}
func (n NoopIdempotency) Set(_ context.Context, key, value string, _ time.Duration) error {
	noopIdempotencyValues.Store(key, value)
	return nil
}
func (n NoopIdempotency) Lock(_ context.Context, _ string, _ time.Duration) (func(), error) {
	return func() {}, nil
}

// memoryIdempotency is used by tests so duplicate writes return the original resource.
type memoryIdempotency struct {
	mu     sync.Mutex
	values map[string]string
}

func newMemoryIdempotency() *memoryIdempotency {
	return &memoryIdempotency{values: map[string]string{}}
}
func (m *memoryIdempotency) Get(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	return v, ok, nil
}
func (m *memoryIdempotency) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}
func (m *memoryIdempotency) Lock(_ context.Context, _ string, _ time.Duration) (func(), error) {
	return func() {}, nil
}

func parseInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
