package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// vacancyResponse — представление вакансии для JSON-ответов API.
type vacancyResponse struct {
	ID          int64      `json:"id"`
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	Applied     bool       `json:"applied"`
}

func toResponse(v Vacancy) vacancyResponse {
	resp := vacancyResponse{
		ID:        v.ID,
		URL:       v.URL,
		Title:     v.Title,
		Body:      v.Body,
		CreatedAt: v.CreatedAt,
		Applied:   v.Applied,
	}
	if v.PublishedAt.Valid {
		resp.PublishedAt = &v.PublishedAt.Time
	}
	return resp
}

// NewAPIServer собирает http.Handler со всеми REST-эндпоинтами для управления БД.
//
//	GET    /api/vacancies       — список всех вакансий
//	GET    /api/vacancies/{id}  — одна вакансия
//	PATCH  /api/vacancies/{id}  — обновить статус отклика, тело {"applied": true|false}
//	DELETE /api/vacancies/{id}  — удалить вакансию
func NewAPIServer(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/vacancies", handleListVacancies(db))
	mux.HandleFunc("GET /api/vacancies/{id}", handleGetVacancy(db))
	mux.HandleFunc("PATCH /api/vacancies/{id}", handleUpdateVacancy(db))
	mux.HandleFunc("DELETE /api/vacancies/{id}", handleDeleteVacancy(db))

	return mux
}

// StartAPIServer запускает REST API на указанном адресе (например, ":8080").
func StartAPIServer(db *sql.DB, addr string) error {
	return http.ListenAndServe(addr, NewAPIServer(db))
}

func handleListVacancies(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vacancies, err := ListVacancies(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := make([]vacancyResponse, 0, len(vacancies))
		for _, v := range vacancies {
			resp = append(resp, toResponse(v))
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func handleGetVacancy(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "некорректный id")
			return
		}

		v, err := GetVacancyByID(db, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if v == nil {
			writeError(w, http.StatusNotFound, "вакансия не найдена")
			return
		}

		writeJSON(w, http.StatusOK, toResponse(*v))
	}
}

func handleUpdateVacancy(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "некорректный id")
			return
		}

		var body struct {
			Applied bool `json:"applied"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "некорректное тело запроса")
			return
		}

		found, err := SetAppliedByID(db, id, body.Applied)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "вакансия не найдена")
			return
		}

		v, err := GetVacancyByID(db, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, toResponse(*v))
	}
}

func handleDeleteVacancy(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "некорректный id")
			return
		}

		found, err := DeleteVacancy(db, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "вакансия не найдена")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
