package main

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
)

const (
	checkInterval = time.Hour
	dbPath        = "./internships.db"
	apiAddr       = ":8080"
	apiBaseURL    = "http://localhost" + apiAddr
)

// main — точка входа в программу: поднимает БД, REST API, телеграм-бота
// и запускает периодическую проверку новых стажировок.
func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println(".env не найден, используются переменные окружения системы")
	}

	db, err := OpenDB(dbPath)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	go func() {
		fmt.Printf("REST API запущен на %s\n", apiAddr)
		if err := StartAPIServer(db, apiAddr); err != nil {
			fmt.Println("ошибка REST API:", err)
		}
	}()

	bot := setupTelegramBot()
	if bot != nil {
		go bot.Start()
	}

	checkInternships(db, bot)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		checkInternships(db, bot)
	}
}
