package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const videoPoolFormatOpenAIVideos = "openai_videos"

const (
	videoPoolTestModel  = "seedance-2.0"
	videoPoolTestPrompt = "A calm ocean wave at sunrise."
)

var videoTaskIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)
var videoModelPattern = regexp.MustCompile(`^[A-Za-z0-9_./:-]{1,100}$`)

var videoPoolHTTPClient = newVideoPoolHTTPClient()

func newVideoPoolHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			DisableKeepAlives:     true,
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type openAIVideoTaskResponse struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	Model       string `json:"model"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	CreatedAt   int64  `json:"created_at"`
	CompletedAt int64  `json:"completed_at"`
	Metadata    struct {
		URL string `json:"url"`
	} `json:"metadata"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type videoUpstreamErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (app *App) listVideoAccountPools(c *gin.Context) {
	var pools []VideoAccountPool
	if err := app.db.Order("id ASC").Find(&pools).Error; err != nil {
		serverError(c, err)
		return
	}
	result := make([]VideoAccountPoolResponse, 0, len(pools))
	for _, pool := range pools {
		result = append(result, videoAccountPoolResponse(pool))
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) createVideoAccountPool(c *gin.Context) {
	var req VideoAccountPoolRequest
	if !bindJSON(c, &req) {
		return
	}
	pool, err := normalizeVideoAccountPoolRequest(req, true)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.db.Create(&pool).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusCreated, videoAccountPoolResponse(pool))
}

func (app *App) updateVideoAccountPool(c *gin.Context) {
	pool, ok := app.findVideoAccountPool(c)
	if !ok {
		return
	}
	var req VideoAccountPoolRequest
	if !bindJSON(c, &req) {
		return
	}
	normalized, err := normalizeVideoAccountPoolRequest(req, false)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	pool.Name = normalized.Name
	pool.Format = normalized.Format
	pool.BaseURL = normalized.BaseURL
	pool.BaseURLIsComplete = normalized.BaseURLIsComplete
	pool.Enabled = normalized.Enabled
	if req.ClearAPIKey {
		pool.APIKey = ""
	} else if normalized.APIKey != "" {
		pool.APIKey = normalized.APIKey
	}
	if pool.Enabled && pool.APIKey == "" {
		badRequest(c, "启用号池前必须设置 API Key")
		return
	}
	if err := app.db.Save(&pool).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, videoAccountPoolResponse(pool))
}

func (app *App) deleteVideoAccountPool(c *gin.Context) {
	pool, ok := app.findVideoAccountPool(c)
	if !ok {
		return
	}
	if err := app.db.Delete(&pool).Error; err != nil {
		serverError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (app *App) startVideoAccountPoolTest(c *gin.Context) {
	pool, ok := app.findVideoAccountPool(c)
	if !ok {
		return
	}
	var req VideoAccountPoolTestRequest
	if !bindJSON(c, &req) {
		return
	}
	params, err := normalizeVideoAccountPoolTestRequest(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	result, err := submitVideoAccountPoolTest(c.Request.Context(), pool, params)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) getVideoAccountPoolTest(c *gin.Context) {
	pool, ok := app.findVideoAccountPool(c)
	if !ok {
		return
	}
	taskID := strings.TrimSpace(c.Param("taskId"))
	if !videoTaskIDPattern.MatchString(taskID) {
		badRequest(c, "视频任务 ID 无效")
		return
	}
	result, err := queryVideoAccountPoolTest(c.Request.Context(), pool, taskID)
	if err != nil {
		c.JSON(http.StatusBadGateway, APIError{Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func submitVideoAccountPoolTest(ctx context.Context, pool VideoAccountPool, params VideoAccountPoolTestRequest) (VideoAccountPoolTestResponse, error) {
	payload := map[string]any{
		"model":        params.Model,
		"prompt":       params.Prompt,
		"duration":     params.Duration,
		"aspect_ratio": params.AspectRatio,
		"resolution":   params.Resolution,
	}
	if len(params.Images) > 0 {
		payload["images"] = params.Images
	}
	if params.VideoURL != "" {
		payload["video_url"] = params.VideoURL
	}
	return requestVideoAccountPoolTest(ctx, pool, http.MethodPost, "", payload)
}

func normalizeVideoAccountPoolTestRequest(req VideoAccountPoolTestRequest) (VideoAccountPoolTestRequest, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = videoPoolTestModel
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = videoPoolTestPrompt
	}
	duration := req.Duration
	if duration == 0 {
		duration = 6
	}
	aspectRatio := strings.TrimSpace(req.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "16:9"
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "480p"
	}
	images := make([]string, 0, len(req.Images))
	for _, image := range req.Images {
		imageURL, err := normalizeHTTPURL(strings.TrimSpace(image))
		if err != nil {
			return VideoAccountPoolTestRequest{}, errors.New("图片素材必须是有效的 HTTP/HTTPS 直链")
		}
		images = append(images, imageURL)
	}
	if len(images) > 7 {
		return VideoAccountPoolTestRequest{}, errors.New("图片素材最多支持 7 张")
	}
	videoURL := ""
	if strings.TrimSpace(req.VideoURL) != "" {
		var err error
		videoURL, err = normalizeHTTPURL(strings.TrimSpace(req.VideoURL))
		if err != nil {
			return VideoAccountPoolTestRequest{}, errors.New("视频素材必须是有效的 HTTP/HTTPS 直链")
		}
	}
	if videoURL != "" && len(images) > 0 {
		return VideoAccountPoolTestRequest{}, errors.New("视频编辑不能同时传入 images 和 video_url")
	}
	if !videoModelPattern.MatchString(model) {
		return VideoAccountPoolTestRequest{}, errors.New("模型 ID 只能包含字母、数字、下划线、点、斜线、冒号或短横线")
	}
	if len([]rune(prompt)) == 0 || len([]rune(prompt)) > 4096 {
		return VideoAccountPoolTestRequest{}, errors.New("提示词不能为空且不能超过 4096 个字符")
	}
	if duration != 6 && duration != 10 && duration != 15 {
		return VideoAccountPoolTestRequest{}, errors.New("时长仅支持 6、10 或 15 秒")
	}
	if aspectRatio != "16:9" && aspectRatio != "9:16" {
		return VideoAccountPoolTestRequest{}, errors.New("比例仅支持 16:9 或 9:16")
	}
	if resolution != "480p" && resolution != "720p" {
		return VideoAccountPoolTestRequest{}, errors.New("分辨率仅支持 480p 或 720p")
	}
	return VideoAccountPoolTestRequest{Model: model, Prompt: prompt, Images: images, VideoURL: videoURL, Duration: duration, AspectRatio: aspectRatio, Resolution: resolution}, nil
}

func queryVideoAccountPoolTest(ctx context.Context, pool VideoAccountPool, taskID string) (VideoAccountPoolTestResponse, error) {
	if !videoTaskIDPattern.MatchString(taskID) {
		return VideoAccountPoolTestResponse{}, errors.New("视频任务 ID 无效")
	}
	return requestVideoAccountPoolTest(ctx, pool, http.MethodGet, "/"+url.PathEscape(taskID), nil)
}

func requestVideoAccountPoolTest(ctx context.Context, pool VideoAccountPool, method, path string, payload any) (VideoAccountPoolTestResponse, error) {
	if pool.Format != videoPoolFormatOpenAIVideos {
		return VideoAccountPoolTestResponse{}, errors.New("该号池格式暂不支持接口测试")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(pool.BaseURL), "/")
	if _, err := normalizeVideoAccountPoolRequest(VideoAccountPoolRequest{
		Name: pool.Name, Format: pool.Format, BaseURL: baseURL, BaseURLIsComplete: pool.BaseURLIsComplete, APIKey: pool.APIKey,
	}, false); err != nil {
		return VideoAccountPoolTestResponse{}, fmt.Errorf("号池配置无效：%w", err)
	}
	apiKey := strings.TrimSpace(pool.APIKey)
	if apiKey == "" {
		return VideoAccountPoolTestResponse{}, errors.New("号池尚未设置 API Key")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return VideoAccountPoolTestResponse{}, err
		}
		body = bytes.NewReader(raw)
	}
	requestCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, videoAccountPoolVideosURL(pool)+path, body)
	if err != nil {
		return VideoAccountPoolTestResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	statusCode, respBody, err := doVideoPoolHTTPRequest(req)
	if err != nil {
		return VideoAccountPoolTestResponse{}, fmt.Errorf("请求视频上游失败：%w", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		return VideoAccountPoolTestResponse{}, parseVideoUpstreamError(statusCode, respBody)
	}
	var upstream openAIVideoTaskResponse
	if err := json.Unmarshal(respBody, &upstream); err != nil {
		return VideoAccountPoolTestResponse{}, errors.New("视频上游返回了无效的 JSON")
	}
	return normalizeVideoAccountPoolTestResponse(upstream)
}

func doVideoPoolHTTPRequest(req *http.Request) (int, []byte, error) {
	attempts := 1
	if req.Method == http.MethodGet {
		attempts = 3
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		resp, err := videoPoolHTTPClient.Do(req.Clone(req.Context()))
		if err == nil {
			respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			_ = resp.Body.Close()
			if readErr == nil {
				return resp.StatusCode, respBody, nil
			}
			err = readErr
		}
		lastErr = err
		if attempt+1 >= attempts || !isUnexpectedEOF(err) {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return 0, nil, req.Context().Err()
		case <-timer.C:
		}
	}
	return 0, nil, lastErr
}

func isUnexpectedEOF(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "unexpected eof")
}

func normalizeVideoAccountPoolTestResponse(upstream openAIVideoTaskResponse) (VideoAccountPoolTestResponse, error) {
	taskID := strings.TrimSpace(upstream.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(upstream.ID)
	}
	if !videoTaskIDPattern.MatchString(taskID) {
		return VideoAccountPoolTestResponse{}, errors.New("视频上游响应缺少有效的任务 ID")
	}
	status := strings.ToLower(strings.TrimSpace(upstream.Status))
	if status == "" {
		status = "queued"
	}
	switch status {
	case "queued", "in_progress", "completed", "failed":
	default:
		return VideoAccountPoolTestResponse{}, fmt.Errorf("视频上游返回了未知任务状态：%s", status)
	}
	if upstream.Progress < 0 || upstream.Progress > 100 {
		return VideoAccountPoolTestResponse{}, errors.New("视频上游返回了无效的任务进度")
	}
	result := VideoAccountPoolTestResponse{
		ID: strings.TrimSpace(upstream.ID), TaskID: taskID, Model: strings.TrimSpace(upstream.Model),
		Status: status, Progress: upstream.Progress, CreatedAt: upstream.CreatedAt, CompletedAt: upstream.CompletedAt,
	}
	if result.ID == "" {
		result.ID = taskID
	}
	if status == "completed" {
		videoURL, err := normalizeHTTPURL(strings.TrimSpace(upstream.Metadata.URL))
		if err != nil {
			return VideoAccountPoolTestResponse{}, errors.New("视频任务已完成，但上游未返回有效的视频 URL")
		}
		result.VideoURL = videoURL
	}
	if upstream.Error != nil && strings.TrimSpace(upstream.Error.Message) != "" {
		result.Error = &VideoAccountPoolTestError{Code: strings.TrimSpace(upstream.Error.Code), Message: strings.TrimSpace(upstream.Error.Message)}
	}
	if status == "failed" && result.Error == nil {
		result.Error = &VideoAccountPoolTestError{Message: "视频上游未提供失败原因"}
	}
	return result, nil
}

func parseVideoUpstreamError(statusCode int, body []byte) error {
	var upstream videoUpstreamErrorResponse
	if json.Unmarshal(body, &upstream) == nil {
		message := strings.TrimSpace(upstream.Message)
		code := strings.TrimSpace(upstream.Code)
		if upstream.Error != nil {
			if strings.TrimSpace(upstream.Error.Message) != "" {
				message = strings.TrimSpace(upstream.Error.Message)
			}
			if strings.TrimSpace(upstream.Error.Code) != "" {
				code = strings.TrimSpace(upstream.Error.Code)
			}
		}
		if message != "" {
			if code != "" {
				return fmt.Errorf("视频上游请求失败（%s）：%s", code, message)
			}
			return fmt.Errorf("视频上游请求失败：%s", message)
		}
	}
	return fmt.Errorf("视频上游请求失败：HTTP %d", statusCode)
}

func (app *App) findVideoAccountPool(c *gin.Context) (VideoAccountPool, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		badRequest(c, "号池 ID 无效")
		return VideoAccountPool{}, false
	}
	var pool VideoAccountPool
	if err := app.db.First(&pool, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "视频号池不存在"})
			return VideoAccountPool{}, false
		}
		serverError(c, err)
		return VideoAccountPool{}, false
	}
	return pool, true
}

func normalizeVideoAccountPoolRequest(req VideoAccountPoolRequest, creating bool) (VideoAccountPool, error) {
	name := strings.TrimSpace(req.Name)
	format := strings.ToLower(strings.TrimSpace(req.Format))
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	apiKey := strings.TrimSpace(req.APIKey)
	if name == "" {
		return VideoAccountPool{}, errors.New("号池名称不能为空")
	}
	if len([]rune(name)) > 100 {
		return VideoAccountPool{}, errors.New("号池名称不能超过 100 个字符")
	}
	if format != videoPoolFormatOpenAIVideos {
		return VideoAccountPool{}, errors.New("暂不支持该接口格式")
	}
	parsed, err := url.ParseRequestURI(baseURL)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return VideoAccountPool{}, errors.New("Base URL 必须是不含账号、查询参数和片段的有效 HTTP/HTTPS 地址")
	}
	if len(baseURL) > 500 {
		return VideoAccountPool{}, errors.New("Base URL 不能超过 500 个字符")
	}
	if len(apiKey) > 8192 {
		return VideoAccountPool{}, errors.New("API Key 长度不能超过 8192 个字符")
	}
	if creating && apiKey == "" {
		return VideoAccountPool{}, errors.New("创建号池时必须填写 API Key")
	}
	if req.ClearAPIKey && apiKey != "" {
		return VideoAccountPool{}, errors.New("不能同时填写并清除 API Key")
	}
	if req.Enabled && creating && apiKey == "" {
		return VideoAccountPool{}, errors.New("启用号池前必须设置 API Key")
	}
	return VideoAccountPool{Name: name, Format: format, BaseURL: baseURL, BaseURLIsComplete: req.BaseURLIsComplete, APIKey: apiKey, Enabled: req.Enabled}, nil
}

func videoAccountPoolVideosURL(pool VideoAccountPool) string {
	baseURL := strings.TrimRight(strings.TrimSpace(pool.BaseURL), "/")
	if pool.BaseURLIsComplete {
		return baseURL
	}
	return baseURL + "/v1/videos"
}

func videoAccountPoolResponse(pool VideoAccountPool) VideoAccountPoolResponse {
	return VideoAccountPoolResponse{
		ID: pool.ID, Name: pool.Name, Format: pool.Format, BaseURL: pool.BaseURL, BaseURLIsComplete: pool.BaseURLIsComplete,
		APIKeySet: strings.TrimSpace(pool.APIKey) != "", Enabled: pool.Enabled,
		CreatedAt: pool.CreatedAt, UpdatedAt: pool.UpdatedAt,
	}
}
