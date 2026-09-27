package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const btnList = "📋 Список вакансий"

// TelegramBot — телеграм-бот для управления БД вакансий через REST API (api.go)
// и отправки пушей о новых вакансиях. Управление идёт через кнопки:
// клавиатура под полем ввода — для навигации, инлайн-кнопки под сообщением
// с вакансией — "Изменить" / "Удалить" / "Откликнуться".
type TelegramBot struct {
	api     *tgbotapi.BotAPI
	chatID  int64
	apiBase string
	client  *http.Client
}

// NewTelegramBot создаёт бота по токену.
// chatID — id чата, куда слать пуши и откуда принимать нажатия кнопок.
// apiBase — адрес запущенного REST API, например "http://localhost:8080".
func NewTelegramBot(token string, chatID int64, apiBase string) (*TelegramBot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания телеграм-бота: %w", err)
	}

	return &TelegramBot{
		api:     api,
		chatID:  chatID,
		apiBase: strings.TrimRight(apiBase, "/"),
		client:  &http.Client{},
	}, nil
}

// setupTelegramBot создаёт телеграм-бота из переменных окружения
// TELEGRAM_BOT_TOKEN и TELEGRAM_CHAT_ID. Если они не заданы, бот не создаётся
// и программа продолжает работать без него (только REST API + цикл проверки).
func setupTelegramBot() *TelegramBot {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatIDStr == "" {
		fmt.Println("TELEGRAM_BOT_TOKEN / TELEGRAM_CHAT_ID не заданы — телеграм-бот отключён")
		return nil
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		fmt.Println("некорректный TELEGRAM_CHAT_ID:", err)
		return nil
	}

	bot, err := NewTelegramBot(token, chatID, apiBaseURL)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	fmt.Println("Телеграм-бот запущен")
	return bot
}

// mainKeyboard — клавиатура под полем ввода.
func mainKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(btnList),
		),
	)
}

// vacancyKeyboard — инлайн-кнопки под сообщением с вакансией.
func vacancyKeyboard(id int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✏️ Изменить", fmt.Sprintf("edit:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("del:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Откликнуться", fmt.Sprintf("apply:%d", id)),
		),
	)
}

// editKeyboard — подменю кнопки "Изменить": явный выбор статуса отклика.
func editKeyboard(id int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Откликнулся", fmt.Sprintf("edit_yes:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Не откликался", fmt.Sprintf("edit_no:%d", id)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("‹ Назад", fmt.Sprintf("back:%d", id)),
		),
	)
}

// deleteConfirmKeyboard — подменю кнопки "Удалить": подтверждение.
func deleteConfirmKeyboard(id int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Да, удалить", fmt.Sprintf("del_yes:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("‹ Отмена", fmt.Sprintf("back:%d", id)),
		),
	)
}

func vacancyText(v vacancyResponse) string {
	status := "❌ не откликался"
	if v.Applied {
		status = "✅ откликнулся"
	}

	if v.Body == "" {
		return fmt.Sprintf("%s\n%s\n\nСтатус: %s", v.Title, v.URL, status)
	}

	return fmt.Sprintf("%s\n%s\n\n%s\n\nСтатус: %s", v.Title, v.URL, v.Body, status)
}

// NotifyNewVacancy отправляет пуш о новой найденной вакансии с кнопками управления.
// v должна быть уже сохранённой записью из БД (с заполненным ID).
func (b *TelegramBot) NotifyNewVacancy(v Vacancy) error {
	msg := tgbotapi.NewMessage(b.chatID, "🆕 Новая стажировка по Go!\n\n"+vacancyText(toResponse(v)))
	msg.ReplyMarkup = vacancyKeyboard(v.ID)
	_, err := b.api.Send(msg)
	return err
}

// Start запускает приём и обработку кнопок бота (long polling). Блокирующий вызов.
func (b *TelegramBot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		switch {
		case update.CallbackQuery != nil:
			b.handleCallback(update.CallbackQuery)
		case update.Message != nil:
			b.handleMessage(update.Message)
		}
	}
}

func (b *TelegramBot) handleMessage(msg *tgbotapi.Message) {
	switch msg.Text {
	case "/start", "/help":
		out := tgbotapi.NewMessage(msg.Chat.ID, "Привет! Нажми кнопку ниже, чтобы посмотреть вакансии.")
		out.ReplyMarkup = mainKeyboard()
		b.send(out)
	case btnList:
		b.sendVacancyList(msg.Chat.ID)
	default:
		out := tgbotapi.NewMessage(msg.Chat.ID, "Используй кнопки ниже 👇")
		out.ReplyMarkup = mainKeyboard()
		b.send(out)
	}
}

func (b *TelegramBot) sendVacancyList(chatID int64) {
	var vacancies []vacancyResponse
	if err := b.apiGet("/api/vacancies", &vacancies); err != nil {
		b.send(tgbotapi.NewMessage(chatID, "Ошибка получения списка: "+err.Error()))
		return
	}

	if len(vacancies) == 0 {
		b.send(tgbotapi.NewMessage(chatID, "Вакансий пока нет."))
		return
	}

	for _, v := range vacancies {
		out := tgbotapi.NewMessage(chatID, vacancyText(v))
		out.ReplyMarkup = vacancyKeyboard(v.ID)
		b.send(out)
	}
}

func (b *TelegramBot) handleCallback(cb *tgbotapi.CallbackQuery) {
	action, id, err := parseCallbackData(cb.Data)
	if err != nil {
		b.answerCallback(cb.ID, "Некорректные данные")
		return
	}

	chatID := cb.Message.Chat.ID
	msgID := cb.Message.MessageID

	switch action {
	case "apply":
		b.applyAndRefresh(chatID, msgID, id, true)
		b.answerCallback(cb.ID, "Отмечено: откликнулся")

	case "edit":
		b.editKeyboardOnMessage(chatID, msgID, id, editKeyboard(id))
		b.answerCallback(cb.ID, "")

	case "edit_yes":
		b.applyAndRefresh(chatID, msgID, id, true)
		b.answerCallback(cb.ID, "Сохранено")

	case "edit_no":
		b.applyAndRefresh(chatID, msgID, id, false)
		b.answerCallback(cb.ID, "Сохранено")

	case "back":
		b.refreshVacancyMessage(chatID, msgID, id, vacancyKeyboard(id))
		b.answerCallback(cb.ID, "")

	case "del":
		b.editKeyboardOnMessage(chatID, msgID, id, deleteConfirmKeyboard(id))
		b.answerCallback(cb.ID, "")

	case "del_yes":
		if err := b.apiDelete(fmt.Sprintf("/api/vacancies/%d", id)); err != nil {
			b.answerCallback(cb.ID, "Ошибка: "+err.Error())
			return
		}
		edit := tgbotapi.NewEditMessageText(chatID, msgID, "🗑 Вакансия удалена")
		b.send(edit)
		b.answerCallback(cb.ID, "Удалено")

	default:
		b.answerCallback(cb.ID, "Неизвестное действие")
	}
}

// applyAndRefresh обновляет статус отклика и перерисовывает сообщение с вакансией.
func (b *TelegramBot) applyAndRefresh(chatID int64, msgID int, id int64, applied bool) {
	body, _ := json.Marshal(map[string]bool{"applied": applied})

	var v vacancyResponse
	if err := b.apiPatch(fmt.Sprintf("/api/vacancies/%d", id), body, &v); err != nil {
		b.send(tgbotapi.NewMessage(chatID, "Ошибка: "+err.Error()))
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, vacancyText(v), vacancyKeyboard(id))
	b.send(edit)
}

// refreshVacancyMessage перечитывает вакансию и возвращает переданную клавиатуру.
func (b *TelegramBot) refreshVacancyMessage(chatID int64, msgID int, id int64, kb tgbotapi.InlineKeyboardMarkup) {
	var v vacancyResponse
	if err := b.apiGet(fmt.Sprintf("/api/vacancies/%d", id), &v); err != nil {
		b.send(tgbotapi.NewMessage(chatID, "Ошибка: "+err.Error()))
		return
	}

	edit := tgbotapi.NewEditMessageTextAndMarkup(chatID, msgID, vacancyText(v), kb)
	b.send(edit)
}

// editKeyboardOnMessage меняет только клавиатуру под уже отправленным сообщением.
func (b *TelegramBot) editKeyboardOnMessage(chatID int64, msgID int, id int64, kb tgbotapi.InlineKeyboardMarkup) {
	edit := tgbotapi.NewEditMessageReplyMarkup(chatID, msgID, kb)
	b.send(edit)
}

func (b *TelegramBot) send(c tgbotapi.Chattable) {
	if _, err := b.api.Send(c); err != nil {
		log.Println("ошибка отправки сообщения в телеграм:", err)
	}
}

func (b *TelegramBot) answerCallback(callbackID, text string) {
	cb := tgbotapi.NewCallback(callbackID, text)
	if _, err := b.api.Request(cb); err != nil {
		log.Println("ошибка ответа на callback:", err)
	}
}

// parseCallbackData разбирает данные вида "action:id".
func parseCallbackData(data string) (action string, id int64, err error) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("некорректный формат callback data: %q", data)
	}

	id, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("некорректный id в callback data: %q", data)
	}

	return parts[0], id, nil
}

// --- обращения к REST API (api.go) ---

func (b *TelegramBot) apiGet(path string, out any) error {
	resp, err := b.client.Get(b.apiBase + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeAPIResponse(resp, out)
}

func (b *TelegramBot) apiPatch(path string, body []byte, out any) error {
	req, err := http.NewRequest(http.MethodPatch, b.apiBase+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeAPIResponse(resp, out)
}

func (b *TelegramBot) apiDelete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, b.apiBase+path, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return decodeAPIResponse(resp, nil)
	}
	return nil
}

func decodeAPIResponse(resp *http.Response, out any) error {
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("%s", apiErr.Error)
		}
		return fmt.Errorf("статус ответа: %s", resp.Status)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
