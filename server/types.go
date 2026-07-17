package main

import "time"

// Appointment is a clinic appointment in the operational workflow.
type Appointment struct {
	ID          string `json:"id"`
	PatientID   string `json:"patientId,omitempty"`
	Patient     string `json:"patient"`
	Department  string `json:"department"`
	Doctor      string `json:"doctor"`
	ScheduledAt string `json:"scheduledAt"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// AppointmentEvent records every state transition for audit and queue replay.
type AppointmentEvent struct {
	ID            string `json:"id"`
	AppointmentID string `json:"appointmentId"`
	FromStatus    string `json:"fromStatus"`
	ToStatus      string `json:"toStatus"`
	Actor         string `json:"actor"`
	CreatedAt     string `json:"createdAt"`
}

// Department is a clinic service line.
type Department struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Doctor is an operational provider profile, not a medical record.
type Doctor struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Department string `json:"department"`
	Status     string `json:"status"`
	TodayCount int    `json:"todayCount"`
}

// Patient contains synthetic identifiers used by the demo workflow.
type Patient struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	LastVisit string `json:"lastVisit"`
}

// Followup is a non-diagnostic operational callback task.
type Followup struct {
	ID        string `json:"id"`
	PatientID string `json:"patientId,omitempty"`
	Patient   string `json:"patient"`
	Summary   string `json:"summary"`
	DueAt     string `json:"dueAt"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateAppointmentInput is accepted by POST /appointments.
type CreateAppointmentInput struct {
	PatientID   string `json:"patientId"`
	Patient     string `json:"patient"`
	Department  string `json:"department"`
	Doctor      string `json:"doctor"`
	ScheduledAt string `json:"scheduledAt"`
}

// UpdateAppointmentStatusInput is accepted by POST /appointments/:id/status.
type UpdateAppointmentStatusInput struct {
	Status string `json:"status" binding:"required"`
	Actor  string `json:"actor"`
}

// CreateFollowupInput is accepted by POST /followups.
type CreateFollowupInput struct {
	PatientID string `json:"patientId"`
	Patient   string `json:"patient"`
	Summary   string `json:"summary"`
	DueAt     string `json:"dueAt"`
}

// Dashboard contains operational KPIs used by admin and mobile clients.
type Dashboard struct {
	TodayAppointments  int `json:"todayAppointments"`
	AverageWaitMinutes int `json:"averageWaitMinutes"`
	Completed          int `json:"completed"`
	CheckedIn          int `json:"checkedIn"`
	PendingFollowups   int `json:"pendingFollowups"`
}

// Matter is a synthetic legal case identified only by an alias.
type Matter struct {
	ID            string           `json:"id"`
	SubjectAlias  string           `json:"subjectAlias"`
	CaseType      string           `json:"caseType"`
	Priority      string           `json:"priority"`
	Deadline      string           `json:"deadline"`
	Assignee      string           `json:"assignee,omitempty"`
	Status        string           `json:"status"`
	ClosureResult string           `json:"closureResult,omitempty"`
	CreatedAt     string           `json:"createdAt"`
	UpdatedAt     string           `json:"updatedAt"`
	Tasks         []MatterTask     `json:"tasks,omitempty"`
	Documents     []MatterDocument `json:"documents,omitempty"`
	Events        []MatterEvent    `json:"events,omitempty"`
}

// MatterTask is the current operational assignment for a matter.
type MatterTask struct {
	ID        string `json:"id"`
	MatterID  string `json:"matterId"`
	Title     string `json:"title"`
	Assignee  string `json:"assignee"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// MatterDocument stores metadata and a client-provided checksum, never file contents.
type MatterDocument struct {
	ID        string `json:"id"`
	MatterID  string `json:"matterId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Checksum  string `json:"checksum"`
	CreatedAt string `json:"createdAt"`
	CreatedBy string `json:"createdBy"`
}

// MatterEvent records a chronological audit entry for the matter timeline.
type MatterEvent struct {
	ID         string `json:"id"`
	MatterID   string `json:"matterId"`
	Action     string `json:"action"`
	FromStatus string `json:"fromStatus,omitempty"`
	ToStatus   string `json:"toStatus,omitempty"`
	Actor      string `json:"actor"`
	Detail     string `json:"detail,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// CreateMatterInput is accepted by POST /matters. It intentionally has no customer identity fields.
type CreateMatterInput struct {
	SubjectAlias string `json:"subjectAlias"`
	CaseType     string `json:"caseType"`
	Priority     string `json:"priority"`
	Deadline     string `json:"deadline"`
}

// AssignMatterInput is accepted by POST /matters/:id/assign.
type AssignMatterInput struct {
	Assignee string `json:"assignee"`
	Actor    string `json:"actor"`
}

// AddMatterFileInput is accepted by POST /matters/:id/file.
type AddMatterFileInput struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Checksum string `json:"checksum"`
}

// UpdateMatterStatusInput is accepted by POST /matters/:id/status.
type UpdateMatterStatusInput struct {
	Status string `json:"status"`
	Actor  string `json:"actor"`
}

// CloseMatterInput is accepted by POST /matters/:id/close.
type CloseMatterInput struct {
	Result string `json:"result"`
	Actor  string `json:"actor"`
}

const (
	AppointmentPending   = "待立案"
	AppointmentChecked   = "已立案"
	AppointmentWaiting   = "待办理"
	AppointmentServing   = "办理中"
	AppointmentCompleted = "已结案"
	AppointmentCancelled = "已撤案"
	FollowupPending      = "待完成"
	FollowupCompleted    = "已完成"
)

const (
	MatterPending       = "待委托"
	MatterFiled         = "已立案"
	MatterCollaborating = "协同中"
	MatterPendingClose  = "待结案"
	MatterClosed        = "已结案"
	MatterTaskPending   = "待处理"
)

var matterTransitions = map[string]map[string]bool{
	MatterPending:       {MatterFiled: true},
	MatterFiled:         {MatterCollaborating: true},
	MatterCollaborating: {MatterPendingClose: true},
	MatterPendingClose:  {MatterClosed: true},
	MatterClosed:        {},
}

var appointmentTransitions = map[string]map[string]bool{
	AppointmentPending:   {AppointmentChecked: true, AppointmentCancelled: true},
	AppointmentChecked:   {AppointmentWaiting: true, AppointmentCancelled: true},
	AppointmentWaiting:   {AppointmentServing: true, AppointmentCancelled: true},
	AppointmentServing:   {AppointmentCompleted: true},
	AppointmentCompleted: {},
	AppointmentCancelled: {},
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }
