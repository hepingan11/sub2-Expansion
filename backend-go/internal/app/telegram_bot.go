package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const telegramPlatform = "telegram"

type telegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

type telegramUpdate struct {
	UpdateID int64           `json:"update_id"`
	Message  telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	Chat      telegramChat `json:"chat"`
	From      telegramUser `json:"from"`
	Text      string       `json:"text"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

type telegramUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type telegramMe struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (app *App) connectTelegramBot(c *gin.Context) {
	cfg, err := app.effectiveTelegramConfig()
	if err != nil {
		serverError(c, err)
		return
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		badRequest(c, "Telegram Bot Token 未配置")
		return
	}
	me, err := app.telegramGetMe(c.Request.Context(), cfg)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIError{Message: "连接 Telegram 失败：" + err.Error()})
		return
	}
	if err := app.telegramDeleteWebhook(c.Request.Context(), cfg, true); err != nil {
		c.JSON(http.StatusBadGateway, APIError{Message: "清理 Telegram Webhook 失败：" + err.Error()})
		return
	}
	cfg.BotUsername = strings.TrimSpace(me.Username)
	cfg.Connected = true
	cfg.BotToken = ""
	cfg.BotTokenSet = true
	app.restartTelegramBot(context.Background())
	c.JSON(http.StatusOK, cfg)
}

func (app *App) startTelegramBot(ctx context.Context) {
	cfg, err := app.effectiveTelegramConfig()
	if err != nil {
		log.Printf("Telegram bot config failed: %v", err)
		return
	}
	if !cfg.Enabled || strings.TrimSpace(cfg.BotToken) == "" {
		return
	}
	app.telegramBotMu.Lock()
	defer app.telegramBotMu.Unlock()
	if app.telegramBotCancel != nil {
		return
	}
	botCtx, cancel := context.WithCancel(ctx)
	app.telegramBotCancel = cancel
	go app.runTelegramBot(botCtx, cfg)
}

func (app *App) stopTelegramBot() {
	app.telegramBotMu.Lock()
	defer app.telegramBotMu.Unlock()
	if app.telegramBotCancel != nil {
		app.telegramBotCancel()
		app.telegramBotCancel = nil
	}
}

func (app *App) restartTelegramBot(ctx context.Context) {
	app.stopTelegramBot()
	app.startTelegramBot(ctx)
}

func (app *App) runTelegramBot(ctx context.Context, cfg TelegramConfig) {
	defer func() {
		app.telegramBotMu.Lock()
		if app.telegramBotCancel != nil && ctx.Err() != nil {
			app.telegramBotCancel = nil
		}
		app.telegramBotMu.Unlock()
	}()
	username := ""
	if me, err := app.telegramGetMe(ctx, cfg); err != nil {
		log.Printf("Telegram bot getMe failed: %v", err)
	} else {
		username = strings.TrimSpace(me.Username)
		log.Printf("Telegram bot started as @%s", username)
	}
	if err := app.telegramDeleteWebhook(ctx, cfg, true); err != nil {
		log.Printf("Telegram deleteWebhook failed: %v", err)
	}

	pollEvery := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if pollEvery <= 0 {
		pollEvery = 2 * time.Second
	}
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		updates, err := app.telegramGetUpdates(ctx, cfg, offset)
		if err != nil {
			log.Printf("Telegram getUpdates failed: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(pollEvery):
				continue
			}
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
			app.handleTelegramUpdate(ctx, cfg, update, username)
		}
	}
}

func (app *App) telegramGetMe(ctx context.Context, cfg TelegramConfig) (telegramMe, error) {
	var me telegramMe
	err := app.telegramJSON(ctx, cfg, "getMe", nil, &me)
	return me, err
}

func (app *App) telegramGetUpdates(ctx context.Context, cfg TelegramConfig, offset int64) ([]telegramUpdate, error) {
	payload := map[string]any{
		"timeout":         30,
		"allowed_updates": []string{"message"},
	}
	if offset > 0 {
		payload["offset"] = offset
	}
	var updates []telegramUpdate
	err := app.telegramJSON(ctx, cfg, "getUpdates", payload, &updates)
	return updates, err
}

func (app *App) telegramDeleteWebhook(ctx context.Context, cfg TelegramConfig, dropPendingUpdates bool) error {
	payload := map[string]any{
		"drop_pending_updates": dropPendingUpdates,
	}
	return app.telegramJSON(ctx, cfg, "deleteWebhook", payload, nil)
}

func (app *App) handleTelegramUpdate(ctx context.Context, cfg TelegramConfig, update telegramUpdate, botUsername string) {
	message := update.Message
	if message.Chat.ID == 0 || message.From.ID == 0 || strings.TrimSpace(message.Text) == "" {
		return
	}
	command, arg := parseTelegramCommand(message.Text)
	if command == "" {
		return
	}
	reply, err := app.handleTelegramCommand(ctx, message.From.ID, botUsername, command, arg)
	if err != nil {
		log.Printf("Telegram command failed: user_id=%d command=%s error=%v", message.From.ID, command, err)
		reply = telegramErrorMessage(err)
	}
	if strings.TrimSpace(reply) == "" {
		return
	}
	if err := app.telegramSendMessage(ctx, cfg, message.Chat.ID, reply); err != nil {
		log.Printf("Telegram sendMessage failed: %v", err)
	}
}

func parseTelegramCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", ""
	}
	command := strings.TrimPrefix(fields[0], "/")
	if index := strings.Index(command, "@"); index >= 0 {
		command = command[:index]
	}
	command = strings.ToLower(command)
	arg := ""
	if len(fields) > 1 {
		arg = strings.TrimSpace(strings.Join(fields[1:], " "))
	}
	return command, arg
}

func (app *App) handleTelegramCommand(ctx context.Context, telegramUserID int64, botUsername, command, arg string) (string, error) {
	externalUserID := strconv.FormatInt(telegramUserID, 10)
	switch command {
	case "start":
		inviteCode := strings.TrimSpace(arg)
		if inviteCode != "" {
			code, err := normalizeInvitationCode(inviteCode)
			if err != nil {
				return "邀请码格式无效。", nil
			}
			if _, err := app.telegramBinding(telegramUserID); err == nil {
				return "这个 Telegram 账号已经绑定。邀请关系只能在首次绑定账号时建立。", nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", err
			}
			return "请先登录并绑定账号，绑定完成后会自动使用邀请码：\n" + app.telegramBindingURL(externalUserID, code), nil
		}
		return app.telegramStartMessage(telegramUserID)
	case "bind":
		inviteCode := strings.TrimSpace(arg)
		if inviteCode != "" {
			code, err := normalizeInvitationCode(inviteCode)
			if err != nil {
				return "邀请码格式无效。", nil
			}
			inviteCode = code
		}
		return "请打开下面的链接登录并绑定 Telegram：\n" + app.telegramBindingURL(externalUserID, inviteCode), nil
	case "checkin", "qiandao", "签到":
		return app.telegramCheckIn(ctx, telegramUserID)
	case "invite":
		return app.telegramInvite(ctx, telegramUserID, botUsername)
	case "me":
		return app.telegramMe(telegramUserID)
	case "help":
		return telegramHelpMessage(), nil
	default:
		return telegramHelpMessage(), nil
	}
}

func (app *App) telegramStartMessage(telegramUserID int64) (string, error) {
	if binding, err := app.telegramBinding(telegramUserID); err == nil {
		return fmt.Sprintf("已绑定 Sub2API 用户 %d。\n\n%s", binding.UserID, telegramHelpMessage()), nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return "欢迎使用。\n\n还没有绑定账号，请先发送 /bind 获取绑定链接。\n\n" + telegramHelpMessage(), nil
}

func (app *App) telegramCheckIn(ctx context.Context, telegramUserID int64) (string, error) {
	binding, err := app.telegramBinding(telegramUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "这个 Telegram 账号还没有绑定，请先发送 /bind。", nil
		}
		return "", err
	}
	userID := strconv.FormatInt(binding.UserID, 10)
	today := Today()
	if response, found, err := app.todayCheckInResponse(userID, today); err != nil {
		return "", err
	} else if found {
		return fmt.Sprintf("今天已经签到过了。\n日期：%s\n奖励：%s", today.Format("2006-01-02"), response.Amount.StringFixed(2)), nil
	}
	response, err := app.createCheckIn(ctx, userID, today, &binding.UserID, checkInMethodSocial, telegramPlatform)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\n日期：%s\n奖励：%s", response.Message, today.Format("2006-01-02"), response.Amount.StringFixed(2)), nil
}

func (app *App) telegramInvite(ctx context.Context, telegramUserID int64, botUsername string) (string, error) {
	binding, err := app.telegramBinding(telegramUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "这个 Telegram 账号还没有绑定，请先发送 /bind。", nil
		}
		return "", err
	}
	code, err := app.ensureInvitationCode(binding.UserID)
	if err != nil {
		return "", err
	}
	config, err := app.invitationConfigForPlatform(telegramPlatform)
	if err != nil {
		return "", err
	}
	link := ""
	if strings.TrimSpace(botUsername) != "" {
		link = fmt.Sprintf("\n邀请链接：https://t.me/%s?start=%s", botUsername, code.Code)
	}
	enabled := config.AfterTime != "" && config.Amount.Cmp(decimal.Zero) > 0
	if !enabled {
		return "你的邀请码：" + code.Code + link + "\n当前邀请奖励尚未启用。", nil
	}
	return fmt.Sprintf("你的邀请码：%s%s\nTelegram 邀请奖励：%s\n新人账号需晚于：%s", code.Code, link, config.Amount.StringFixed(2), config.AfterTime), nil
}

func (app *App) telegramMe(telegramUserID int64) (string, error) {
	binding, err := app.telegramBinding(telegramUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "这个 Telegram 账号还没有绑定，请先发送 /bind。", nil
		}
		return "", err
	}
	return fmt.Sprintf("Telegram 已绑定 Sub2API 用户 %d。", binding.UserID), nil
}

func (app *App) telegramBinding(telegramUserID int64) (SocialAccountBinding, error) {
	var binding SocialAccountBinding
	err := app.db.Where("platform = ? AND external_user_id = ?", telegramPlatform, strconv.FormatInt(telegramUserID, 10)).First(&binding).Error
	return binding, err
}

func (app *App) telegramBindingURL(externalUserID, inviteCode string) string {
	baseURL, err := app.frontendPublicURL()
	if err != nil {
		baseURL = ""
	}
	return buildSocialBindingURL(baseURL, telegramPlatform, externalUserID, inviteCode)
}

func telegramHelpMessage() string {
	return strings.Join([]string{
		"可用指令：",
		"/bind - 绑定 Sub2API 账号",
		"/checkin - 今日签到",
		"/invite - 获取邀请码",
		"/me - 查看绑定状态",
	}, "\n")
}

func telegramErrorMessage(err error) string {
	if isBusinessConflict(err) {
		return err.Error()
	}
	var upstreamErr upstreamAPIError
	if errors.As(err, &upstreamErr) {
		return "Sub2API 调用失败：" + upstreamErr.message
	}
	return "操作失败，请稍后再试。"
}

func (app *App) telegramSendMessage(ctx context.Context, cfg TelegramConfig, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	return app.telegramJSON(ctx, cfg, "sendMessage", payload, nil)
}

func (app *App) telegramJSON(ctx context.Context, cfg TelegramConfig, method string, payload any, out any) error {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.APIBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	token := strings.TrimSpace(cfg.BotToken)
	if token == "" {
		return errors.New("Telegram bot token is not configured")
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, baseURL+"/bot"+token+"/"+method, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var envelope telegramAPIResponse[json.RawMessage]
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.OK {
		message := strings.TrimSpace(envelope.Description)
		if message == "" {
			message = resp.Status
		}
		return errors.New(message)
	}
	if out == nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}
