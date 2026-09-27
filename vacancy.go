package main

import (
	"database/sql"
	"fmt"
	"time"
)

// Vacancy — запись о вакансии в БД.
type Vacancy struct {
	ID          int64
	URL         string
	Title       string
	Body        string
	PublishedAt sql.NullTime // дата публикации вакансии (может быть неизвестна)
	CreatedAt   time.Time    // дата, когда запись появилась в БД
	Applied     bool         // откликнулся ли я
}

// SaveVacancy сохраняет вакансию, если такой ссылки ещё нет в БД.
// Если вакансия с таким URL уже есть — ничего не делает.
func SaveVacancy(db *sql.DB, v Vacancy) error {
	_, err := db.Exec(
		`INSERT INTO vacancies (url, title, body, published_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(url) DO NOTHING`,
		v.URL, v.Title, v.Body, v.PublishedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка сохранения вакансии: %w", err)
	}
	return nil
}

// GetVacancyByURL возвращает вакансию по ссылке.
// Если вакансия не найдена, возвращает (nil, nil).
func GetVacancyByURL(db *sql.DB, url string) (*Vacancy, error) {
	row := db.QueryRow(
		`SELECT id, url, title, body, published_at, created_at, applied
		 FROM vacancies WHERE url = ?`,
		url,
	)

	var v Vacancy
	err := row.Scan(&v.ID, &v.URL, &v.Title, &v.Body, &v.PublishedAt, &v.CreatedAt, &v.Applied)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка получения вакансии: %w", err)
	}

	return &v, nil
}

// GetVacancyByID возвращает вакансию по её id.
// Если вакансия не найдена, возвращает (nil, nil).
func GetVacancyByID(db *sql.DB, id int64) (*Vacancy, error) {
	row := db.QueryRow(
		`SELECT id, url, title, body, published_at, created_at, applied
		 FROM vacancies WHERE id = ?`,
		id,
	)

	var v Vacancy
	err := row.Scan(&v.ID, &v.URL, &v.Title, &v.Body, &v.PublishedAt, &v.CreatedAt, &v.Applied)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка получения вакансии: %w", err)
	}

	return &v, nil
}

// DeleteVacancy удаляет вакансию по id. Возвращает false, если такой записи не было.
func DeleteVacancy(db *sql.DB, id int64) (bool, error) {
	res, err := db.Exec(`DELETE FROM vacancies WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("ошибка удаления вакансии: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("ошибка удаления вакансии: %w", err)
	}

	return affected > 0, nil
}

// SetAppliedByID отмечает, откликнулся ли пользователь на вакансию, по id.
// Возвращает false, если такой записи не было.
func SetAppliedByID(db *sql.DB, id int64, applied bool) (bool, error) {
	res, err := db.Exec(`UPDATE vacancies SET applied = ? WHERE id = ?`, applied, id)
	if err != nil {
		return false, fmt.Errorf("ошибка обновления статуса отклика: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("ошибка обновления статуса отклика: %w", err)
	}

	return affected > 0, nil
}

// ListVacancies возвращает все вакансии из БД, отсортированные по дате появления в БД.
func ListVacancies(db *sql.DB) ([]Vacancy, error) {
	rows, err := db.Query(
		`SELECT id, url, title, body, published_at, created_at, applied
		 FROM vacancies ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка вакансий: %w", err)
	}
	defer rows.Close()

	var result []Vacancy
	for rows.Next() {
		var v Vacancy
		if err := rows.Scan(&v.ID, &v.URL, &v.Title, &v.Body, &v.PublishedAt, &v.CreatedAt, &v.Applied); err != nil {
			return nil, fmt.Errorf("ошибка чтения вакансии: %w", err)
		}
		result = append(result, v)
	}

	return result, rows.Err()
}

// SetApplied отмечает, откликнулся ли пользователь на вакансию.
func SetApplied(db *sql.DB, url string, applied bool) error {
	_, err := db.Exec(`UPDATE vacancies SET applied = ? WHERE url = ?`, applied, url)
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса отклика: %w", err)
	}
	return nil
}
