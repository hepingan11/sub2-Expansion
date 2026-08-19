package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

const (
	defaultVideoAccountTestModel      = "seedance-2.0"
	defaultVideoAccountTestPrompt     = "A calm ocean wave at sunrise."
	defaultVideoAccountTestDuration   = 6
	defaultVideoAccountTestAspect     = "16:9"
	defaultVideoAccountTestResolution = "480p"
)

var videoAccountTestPollInterval = 2 * time.Second
var videoAccountTestTimeout = 3 * time.Minute

type videoAccountTaskResponse struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	Model       string `json:"model"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	URL         string `json:"url"`
	VideoURL    string `json:"video_url"`
	OutputURL   string `json:"output_url"`
	DownloadURL string `json:"download_url"`
	Metadata    struct {
		URL string `json:"url"`
	} `json:"metadata"`
	Results json.RawMessage `json:"results"`
	Data    *struct {
		TaskID  string          `json:"task_id"`
		Status  string          `json:"status"`
		Results json.RawMessage `json:"results"`
		Message string          `json:"message"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (r *videoAccountTaskResponse) normalizeComfy() {
	if r.Data == nil {
		return
	}
	r.TaskID, r.Status, r.Results = r.Data.TaskID, r.Data.Status, r.Data.Results
	if r.Error == nil && strings.TrimSpace(r.Data.Message) != "" {
		r.Error = &struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{Message: r.Data.Message}
	}
}

func (s *AccountTestService) testVideoAccountConnection(c *gin.Context, account *Account, modelID, prompt string, opts AccountTestOptions) error {
	if s.httpUpstream == nil {
		return s.sendErrorAndEnd(c, "HTTP upstream not configured")
	}
	if err := NormalizeVideoAccountCredentials(account.Platform, account.Type, account.Credentials, false); err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Invalid Video account credentials: %s", err.Error()))
	}
	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return s.sendErrorAndEnd(c, "No Video API key is available")
	}

	params, err := normalizeVideoAccountTestOptions(modelID, prompt, opts)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}

	s.prepareVideoAccountTestSSE(c)
	s.sendEvent(c, TestEvent{Type: "test_start", Model: params.Model})
	s.sendEvent(c, TestEvent{Type: "status", Text: "Submitting OpenAI Videos request..."})

	ctx, cancel := context.WithTimeout(c.Request.Context(), videoAccountTestTimeout)
	defer cancel()
	created, err := s.requestVideoAccountTask(ctx, account, apiKey, http.MethodPost, "", params.payload())
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	created.normalizeComfy()
	taskID := strings.TrimSpace(created.TaskID)
	if taskID == "" {
		taskID = strings.TrimSpace(created.ID)
	}
	if !videoAccountTaskIDPattern.MatchString(taskID) {
		return s.sendErrorAndEnd(c, "Video upstream response did not include a valid task ID")
	}

	lastProgress := -1
	for {
		status := strings.ToLower(strings.TrimSpace(created.Status))
		if status == "" {
			status = "queued"
		}
		if created.Progress != lastProgress {
			s.sendEvent(c, TestEvent{Type: "status", Text: fmt.Sprintf("Video task %s: %s (%d%%)", taskID, status, created.Progress)})
			lastProgress = created.Progress
		}
		switch status {
		case "completed", "succeeded", "success":
			videoURL := firstVideoTaskURL(created)
			if videoURL == "" {
				return s.sendErrorAndEnd(c, "Video task completed without a playable video URL")
			}
			s.sendEvent(c, TestEvent{Type: "video", VideoURL: videoURL, MimeType: "video/mp4"})
			s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
			return nil
		case "failed", "cancelled", "canceled":
			message := "Video task failed"
			if created.Error != nil && strings.TrimSpace(created.Error.Message) != "" {
				message = strings.TrimSpace(created.Error.Message)
			}
			return s.sendErrorAndEnd(c, message)
		case "queued", "pending", "in_progress", "processing", "running":
		default:
			return s.sendErrorAndEnd(c, fmt.Sprintf("Video upstream returned an unknown task status: %s", status))
		}

		timer := time.NewTimer(videoAccountTestPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return s.sendErrorAndEnd(c, "Timed out waiting for the video task to finish")
			}
			return ctx.Err()
		case <-timer.C:
		}
		created, err = s.requestVideoAccountTask(ctx, account, apiKey, http.MethodGet, "/"+url.PathEscape(taskID), nil)
		if err != nil {
			return s.sendErrorAndEnd(c, err.Error())
		}
		created.normalizeComfy()
	}
}

type normalizedVideoAccountTestOptions struct {
	Model       string
	Prompt      string
	Images      []string
	VideoURL    string
	Duration    int
	AspectRatio string
	Resolution  string
}

