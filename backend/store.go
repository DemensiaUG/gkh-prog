package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Service struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Kind        string  `json:"kind"`
	AccountNo   string  `json:"account_no"`
	Debt        float64 `json:"debt"`
	DeadlineDay int     `json:"deadline_day"`
	HasMeter    bool    `json:"has_meter"`
}

type ServiceStatus struct {
	Service
	NextDeadline string `json:"next_deadline"`
	DaysLeft     int    `json:"days_left"`
	Reminder     bool   `json:"reminder"` // осталось 2 дня и меньше
}

type Reading struct {
	ID          int64    `json:"id"`
	ServiceID   int64    `json:"service_id"`
	Service     string   `json:"service"`
	Value       float64  `json:"value"`
	PrevValue   *float64 `json:"prev_value"`
	Consumption *float64 `json:"consumption"`
	Period      string   `json:"period"`
	CreatedAt   string   `json:"created_at"`
}

type Accrual struct {
	ID        int64   `json:"id"`
	ServiceID int64   `json:"service_id"`
	Service   string  `json:"service"`
	Month     string  `json:"month"`
	Amount    float64 `json:"amount"`
}

type Payment struct {
	ID        int64   `json:"id"`
	ServiceID int64   `json:"service_id"`
	Service   string  `json:"service"`
	Amount    float64 `json:"amount"`
	PaidOn    string  `json:"paid_on"`
	Note      string  `json:"note"`
}

type Receipt struct {
	ID         int64  `json:"id"`
	ServiceID  *int64 `json:"service_id"`
	Service    string `json:"service"`
	Title      string `json:"title"`
	Month      string `json:"month"`
	Filename   string `json:"filename"`
	UploadedAt string `json:"uploaded_at"`
}

type MonthStat struct {
	Month string  `json:"month"`
	Total float64 `json:"total"`
}

type Summary struct {
	TotalDebt float64         `json:"total_debt"`
	Services  []ServiceStatus `json:"services"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Init() error {
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	if _, err := s.db.Exec(string(schema)); err != nil {
		return fmt.Errorf("exec schema: %w", err)
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return fmt.Errorf("create uploads: %w", err)
	}

	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM services`).Scan(&count)
	if count == 0 {
		seed := []Service{
			{Name: "Управляющая компания", Kind: "квартплата", DeadlineDay: 10},
			{Name: "Электроэнергия", Kind: "свет", DeadlineDay: 25, HasMeter: true},
			{Name: "Холодная вода", Kind: "вода", DeadlineDay: 25, HasMeter: true},
			{Name: "Интернет", Kind: "интернет", DeadlineDay: 5},
		}
		for _, sv := range seed {
			s.db.Exec(`INSERT INTO services (name, kind, deadline_day, has_meter)
			           VALUES (?, ?, ?, ?)`, sv.Name, sv.Kind, sv.DeadlineDay, sv.HasMeter)
		}
	}
	return nil
}

// ---------- Службы и дедлайны ----------

