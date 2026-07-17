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

const (
	AppointmentPending   = "待确认"
	AppointmentChecked   = "已确认"
	AppointmentWaiting   = "候诊中"
	AppointmentServing   = "处理中"
	AppointmentCompleted = "已完成"
	AppointmentCancelled = "已取消"
	FollowupPending      = "待完成"
	FollowupCompleted    = "已完成"
)

var appointmentTransitions = map[string]map[string]bool{
	AppointmentPending:   {AppointmentChecked: true, AppointmentCancelled: true},
	AppointmentChecked:   {AppointmentWaiting: true, AppointmentCancelled: true},
	AppointmentWaiting:   {AppointmentServing: true, AppointmentCancelled: true},
	AppointmentServing:   {AppointmentCompleted: true},
	AppointmentCompleted: {},
	AppointmentCancelled: {},
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }
