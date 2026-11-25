package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Пути настраиваются переменными окружения (нужно для Docker),
// по умолчанию — как раньше, в рабочую папку.
var (
	dbPath     = "gkh.db"
	uploadsDir = "uploads"
)

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func idFrom(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func parseOptionalID(s string) *int64 {
	if s == "" {
		return nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &id
}

func main() {
	if v := os.Getenv("GKH_DB"); v != "" {
		dbPath = v
	}
	if v := os.Getenv("GKH_UPLOADS"); v != "" {
		uploadsDir = v
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		log.Fatal(err)
	}
	store := NewStore(db)
	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	// --- Службы ---
	mux.HandleFunc("GET /api/services", func(w http.ResponseWriter, r *http.Request) {
		list, err := store.ListServices()
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, list)
	})

	mux.HandleFunc("POST /api/services", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string `json:"name"`
			Kind        string `json:"kind"`
			AccountNo   string `json:"account_no"`
			DeadlineDay int    `json:"deadline_day"`
			HasMeter    bool   `json:"has_meter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeErr(w, 400, "name is required")
			return
		}
		if req.DeadlineDay < 1 || req.DeadlineDay > 28 {
			writeErr(w, 400, "deadline_day must be 1..28")
			return
		}
		sv, err := store.CreateService(strings.TrimSpace(req.Name), req.Kind,
			req.AccountNo, req.DeadlineDay, req.HasMeter)
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 201, sv)
	})

	mux.HandleFunc("DELETE /api/services/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := idFrom(r)
		if err != nil {
			writeErr(w, 400, "invalid id")
			return
		}
		if err := store.DeleteService(id); err != nil {
			writeErr(w, 404, "not found")
			return
		}
		w.WriteHeader(204)
	})

	// --- Дашборд ---
	mux.HandleFunc("GET /api/summary", func(w http.ResponseWriter, r *http.Request) {
		sum, err := store.Summary()
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, sum)
	})

	// --- Начисления ---
	mux.HandleFunc("POST /api/accruals", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceID int64   `json:"service_id"`
			Month     string  `json:"month"`
			Amount    float64 `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, 400, "invalid json")
			return
		}
		if req.Amount <= 0 {
			writeErr(w, 400, "amount must be positive")
			return
		}
		a, err := store.AddAccrual(req.ServiceID, req.Month, req.Amount)
		switch {
		case errors.Is(err, ErrNotFound):
			writeErr(w, 400, "service not found")
		case err != nil:
			writeErr(w, 400, err.Error())
		default:
			writeJSON(w, 201, a)
		}
	})

	// --- Оплаты ---
	mux.HandleFunc("POST /api/payments", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceID int64   `json:"service_id"`
			Amount    float64 `json:"amount"`
			PaidOn    string  `json:"paid_on"`
			Note      string  `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, 400, "invalid json")
			return
		}
		if req.Amount <= 0 {
			writeErr(w, 400, "amount must be positive")
			return
		}
		if req.PaidOn == "" {
			req.PaidOn = time.Now().Format("2006-01-02")
		}
		p, err := store.AddPayment(req.ServiceID, req.Amount, req.PaidOn, req.Note)
		switch {
		case errors.Is(err, ErrNotFound):
			writeErr(w, 400, "service not found")
		case err != nil:
			writeErr(w, 400, err.Error())
		default:
			writeJSON(w, 201, p)
		}
	})

	// --- Показания ---
	mux.HandleFunc("POST /api/readings", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceID int64   `json:"service_id"`
			Value     float64 `json:"value"`
			Period    string  `json:"period"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, 400, "invalid json")
			return
		}
		if req.Period == "" {
			req.Period = time.Now().Format("2006-01")
		}
		rd, err := store.CreateReading(req.ServiceID, req.Value, req.Period)
		switch {
		case errors.Is(err, ErrNotFound):
			writeErr(w, 400, "service with meter not found")
		case err != nil:
			writeErr(w, 400, err.Error())
		default:
			writeJSON(w, 201, rd)
		}
	})

	mux.HandleFunc("GET /api/readings", func(w http.ResponseWriter, r *http.Request) {
		svcID, err := strconv.ParseInt(r.URL.Query().Get("service_id"), 10, 64)
		if err != nil {
			writeErr(w, 400, "service_id is required")
			return
		}
		list, err := store.ListReadings(svcID, 5)
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, list)
	})

	// Очистка всех показаний — для кнопки «Очистить все показания» во фронтенде
	mux.HandleFunc("POST /api/readings/clear", func(w http.ResponseWriter, r *http.Request) {
		if _, err := db.Exec(`DELETE FROM readings`); err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		w.WriteHeader(204)
	})

	// --- Статистика ---
	mux.HandleFunc("GET /api/stats/monthly", func(w http.ResponseWriter, r *http.Request) {
		stats, err := store.MonthlyStats()
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, stats)
	})

	// --- Чеки ---
	mux.HandleFunc("POST /api/receipts", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			writeErr(w, 400, "file too large or invalid form")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeErr(w, 400, "file is required")
			return
		}
		defer file.Close()

		name := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(header.Filename))
		dst, err := os.Create(filepath.Join(uploadsDir, name))
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			writeErr(w, 500, "internal error")
			return
		}

		rc, err := store.CreateReceipt(
			parseOptionalID(r.FormValue("service_id")),
			r.FormValue("title"),
			r.FormValue("month"),
			name,
		)
		if err != nil {
			writeErr(w, 400, err.Error())
			return
		}
		writeJSON(w, 201, rc)
	})

	mux.HandleFunc("GET /api/receipts", func(w http.ResponseWriter, r *http.Request) {
		list, err := store.ListReceipts(r.URL.Query().Get("month"))
		if err != nil {
			writeErr(w, 500, "internal error")
			return
		}
		writeJSON(w, 200, list)
	})

	mux.HandleFunc("GET /api/receipts/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		id, err := idFrom(r)
		if err != nil {
			writeErr(w, 400, "invalid id")
			return
		}
		// имя файла генерируем сами, так что подмена пути исключена
		var filename string
		if err := store.db.QueryRow(
			`SELECT filename FROM receipts WHERE id = ?`, id).Scan(&filename); err != nil {
			writeErr(w, 404, "not found")
			return
		}
		http.ServeFile(w, r, filepath.Join(uploadsDir, filename))
	})

	mux.HandleFunc("DELETE /api/receipts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := idFrom(r)
		if err != nil {
			writeErr(w, 400, "invalid id")
			return
		}
		filename, err := store.DeleteReceipt(id)
		if err != nil {
			writeErr(w, 404, "not found")
			return
		}
		os.Remove(filepath.Join(uploadsDir, filename))
		w.WriteHeader(204)
	})

	log.Println("listening on :8091")
	log.Fatal(http.ListenAndServe(":8091", cors(mux)))
}
