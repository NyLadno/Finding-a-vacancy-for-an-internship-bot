package main

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// OpenDB открывает (или создаёт) файл базы данных SQLite и применяет схему.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %w", err)
	}

	// SQLite допускает только одного писателя одновременно: несколько
	// соединений из пула database/sql приводят к взаимной блокировке
	// при параллельных запросах из REST API, бота и фонового чекера.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка применения схемы: %w", err)
	}

	return db, nil
}
