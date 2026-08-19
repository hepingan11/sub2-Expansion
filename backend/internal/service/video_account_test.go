package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type videoTestUpstream struct {
	responses []*http.Response
	requests  []*http.Request
}

func (u *videoTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.requests = append(u.requests, req)
	if len(u.responses) == 0 {
		return nil, io.EOF
	}
	response := u.responses[0]
	u.responses = u.responses[1:]
	return response, nil
}

func (u *videoTestUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func videoAccountForTest(complete bool) *Account {
	return &Account{
		ID: 7, Platform: PlatformVideo, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":             "https://video.example.test",
			"api_key":              "secret",
			"format":               VideoAccountFormatOpenAIVideos,
			"base_url_is_complete": complete,
			"model_mapping":        map[string]any{"seedance-2.0": "provider-video"},
		},
	}
}

func comfyUIVideoAccountForTest() *Account {
	return &Account{ID: 8, Platform: PlatformVideo, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url": "https://autodl.art", "api_key": "raw-token", "format": VideoAccountFormatComfyUI,
		"workflow_id": "default_workflow", "model_mapping": map[string]any{"seedance-2.0": "mapped_workflow"},
	}}
}

func TestNormalizeVideoAccountCredentialsRejectsUnsafeValues(t *testing.T) {
	account := videoAccountForTest(false)
	require.NoError(t, NormalizeVideoAccountCredentials(account.Platform, account.Type, account.Credentials, true))

	for _, baseURL := range []string{"video.example.test", "https://user:pass@video.example.test", "https://video.example.test?x=1"} {
		credentials := map[string]any{"base_url": baseURL, "api_key": "secret"}
		require.Error(t, NormalizeVideoAccountCredentials(PlatformVideo, AccountTypeAPIKey, credentials, true), baseURL)
	}
	require.Error(t, NormalizeVideoAccountCredentials(PlatformVideo, "oauth", account.Credentials, true))
}

func TestVideoAccountURLs(t *testing.T) {
	account := videoAccountForTest(false)
	require.Equal(t, "https://video.example.test/v1/videos", videoAccountVideosURL(account))
	require.Equal(t, "https://video.example.test/v1/models", videoAccountModelsURL(account))

	account.Credentials["base_url"] = "https://video.example.test/v1/videos"
	account.Credentials["base_url_is_complete"] = true
	require.Equal(t, "https://video.example.test/v1/videos", videoAccountVideosURL(account))
	require.Equal(t, "https://video.example.test/v1/models", videoAccountModelsURL(account))
}

func TestNormalizeComfyUIVideoAccountCredentials(t *testing.T) {
	account := comfyUIVideoAccountForTest()
	require.NoError(t, NormalizeVideoAccountCredentials(account.Platform, account.Type, account.Credentials, true))
	require.Equal(t, "https://autodl.art/api/v1/comfyui/comfyui_workflow/mapped_workflow", videoAccountComfyCreateURL(account, videoAccountComfyWorkflowID(account, "seedance-2.0")))
	require.Equal(t, "https://autodl.art/api/v1/comfyui/comfyui_workflow/result/task_123", videoAccountComfyResultURL(account, "task_123"))

	for _, workflow := range []string{"../escape", "bad/workflow", ""} {
		credentials := map[string]any{"base_url": "https://autodl.art", "api_key": "token", "format": VideoAccountFormatComfyUI, "workflow_id": workflow}
		require.Error(t, NormalizeVideoAccountCredentials(PlatformVideo, AccountTypeAPIKey, credentials, true), workflow)
	}
}

func TestForwardVideoAccountConvertsComfyUIResponses(t *testing.T) {
	upstream := &videoTestUpstream{responses: []*http.Response{{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"code":"Success","data":{"task_id":"task_123","status":"QUEUED","results":[]}}`)),
	}, {
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"code":"Success","data":{"task_id":"task_123","status":"SUCCESS","results":[{"type":"video","url":"https://cdn.example.test/video.mp4"}]}}`)),
	}}}
	service := &OpenAIGatewayService{httpUpstream: upstream}
	account := comfyUIVideoAccountForTest()
	gin.SetMode(gin.TestMode)

	createRecorder := httptest.NewRecorder()
	createCtx, _ := gin.CreateTestContext(createRecorder)
	createCtx.Request, _ = http.NewRequest(http.MethodPost, "/v1/videos", nil)
	_, err := service.ForwardVideoAccount(context.Background(), createCtx, account, GrokMediaEndpointVideosGenerations, "", []byte(`{"model":"seedance-2.0","prompt":"ocean","duration":1,"resolution":"480p竖"}`), "application/json")
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"task_123","task_id":"task_123","status":"queued"}`, createRecorder.Body.String())
	require.Equal(t, "https://autodl.art/api/v1/comfyui/comfyui_workflow/mapped_workflow", upstream.requests[0].URL.String())
	require.Equal(t, "raw-token", upstream.requests[0].Header.Get("Authorization"))
	forwarded, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.NotContains(t, string(forwarded), `"model"`)

	statusRecorder := httptest.NewRecorder()
	statusCtx, _ := gin.CreateTestContext(statusRecorder)
	statusCtx.Request, _ = http.NewRequest(http.MethodGet, "/v1/videos/task_123", nil)
	result, err := service.ForwardVideoAccount(context.Background(), statusCtx, account, GrokMediaEndpointVideoStatus, "task_123", nil, "")
	require.NoError(t, err)
	require.Equal(t, 1, result.VideoCount)
	require.Contains(t, statusRecorder.Body.String(), `"status":"completed"`)
	require.Contains(t, statusRecorder.Body.String(), `https://cdn.example.test/video.mp4`)
	require.Equal(t, "https://autodl.art/api/v1/comfyui/comfyui_workflow/result/task_123", upstream.requests[1].URL.String())
}

func TestForwardVideoAccountUsesOpenAIVideosWireFormat(t *testing.T) {
	upstream := &videoTestUpstream{responses: []*http.Response{{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_123","status":"queued"}`)),
	}, {
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_123","status":"completed","metadata":{"url":"https://cdn.example.test/video.mp4"}}`)),
	}}}
	service := &OpenAIGatewayService{httpUpstream: upstream}
	account := videoAccountForTest(false)

	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		endpoint GrokMediaEndpoint
		id       string
		body     string
	}{
		{GrokMediaEndpointVideosGenerations, "", `{"model":"seedance-2.0","prompt":"ocean"}`},
		{GrokMediaEndpointVideoStatus, "task_123", ""},
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, "http://gateway.test/v1/videos", bytes.NewReader([]byte(tc.body)))
		result, err := service.ForwardVideoAccount(context.Background(), ctx, account, tc.endpoint, tc.id, []byte(tc.body), "application/json")
		require.NoError(t, err)
		require.NotNil(t, result)
	}
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "POST", upstream.requests[0].Method)
	require.Equal(t, "https://video.example.test/v1/videos", upstream.requests[0].URL.String())
	require.Equal(t, "Bearer secret", upstream.requests[0].Header.Get("Authorization"))
	forwardedBody, err := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, err)
	require.Contains(t, string(forwardedBody), `"model":"provider-video"`)
	require.Equal(t, "GET", upstream.requests[1].Method)
	require.Equal(t, "https://video.example.test/v1/videos/task_123", upstream.requests[1].URL.String())
}

func TestForwardVideoAccountRejectsUnsafeTaskID(t *testing.T) {
	service := &OpenAIGatewayService{httpUpstream: &videoTestUpstream{}}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request, _ = http.NewRequest(http.MethodGet, "http://gateway.test/v1/videos/x", nil)
	_, err := service.ForwardVideoAccount(context.Background(), ctx, videoAccountForTest(false), GrokMediaEndpointVideoStatus, "../escape", nil, "")
	require.Error(t, err)
}