func normalizeVideoAccountTestOptions(modelID, prompt string, opts AccountTestOptions) (normalizedVideoAccountTestOptions, error) {
	result := normalizedVideoAccountTestOptions{
		Model: strings.TrimSpace(modelID), Prompt: strings.TrimSpace(prompt),
		Images: opts.VideoImages, VideoURL: strings.TrimSpace(opts.VideoURL),
		Duration: opts.VideoDuration, AspectRatio: strings.TrimSpace(opts.VideoAspectRatio),
		Resolution: strings.TrimSpace(opts.VideoResolution),
	}
	if result.Model == "" {
		result.Model = defaultVideoAccountTestModel
	}
	if result.Prompt == "" {
		result.Prompt = defaultVideoAccountTestPrompt
	}
	if result.Duration == 0 {
		result.Duration = defaultVideoAccountTestDuration
	}
	if result.AspectRatio == "" {
		result.AspectRatio = defaultVideoAccountTestAspect
	}
	if result.Resolution == "" {
		result.Resolution = defaultVideoAccountTestResolution
	}
	if !videoAccountModelPattern.MatchString(result.Model) {
		return normalizedVideoAccountTestOptions{}, errors.New("Invalid video model ID")
	}
	if len([]rune(result.Prompt)) > 4096 {
		return normalizedVideoAccountTestOptions{}, errors.New("Video prompt must not exceed 4096 characters")
	}
	if result.Duration < 1 {
		return normalizedVideoAccountTestOptions{}, errors.New("Video duration must be at least 1 second")
	}
	validAspect := map[string]bool{"16:9": true, "9:16": true, "1:1": true, "4:3": true, "3:4": true}
	if !validAspect[result.AspectRatio] {
		return normalizedVideoAccountTestOptions{}, errors.New("Unsupported video aspect ratio")
	}
	if len([]rune(result.Resolution)) > 64 || strings.ContainsAny(result.Resolution, "\r\n") {
		return normalizedVideoAccountTestOptions{}, errors.New("Video resolution is invalid")
	}
	if result.VideoURL != "" && len(result.Images) > 0 {
		return normalizedVideoAccountTestOptions{}, errors.New("Video images and video_url cannot be used together")
	}
	for _, rawURL := range append(append([]string{}, result.Images...), result.VideoURL) {
		if strings.TrimSpace(rawURL) == "" {
			continue
		}
		parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return normalizedVideoAccountTestOptions{}, errors.New("Video source media must use valid HTTP/HTTPS URLs")
		}
	}
	if len(result.Images) > 7 {
		return normalizedVideoAccountTestOptions{}, errors.New("Video tests support up to 7 image URLs")
	}
	return result, nil
}

var regexpVideoResolution = regexp.MustCompile(`^[1-9][0-9]{0,5}p$`)

func (p normalizedVideoAccountTestOptions) payload() map[string]any {
	payload := map[string]any{
		"model": p.Model, "prompt": p.Prompt, "duration": p.Duration,
		"aspect_ratio": p.AspectRatio, "resolution": p.Resolution,
	}
	if len(p.Images) > 0 {
		payload["images"] = p.Images
	}
	if p.VideoURL != "" {
		payload["video_url"] = p.VideoURL
	}
	return payload
}

func (s *AccountTestService) requestVideoAccountTask(ctx context.Context, account *Account, apiKey, method, path string, payload any) (videoAccountTaskResponse, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return videoAccountTaskResponse{}, err
		}
		body = bytes.NewReader(raw)
	}
	targetURL := videoAccountVideosURL(account) + path
	if videoAccountFormat(account) == VideoAccountFormatComfyUI {
		if method == http.MethodPost {
			model := ""
			if values, ok := payload.(map[string]any); ok {
				model, _ = values["model"].(string)
				delete(values, "model")
				raw, marshalErr := json.Marshal(values)
				if marshalErr != nil {
					return videoAccountTaskResponse{}, marshalErr
				}
				body = bytes.NewReader(raw)
			}
			workflowID := videoAccountComfyWorkflowID(account, model)
			if !videoAccountWorkflowIDPattern.MatchString(workflowID) {
				return videoAccountTaskResponse{}, errors.New("Invalid ComfyUI workflow ID")
			}
			targetURL = videoAccountComfyCreateURL(account, workflowID)
		} else {
			taskID := strings.TrimPrefix(path, "/")
			if !videoAccountTaskIDPattern.MatchString(taskID) {
				return videoAccountTaskResponse{}, errors.New("Invalid Video task ID")
			}
			targetURL = videoAccountComfyResultURL(account, taskID)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return videoAccountTaskResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	if videoAccountFormat(account) == VideoAccountFormatComfyUI {
		req.Header.Set("Authorization", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	account.ApplyHeaderOverrides(req.Header)
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	var profile *tlsfingerprint.Profile
	if s.tlsFPProfileService != nil {
		profile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, profile)
	if err != nil {
		return videoAccountTaskResponse{}, fmt.Errorf("Video upstream request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return videoAccountTaskResponse{}, fmt.Errorf("Failed to read Video upstream response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return videoAccountTaskResponse{}, fmt.Errorf("Video upstream returned HTTP %d: %s", resp.StatusCode, videoAccountUpstreamError(responseBody))
	}
	var result videoAccountTaskResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return videoAccountTaskResponse{}, errors.New("Video upstream returned invalid JSON")
	}
	return result, nil
}

func videoAccountUpstreamError(body []byte) string {
	var payload struct {
		Message string `json:"message"`
		Error   *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil {
		if payload.Error != nil && strings.TrimSpace(payload.Error.Message) != "" {
			return strings.TrimSpace(payload.Error.Message)
		}
		if strings.TrimSpace(payload.Message) != "" {
			return strings.TrimSpace(payload.Message)
		}
	}
	return "request failed"
}

func firstVideoTaskURL(task videoAccountTaskResponse) string {
	for _, candidate := range []string{task.Metadata.URL, task.VideoURL, task.OutputURL, task.URL, task.DownloadURL} {
		candidate = strings.TrimSpace(candidate)
		parsed, err := url.ParseRequestURI(candidate)
		if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return candidate
		}
	}
	if len(task.Results) > 0 {
		return firstComfyResultURL(task.Results)
	}
	return ""
}

func (s *AccountTestService) prepareVideoAccountTestSSE(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.Flush()
}
