package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ForwardVideoAccount forwards the OpenAI Videos create and status endpoints
// without applying xAI-specific model aliases or response rewrites.
func (s *OpenAIGatewayService) ForwardVideoAccount(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startedAt := time.Now()
	if account == nil || !account.IsVideo() {
		return nil, fmt.Errorf("video account is required")
	}
	if endpoint != GrokMediaEndpointVideosGenerations && endpoint != GrokMediaEndpointVideoStatus {
		return nil, fmt.Errorf("video endpoint %s is not supported", endpoint)
	}
	if err := NormalizeVideoAccountCredentials(account.Platform, account.Type, account.Credentials, false); err != nil {
		return nil, err
	}

	format := videoAccountFormat(account)
	targetURL := videoAccountVideosURL(account)
	method := http.MethodPost
	if endpoint == GrokMediaEndpointVideoStatus {
		if !videoAccountTaskIDPattern.MatchString(strings.TrimSpace(requestID)) {
			return nil, fmt.Errorf("invalid video request ID")
		}
		method = http.MethodGet
		if format == VideoAccountFormatComfyUI {
			targetURL = videoAccountComfyResultURL(account, requestID)
		} else {
			targetURL += "/" + requestID
		}
	}

	requestInfo := ParseGrokMediaRequest(contentType, body)
	upstreamModel := requestInfo.Model
	if method == http.MethodPost && gjson.ValidBytes(body) {
		if mapped := strings.TrimSpace(account.GetMappedModel(requestInfo.Model)); mapped != "" {
			upstreamModel = mapped
		}
		if upstreamModel != requestInfo.Model && format == VideoAccountFormatOpenAIVideos {
			var err error
			body, err = sjson.SetBytes(body, "model", upstreamModel)
			if err != nil {
				return nil, fmt.Errorf("rewrite video account mapped model: %w", err)
			}
		}
		if format == VideoAccountFormatComfyUI {
			workflowID := videoAccountComfyWorkflowID(account, requestInfo.Model)
			if !videoAccountWorkflowIDPattern.MatchString(workflowID) {
				return nil, fmt.Errorf("invalid ComfyUI workflow ID")
			}
			targetURL = videoAccountComfyCreateURL(account, workflowID)
			body, _ = sjson.DeleteBytes(body, "model")
		}
	}

	var requestBody *bytes.Reader
	if method == http.MethodPost {
		requestBody = bytes.NewReader(body)
	} else {
		requestBody = bytes.NewReader(nil)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	request, err := http.NewRequestWithContext(upstreamCtx, method, targetURL, requestBody)
	if err != nil {
		return nil, err
	}
	if format == VideoAccountFormatComfyUI {
		request.Header.Set("Authorization", strings.TrimSpace(account.GetCredential("api_key")))
	} else {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(account.GetCredential("api_key")))
	}
	request.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		if strings.TrimSpace(contentType) == "" {
			contentType = "application/json"
		}
		request.Header.Set("Content-Type", contentType)
	}
	account.ApplyHeaderOverrides(request.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStartedAt := time.Now()
	response, err := s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStartedAt).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = response.Body.Close() }()

	requestIDHeader := firstNonEmpty(response.Header.Get("x-request-id"), response.Header.Get("request-id"))
	if response.StatusCode >= http.StatusBadRequest {
		return s.handleErrorResponse(ctx, response, c, account, body, requestInfo.Model)
	}
	responseBody, err := ReadUpstreamResponseBody(response.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if format == VideoAccountFormatComfyUI {
		responseBody = normalizeComfyUIVideoResponse(responseBody, requestID)
	}
	writeGrokMediaResponse(c, response, responseBody, s.responseHeaderFilter)

	result := &OpenAIForwardResult{
		RequestID:            requestIDHeader,
		ResponseID:           extractGrokMediaVideoRequestID(responseBody),
		Model:                requestInfo.Model,
		BillingModel:         requestInfo.Model,
		UpstreamModel:        upstreamModel,
		ResponseHeaders:      response.Header.Clone(),
		Duration:             time.Since(startedAt),
		VideoResolution:      requestInfo.Resolution,
		VideoDurationSeconds: requestInfo.DurationSeconds,
	}
	if endpoint == GrokMediaEndpointVideoStatus {
		result.ResponseID = firstNonEmpty(result.ResponseID, strings.TrimSpace(requestID))
		if responseModel := strings.TrimSpace(gjson.GetBytes(responseBody, "model").String()); responseModel != "" {
			result.Model = responseModel
			result.BillingModel = responseModel
		}
		status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "status").String()))
		if status == "completed" || status == "succeeded" {
			for _, path := range []string{"metadata.url", "video_url", "output_url", "url", "download_url"} {
				if strings.TrimSpace(gjson.GetBytes(responseBody, path).String()) != "" {
					result.VideoCount = 1
					break
				}
			}
		}
	}
	return result, nil
}

func normalizeComfyUIVideoResponse(body []byte, fallbackID string) []byte {
	var envelope struct {
		Data struct {
			TaskID  string          `json:"task_id"`
			Status  string          `json:"status"`
			Results json.RawMessage `json:"results"`
			Message string          `json:"message"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Data.TaskID == "" {
		return body
	}
	status := map[string]string{"queued": "queued", "running": "processing", "success": "completed", "failed": "failed"}[strings.ToLower(strings.TrimSpace(envelope.Data.Status))]
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(envelope.Data.Status))
	}
	id := envelope.Data.TaskID
	if id == "" {
		id = fallbackID
	}
	result := map[string]any{"id": id, "task_id": id, "status": status}
	if envelope.Data.Message != "" {
		result["message"] = envelope.Data.Message
	}
	if len(envelope.Data.Results) > 0 {
		if u := firstComfyResultURL(envelope.Data.Results); u != "" {
			result["metadata"] = map[string]string{"url": u}
		}
	}
	out, _ := json.Marshal(result)
	return out
}

func firstComfyResultURL(raw json.RawMessage) string {
	var values []any
	if json.Unmarshal(raw, &values) != nil {
		var single map[string]any
		if json.Unmarshal(raw, &single) == nil {
			for _, key := range []string{"url", "video_url", "output_url", "download_url"} {
				if value, ok := single[key].(string); ok && isHTTPURL(value) {
					return value
				}
			}
		}
		return ""
	}
	for _, value := range values {
		switch item := value.(type) {
		case string:
			if isHTTPURL(item) {
				return item
			}
		case map[string]any:
			for _, key := range []string{"url", "video_url", "output_url", "download_url"} {
				if s, ok := item[key].(string); ok && isHTTPURL(s) {
					return s
				}
			}
		}
	}
	return ""
}

func isHTTPURL(raw string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}