func (s *Store) ListServices() ([]Service, error) {
	rows, err := s.db.Query(
		`SELECT id, name, kind, account_no, debt, deadline_day, has_meter
		 FROM services ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		var sv Service
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.Kind, &sv.AccountNo,
			&sv.Debt, &sv.DeadlineDay, &sv.HasMeter); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) CreateService(name, kind, accountNo string,
	deadlineDay int, hasMeter bool) (Service, error) {

	res, err := s.db.Exec(
		`INSERT INTO services (name, kind, account_no, deadline_day, has_meter)
		 VALUES (?, ?, ?, ?, ?)`, name, kind, accountNo, deadlineDay, hasMeter)
	if err != nil {
		return Service{}, err
	}
	id, _ := res.LastInsertId()
	return Service{ID: id, Name: name, Kind: kind, AccountNo: accountNo,
		DeadlineDay: deadlineDay, HasMeter: hasMeter}, nil
}

func (s *Store) DeleteService(id int64) error {
	res, err := s.db.Exec(`DELETE FROM services WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Summary считает общий долг и до каждого ближайшего дедлайна.
func (s *Store) Summary() (Summary, error) {
	svcs, err := s.ListServices()
	if err != nil {
		return Summary{}, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	sum := Summary{Services: []ServiceStatus{}}
	for _, sv := range svcs {
		y, m, _ := now.Date()
		deadline := time.Date(y, m, sv.DeadlineDay, 0, 0, 0, 0, now.Location())
		if deadline.Before(today) {
			deadline = deadline.AddDate(0, 1, 0) // день месяца уже прошёл — берём следующий
		}
		days := int(deadline.Sub(today).Hours() / 24)
		sum.TotalDebt += sv.Debt
		sum.Services = append(sum.Services, ServiceStatus{
			Service:      sv,
			NextDeadline: deadline.Format("2006-01-02"),
			DaysLeft:     days,
			Reminder:     days <= 2,
		})
	}
	return sum, nil
}

// ---------- Начисления и оплаты ----------

func (s *Store) AddAccrual(serviceID int64, month string, amount float64) (Accrual, error) {
	if _, err := time.Parse("2006-01", month); err != nil {
		return Accrual{}, fmt.Errorf("month must be YYYY-MM")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Accrual{}, err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRow(`SELECT name FROM services WHERE id = ?`, serviceID).Scan(&name); err != nil {
		return Accrual{}, ErrNotFound
	}
	if _, err := tx.Exec(
		`INSERT INTO accruals (service_id, month, amount) VALUES (?, ?, ?)`,
		serviceID, month, amount); err != nil {
		return Accrual{}, err
	}
	if _, err := tx.Exec(
		`UPDATE services SET debt = debt + ? WHERE id = ?`, amount, serviceID); err != nil {
		return Accrual{}, err
	}
	if err := tx.Commit(); err != nil {
		return Accrual{}, err
	}
	return Accrual{ServiceID: serviceID, Service: name, Month: month, Amount: amount}, nil
}

func (s *Store) AddPayment(serviceID int64, amount float64, paidOn, note string) (Payment, error) {
	if _, err := time.Parse("2006-01-02", paidOn); err != nil {
		return Payment{}, fmt.Errorf("paid_on must be YYYY-MM-DD")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Payment{}, err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRow(`SELECT name FROM services WHERE id = ?`, serviceID).Scan(&name); err != nil {
		return Payment{}, ErrNotFound
	}
	if _, err := tx.Exec(
		`INSERT INTO payments (service_id, amount, paid_on, note) VALUES (?, ?, ?, ?)`,
		serviceID, amount, paidOn, note); err != nil {
		return Payment{}, err
	}
	if _, err := tx.Exec(
		`UPDATE services SET debt = debt - ? WHERE id = ?`, amount, serviceID); err != nil {
		return Payment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Payment{}, err
	}
	return Payment{ServiceID: serviceID, Service: name, Amount: amount, PaidOn: paidOn, Note: note}, nil
}

// ---------- Показания ----------

func (s *Store) getReading(id int64) (Reading, error) {
	var r Reading
	err := s.db.QueryRow(
		`SELECT r.id, r.service_id, s.name, r.value, r.period, r.created_at
		 FROM readings r JOIN services s ON s.id = r.service_id WHERE r.id = ?`, id,
	).Scan(&r.ID, &r.ServiceID, &r.Service, &r.Value, &r.Period, &r.CreatedAt)
	if err != nil {
		return r, err
	}

	var pv sql.NullFloat64
	err = s.db.QueryRow(
		`SELECT value FROM readings WHERE service_id = ? AND id < ? ORDER BY id DESC LIMIT 1`,
		r.ServiceID, id,
	).Scan(&pv)
	switch {
	case err == nil:
		r.PrevValue = &pv.Float64
		c := r.Value - pv.Float64
		r.Consumption = &c
	case errors.Is(err, sql.ErrNoRows):
		// первое показание — расхода ещё нет
	default:
		return r, err
	}
	return r, nil
}

func (s *Store) CreateReading(serviceID int64, value float64, period string) (Reading, error) {
	if _, err := time.Parse("2006-01", period); err != nil {
		return Reading{}, fmt.Errorf("period must be YYYY-MM")
	}
	var exists int
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM services WHERE id = ? AND has_meter = 1`, serviceID,
	).Scan(&exists); err != nil {
		return Reading{}, err
	}
	if exists == 0 {
		return Reading{}, ErrNotFound
	}

	res, err := s.db.Exec(
		`INSERT INTO readings (service_id, value, period) VALUES (?, ?, ?)`,
		serviceID, value, period)
	if err != nil {
		return Reading{}, err
	}
	id, _ := res.LastInsertId()
	return s.getReading(id)
}

func (s *Store) ListReadings(serviceID int64, limit int) ([]Reading, error) {
	rows, err := s.db.Query(
		`SELECT id FROM readings WHERE service_id = ? ORDER BY id DESC LIMIT ?`,
		serviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Reading{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		r, err := s.getReading(id)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- Статистика за 6 месяцев ----------

func (s *Store) MonthlyStats() ([]MonthStat, error) {
	now := time.Now()
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	start := first.AddDate(0, -5, 0) // начало 6-месячного окна

	rows, err := s.db.Query(
		`SELECT strftime('%Y-%m', paid_on) AS m, SUM(amount)
		 FROM payments WHERE paid_on >= ? GROUP BY m`, start.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := map[string]float64{}
	for rows.Next() {
		var m string
		var t float64
		if err := rows.Scan(&m, &t); err != nil {
			return nil, err
		}
		totals[m] = t
	}

	out := []MonthStat{}
	for i := 0; i < 6; i++ {
		m := start.AddDate(0, i, 0).Format("2006-01")
		out = append(out, MonthStat{Month: m, Total: totals[m]})
	}
	return out, rows.Err()
}

// ---------- Чеки ----------

func (s *Store) CreateReceipt(serviceID *int64, title, month, filename string) (Receipt, error) {
	if _, err := time.Parse("2006-01", month); err != nil {
		return Receipt{}, fmt.Errorf("month must be YYYY-MM")
	}
	var svc string
	if serviceID != nil {
		s.db.QueryRow(`SELECT name FROM services WHERE id = ?`, *serviceID).Scan(&svc)
	}
	res, err := s.db.Exec(
		`INSERT INTO receipts (service_id, title, month, filename) VALUES (?, ?, ?, ?)`,
		serviceID, title, month, filename)
	if err != nil {
		return Receipt{}, err
	}
	id, _ := res.LastInsertId()
	return Receipt{ID: id, ServiceID: serviceID, Service: svc,
		Title: title, Month: month, Filename: filename,
		UploadedAt: time.Now().Format("2006-01-02 15:04")}, nil
}

func (s *Store) ListReceipts(month string) ([]Receipt, error) {
	q := `SELECT r.id, r.service_id, COALESCE(s.name, ''), r.title, r.month,
	             r.filename, r.uploaded_at
	      FROM receipts r LEFT JOIN services s ON s.id = r.service_id`
	args := []any{}
	if month != "" {
		q += ` WHERE r.month = ?`
		args = append(args, month)
	}
	q += ` ORDER BY r.id DESC LIMIT 50`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Receipt{}
	for rows.Next() {
		var r Receipt
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.Service, &r.Title,
			&r.Month, &r.Filename, &r.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteReceipt(id int64) (string, error) {
	var filename string
	if err := s.db.QueryRow(
		`SELECT filename FROM receipts WHERE id = ?`, id).Scan(&filename); err != nil {
		return "", ErrNotFound
	}
	if _, err := s.db.Exec(`DELETE FROM receipts WHERE id = ?`, id); err != nil {
		return "", err
	}
	return filename, nil
}
