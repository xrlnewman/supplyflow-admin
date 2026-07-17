package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
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
}

func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		appointments: map[string]Appointment{}, events: map[string][]AppointmentEvent{}, followups: map[string]Followup{},
		departments: []Department{{ID: "dep-general", Name: "全科门诊"}, {ID: "dep-derma", Name: "皮肤科"}, {ID: "dep-rehab", Name: "康复理疗"}, {ID: "dep-nutrition", Name: "营养咨询"}},
		doctors:     []Doctor{{ID: "doc-01", Name: "林负责人", Department: "全科门诊", Status: "出诊中", TodayCount: 18}, {ID: "doc-02", Name: "沈负责人", Department: "皮肤科", Status: "出诊中", TodayCount: 16}, {ID: "doc-03", Name: "赵负责人", Department: "康复理疗", Status: "出诊中", TodayCount: 12}, {ID: "doc-04", Name: "周负责人", Department: "营养咨询", Status: "休息中", TodayCount: 10}, {ID: "doc-05", Name: "陈负责人", Department: "全科门诊", Status: "出诊中", TodayCount: 14}, {ID: "doc-06", Name: "王负责人", Department: "皮肤科", Status: "出诊中", TodayCount: 16}},
	}
	for i := 1; i <= 30; i++ {
		s.patients = append(s.patients, Patient{ID: fmt.Sprintf("PT-%03d", i), Name: fmt.Sprintf("演示客户%02d", i), Phone: fmt.Sprintf("1380000%04d", i), LastVisit: "2026-07-15"})
	}
	statuses := []string{AppointmentCompleted, AppointmentServing, AppointmentWaiting, AppointmentChecked, AppointmentPending}
	for i := 1; i <= 20; i++ {
		status := statuses[(i-1)%len(statuses)]
		id := fmt.Sprintf("AP-0716-%03d", 80+i)
		s.appointments[id] = Appointment{ID: id, PatientID: fmt.Sprintf("PT-%03d", i), Patient: s.patients[i-1].Name, Department: s.departments[(i-1)%len(s.departments)].Name, Doctor: s.doctors[(i-1)%len(s.doctors)].Name, ScheduledAt: fmt.Sprintf("2026-07-16T%02d:00:00+08:00", 8+(i%10)), Status: status, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
		if status != AppointmentPending {
			s.events[id] = append(s.events[id], AppointmentEvent{ID: id + "-EV-1", AppointmentID: id, FromStatus: AppointmentPending, ToStatus: status, Actor: "seed", CreatedAt: nowUTC()})
		}
	}
	for i := 1; i <= 12; i++ {
		id := fmt.Sprintf("FW-0716-%03d", i)
		s.followups[id] = Followup{ID: id, PatientID: fmt.Sprintf("PT-%03d", i), Patient: s.patients[i-1].Name, Summary: "复诊提醒与满意度回访", DueAt: "2026-07-17", Status: FollowupPending, CreatedAt: nowUTC(), UpdatedAt: nowUTC()}
	}
	s.seq.Store(1000)
	return s
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
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(status='已完成'),0), COALESCE(SUM(status IN ('已确认','候诊中','处理中')),0) FROM appointments`).Scan(&d.TodayAppointments, &d.Completed, &d.CheckedIn)
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
		a.ID = fmt.Sprintf("AP-%d", time.Now().UnixNano())
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
