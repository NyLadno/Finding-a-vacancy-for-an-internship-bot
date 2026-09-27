package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	baseURL   = "https://youngjunior.ru"
	targetURL = baseURL + "/go/internships"
)

// FetchHTML получает HTML страницы по указанному URL через net/http.
func FetchHTML(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("ошибка запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("статус ответа: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения: %w", err)
	}

	return string(body), nil
}

// ищем заголовок, в котором есть упоминания Go/golang
var goLangRe = regexp.MustCompile(`(?i)\b(go|golang)\b`)

// затем смотрим, что бы там была имнно стажировка
var internRe = regexp.MustCompile(`(?i)(стажер|стажёр|стажировк|intern|trainee)`)

// Internship — вакансия стажировки, прошедшая фильтрацию.
type Internship struct {
	Title string
	URL   string
}

// ParseGoInternships парсит HTML страницы и возвращает заголовки h3
// вместе со ссылками на вакансии.
func ParseGoInternships(html string) ([]Internship, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %w", err)
	}

	var result []Internship

	doc.Find("h3.__className_dda9cc").Each(func(i int, s *goquery.Selection) {
		title := strings.TrimSpace(s.Text())
		if title == "" {
			return
		}

		if !goLangRe.MatchString(title) || !internRe.MatchString(title) {
			return
		}

		href, ok := s.Closest("a").Attr("href")
		if !ok {
			return
		}

		link, err := resolveURL(baseURL, href)
		if err != nil {
			return
		}

		result = append(result, Internship{Title: title, URL: link})
	})

	return result, nil
}

// FetchVacancyBody получает страницу конкретной вакансии и достаёт описание
// из JSON-LD блока (schema.org/JobPosting), который сайт кладёт на каждую
// страницу вакансии.
func FetchVacancyBody(vacancyURL string) (string, error) {
	html, err := FetchHTML(vacancyURL)
	if err != nil {
		return "", err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга HTML: %w", err)
	}

	var body string
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(i int, s *goquery.Selection) bool {
		var posting struct {
			Type        string `json:"@type"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal([]byte(s.Text()), &posting); err != nil {
			return true
		}
		if posting.Type == "JobPosting" && posting.Description != "" {
			body = posting.Description
			return false
		}
		return true
	})

	return body, nil
}

// resolveURL превращает относительную ссылку в абсолютную, используя base.
func resolveURL(base, href string) (string, error) {
	baseParsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	refParsed, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	return baseParsed.ResolveReference(refParsed).String(), nil
}

// checkInternships получает HTML страницы, парсит стажировки по Go,
// сохраняет новые (ранее не встречавшиеся) вакансии в БД
// и, если передан bot, присылает пуш о каждой новой вакансии.
func checkInternships(db *sql.DB, bot *TelegramBot) {
	fmt.Printf("[%s] проверка стажировок...\n", time.Now().Format("2006-01-02 15:04:05"))

	html, err := FetchHTML(targetURL)
	if err != nil {
		fmt.Println(err)
		return
	}

	internships, err := ParseGoInternships(html)
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(internships) == 0 {
		fmt.Println("Стажировки по Go не найдены")
		return
	}

	for _, i := range internships {
		existing, err := GetVacancyByURL(db, i.URL)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if existing != nil {
			// вакансия уже есть в БД — пропускаем
			continue
		}

		body, err := FetchVacancyBody(i.URL)
		if err != nil {
			fmt.Println("не удалось получить описание вакансии:", err)
		}

		if err := SaveVacancy(db, Vacancy{
			URL:   i.URL,
			Title: i.Title,
			Body:  body,
		}); err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Printf("Новая вакансия: %s — %s\n", i.Title, i.URL)

		if bot != nil {
			saved, err := GetVacancyByURL(db, i.URL)
			if err != nil || saved == nil {
				fmt.Println("не удалось получить сохранённую вакансию для пуша:", err)
				continue
			}
			if err := bot.NotifyNewVacancy(*saved); err != nil {
				fmt.Println("ошибка отправки пуша:", err)
			}
		}
	}
}
