package app

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/base64"
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

type telegramChatMember struct {
	Status   string `json:"status"`
	IsMember bool   `json:"is_member"`
}

type telegramBindingTokenClaims struct {
	Platform   string `json:"platform"`
	UserID     string `json:"userId"`
	InviteCode string `json:"inviteCode,omitempty"`
	ExpiresAt  int64  `json:"expiresAt"`
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
	if cfg.MembershipCheckEnabled {
		member, err := app.telegramGetChatMember(c.Request.Context(), cfg, me.ID)
		if err != nil {
			c.JSON(http.StatusBadGateway, APIError{Message: "校验 Telegram 目标群失败：" + err.Error()})
			return
		}
		status := strings.ToLower(strings.TrimSpace(member.Status))
		if status != "creator" && status != "administrator" {
			badRequest(c, "Telegram Bot 必须是目标群管理员")
			return
		}
	}
	if err := app.saveSetting(telegramBotUsernameKey, strings.TrimPrefix(strings.TrimSpace(me.Username), "@")); err != nil {
		serverError(c, err)
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
	done := make(chan struct{})
	app.telegramBotDone = done
	go app.runTelegramBot(botCtx, cfg, done)
}

func (app *App) stopTelegramBot() {
	app.telegramBotMu.Lock()
	cancel := app.telegramBotCancel
	done := app.telegramBotDone
	app.telegramBotMu.Unlock()
	if cancel != nil {
		cancel()
		if done != nil {
			<-done
		}
	}
}

func (app *App) restartTelegramBot(ctx context.Context) {
	app.stopTelegramBot()
	app.startTelegramBot(ctx)
}

func (app *App) runTelegramBot(ctx context.Context, cfg TelegramConfig, done chan struct{}) {
	defer func() {
		close(done)
		app.telegramBotMu.Lock()
		if app.telegramBotDone == done {
			app.telegramBotCancel = nil
			app.telegramBotDone = nil
		}
		app.telegramBotMu.Unlock()
	}()
	username := ""
	if me, err := app.telegramGetMe(ctx, cfg); err != nil {
		log.Printf("Telegram bot getMe failed: %v", err)
	} else {
		username = strings.TrimSpace(me.Username)
		if err := app.saveSetting(telegramBotUsernameKey, strings.TrimPrefix(username, "@")); err != nil {
			log.Printf("Telegram bot username save failed: %v", err)
		}
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
			if strings.Contains(strings.ToLower(err.Error()), "conflict") {
				log.Printf("Telegram bot stopped because another instance owns this bot token")
				return
			}
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
	reply, err := app.handleTelegramCommand(ctx, cfg, message.From.ID, botUsername, command, arg)
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

func (app *App) handleTelegramCommand(ctx context.Context, cfg TelegramConfig, telegramUserID int64, botUsername, command, arg string) (string, error) {
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
			if prompt, allowed, err := app.telegramMembershipGate(ctx, cfg, telegramUserID, botUsername, code); err != nil {
				return "", err
			} else if !allowed {
				return prompt, nil
			}
			bindingURL, err := app.telegramBindingURL(cfg, externalUserID, code)
			if err != nil {
				return "", err
			}
			return "请先登录并绑定账号，绑定完成后会自动使用邀请码：\n" + bindingURL, nil
		}
		return app.telegramStartMessage(telegramUserID)
	case "bind":
		if binding, err := app.telegramBinding(telegramUserID); err == nil {
			frontendURL, err := app.frontendPublicURL()
			if err != nil {
				return "", err
			}
			return telegramAlreadyBoundMessage(binding.UserID, frontendURL), nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		inviteCode := strings.TrimSpace(arg)
		if inviteCode != "" {
			code, err := normalizeInvitationCode(inviteCode)
			if err != nil {
				return "邀请码格式无效。", nil
			}
			inviteCode = code
		}
		if prompt, allowed, err := app.telegramMembershipGate(ctx, cfg, telegramUserID, botUsername, inviteCode); err != nil {
			return "", err
		} else if !allowed {
			return prompt, nil
		}
		bindingURL, err := app.telegramBindingURL(cfg, externalUserID, inviteCode)
		if err != nil {
			return "", err
		}
		return "请打开下面的链接登录并绑定 Telegram：\n" + bindingURL, nil
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

func telegramAlreadyBoundMessage(userID int64, frontendURL string) string {
	message := fmt.Sprintf("这个 Telegram 账号已经绑定 Sub2API 用户 %d。", userID)
	if frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/"); frontendURL != "" {
		return message + "\n前端公开地址：\n" + frontendURL
	}
	return message + "\n前端公开地址尚未配置。"
}

func telegramBindingCompletedMessage(userID int64, frontendURL string, bindingCreated bool, invitation *InvitationBindingResult) string {
	lines := make([]string, 0, 6)
	if bindingCreated {
		lines = append(lines, "Telegram 账号绑定成功。", fmt.Sprintf("Sub2API 用户：%d", userID))
	}
	if invitation != nil && invitation.Bound {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "邀请关系建立成功。", "邀请码："+invitation.InviteCode, "邀请奖励已发放给邀请人。")
	}
	if frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/"); frontendURL != "" {
		lines = append(lines, "", "前端公开地址：", frontendURL)
	}
	return strings.Join(lines, "\n")
}

func telegramInvitationSucceededMessage(invitation InvitationBindingResult) string {
	return fmt.Sprintf("邀请成功。\n邀请码：%s\n邀请奖励：%s\n奖励已发放到你的 Sub2API 余额。", invitation.InviteCode, invitation.RewardAmount.StringFixed(2))
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

func (app *App) telegramBindingURL(cfg TelegramConfig, externalUserID, inviteCode string) (string, error) {
	baseURL, err := app.frontendPublicURL()
	if err != nil {
		return "", err
	}
	token, err := app.issueTelegramBindingToken(externalUserID, inviteCode, cfg.BindingTokenTTLMinutes)
	if err != nil {
		return "", err
	}
	return buildSocialBindingURLWithToken(baseURL, telegramPlatform, externalUserID, inviteCode, token), nil
}

func (app *App) telegramMembershipGate(ctx context.Context, cfg TelegramConfig, telegramUserID int64, botUsername, inviteCode string) (string, bool, error) {
	if !cfg.MembershipCheckEnabled {
		return "", true, nil
	}
	member, err := app.telegramGetChatMember(ctx, cfg, telegramUserID)
	if err != nil {
		return "", false, fmt.Errorf("check Telegram group membership: %w", err)
	}
	if telegramMemberIsActive(member) {
		return "", true, nil
	}
	lines := []string{"绑定前请先加入指定 Telegram 群组：", cfg.GroupJoinURL}
	if inviteCode != "" && strings.TrimSpace(botUsername) != "" {
		lines = append(lines, "", "加入后重新打开下面的邀请链接完成校验：", fmt.Sprintf("https://t.me/%s?start=%s", botUsername, inviteCode))
	} else {
		lines = append(lines, "", "加入后重新发送 /bind 完成校验。")
	}
	return strings.Join(lines, "\n"), false, nil
}

func (app *App) telegramGetChatMember(ctx context.Context, cfg TelegramConfig, telegramUserID int64) (telegramChatMember, error) {
	if strings.TrimSpace(cfg.RequiredGroupChatID) == "" {
		return telegramChatMember{}, errors.New("required Telegram group Chat ID is not configured")
	}
	var member telegramChatMember
	err := app.telegramJSON(ctx, cfg, "getChatMember", map[string]any{
		"chat_id": cfg.RequiredGroupChatID,
		"user_id": telegramUserID,
	}, &member)
	return member, err
}

func telegramMemberIsActive(member telegramChatMember) bool {
	switch strings.ToLower(strings.TrimSpace(member.Status)) {
	case "creator", "administrator", "member":
		return true
	case "restricted":
		return member.IsMember
	default:
		return false
	}
}

func (app *App) issueTelegramBindingToken(externalUserID, inviteCode string, ttlMinutes int) (string, error) {
	claims := telegramBindingTokenClaims{
		Platform:   telegramPlatform,
		UserID:     strings.TrimSpace(externalUserID),
		InviteCode: strings.ToUpper(strings.TrimSpace(inviteCode)),
		ExpiresAt:  time.Now().Add(time.Duration(ttlMinutes) * time.Minute).Unix(),
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + app.sign("telegram-binding."+payload), nil
}

func (app *App) verifyTelegramBindingToken(token, externalUserID, inviteCode string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || !hmac.Equal([]byte(app.sign("telegram-binding."+parts[0])), []byte(parts[1])) {
		return errors.New("Telegram 绑定凭证无效，请从 Bot 重新获取绑定链接")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("Telegram 绑定凭证无效，请从 Bot 重新获取绑定链接")
	}
	var claims telegramBindingTokenClaims
	if err := json.Unmarshal(raw, &claims); err != nil {
		return errors.New("Telegram 绑定凭证无效，请从 Bot 重新获取绑定链接")
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return errors.New("Telegram 绑定凭证已过期，请从 Bot 重新获取绑定链接")
	}
	if claims.Platform != telegramPlatform || claims.UserID != strings.TrimSpace(externalUserID) || claims.InviteCode != strings.ToUpper(strings.TrimSpace(inviteCode)) {
		return errors.New("Telegram 绑定参数与凭证不匹配，请从 Bot 重新获取绑定链接")
	}
	return nil
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

func (app *App) queueTelegramBindingNotifications(externalUserID string, userID int64, bindingCreated bool, invitation *InvitationBindingResult) {
	if !bindingCreated && (invitation == nil || !invitation.Bound) {
		return
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(externalUserID), 10, 64)
	if err != nil {
		log.Printf("Telegram binding notification skipped: invalid chat_id=%q", externalUserID)
		return
	}
	frontendURL, err := app.frontendPublicURL()
	if err != nil {
		log.Printf("Telegram binding notification frontend URL failed: %v", err)
		frontendURL = ""
	}
	message := telegramBindingCompletedMessage(userID, frontendURL, bindingCreated, invitation)
	app.queueTelegramMessage(chatID, message)
	if invitation != nil && invitation.Bound && invitation.InviterUserID > 0 {
		app.queueTelegramMessageForSub2User(invitation.InviterUserID, telegramInvitationSucceededMessage(*invitation))
	}
}

func (app *App) queueTelegramMessage(chatID int64, text string) {
	go app.sendTelegramNotification(chatID, text)
}

func (app *App) queueTelegramMessageForSub2User(userID int64, text string) {
	go func() {
		var binding SocialAccountBinding
		err := app.db.Where("user_id = ? AND platform = ?", userID, telegramPlatform).First(&binding).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if err != nil {
			log.Printf("Telegram invitation notification binding lookup failed: user_id=%d error=%v", userID, err)
			return
		}
		chatID, err := strconv.ParseInt(strings.TrimSpace(binding.ExternalUserID), 10, 64)
		if err != nil {
			log.Printf("Telegram invitation notification skipped: invalid chat_id=%q", binding.ExternalUserID)
			return
		}
		app.sendTelegramNotification(chatID, text)
	}()
}

func (app *App) sendTelegramNotification(chatID int64, text string) {
	if chatID == 0 || strings.TrimSpace(text) == "" {
		return
	}
	cfg, err := app.effectiveTelegramConfig()
	if err != nil {
		log.Printf("Telegram notification config failed: %v", err)
		return
	}
	if !cfg.Enabled || strings.TrimSpace(cfg.BotToken) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := app.telegramSendMessage(ctx, cfg, chatID, text); err != nil {
		log.Printf("Telegram notification failed: chat_id=%d error=%v", chatID, err)
	}
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
