package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type videoRoundTripFunc func(*http.Request) (*http.Response, error)

func (f videoRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNormalizeVideoAccountPoolRequest(t *testing.T) {
	pool, err := normalizeVideoAccountPoolRequest(VideoAccountPoolRequest{
		Name: " 主号池 ", Format: "openai_videos", BaseURL: "https://video.example.com/", APIKey: " secret ", Enabled: true,
	}, true)
	if err != nil {
		t.Fatalf("normalizeVideoAccountPoolRequest() error = %v", err)
	}
	if pool.Name != "主号池" || pool.BaseURL != "https://video.example.com" || pool.APIKey != "secret" {
		t.Fatalf("unexpected normalized pool: %#v", pool)
	}
}

func TestVideoAccountPoolVideosURL(t *testing.T) {
	basePool := VideoAccountPool{BaseURL: "https://video.example.com/"}
	if got := videoAccountPoolVideosURL(basePool); got != "https://video.example.com/v1/videos" {
		t.Fatalf("videoAccountPoolVideosURL(base) = %q", got)
	}
	completePool := VideoAccountPool{BaseURL: "https://video.example.com/custom/generate/", BaseURLIsComplete: true}
	if got := videoAccountPoolVideosURL(completePool); got != "https://video.example.com/custom/generate" {
		t.Fatalf("videoAccountPoolVideosURL(complete) = %q", got)
	}
}

func TestVideoAccountPoolTestFlow(t *testing.T) {
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos":
			createCalled = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload["model"] != "seedance-2.0" || payload["prompt"] != "test prompt" || payload["resolution"] != "480p" || payload["aspect_ratio"] != "16:9" || payload["duration"] != float64(6) {
				t.Fatalf("unexpected test payload: %#v", payload)
			}
			if len(payload) != 5 {
				t.Fatalf("test payload contains unsupported fields: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_123","status":"queued","progress":0}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/task_123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"task_123","task_id":"task_123","model":"seedance-2.0","status":"completed","progress":100,"metadata":{"url":"https://cdn.example.com/video.mp4"}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	pool := VideoAccountPool{Name: "test", Format: videoPoolFormatOpenAIVideos, BaseURL: server.URL, APIKey: "secret-key"}
	created, err := submitVideoAccountPoolTest(context.Background(), pool, VideoAccountPoolTestRequest{
		Model: "seedance-2.0", Prompt: "test prompt", Duration: 6, AspectRatio: "16:9", Resolution: "480p",
	})
	if err != nil {
		t.Fatalf("submitVideoAccountPoolTest() error = %v", err)
	}
	if !createCalled || created.TaskID != "task_123" || created.Status != "queued" {
		t.Fatalf("unexpected create response: %#v", created)
	}
	completed, err := queryVideoAccountPoolTest(context.Background(), pool, created.TaskID)
	if err != nil {
		t.Fatalf("queryVideoAccountPoolTest() error = %v", err)
	}
	if completed.Status != "completed" || completed.Progress != 100 || completed.VideoURL != "https://cdn.example.com/video.mp4" {
		t.Fatalf("unexpected completed response: %#v", completed)
	}
}

func TestVideoPoolGETRetriesUnexpectedEOF(t *testing.T) {
	originalClient := videoPoolHTTPClient
	defer func() { videoPoolHTTPClient = originalClient }()
	attempts := 0
	videoPoolHTTPClient = &http.Client{Transport: videoRoundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"task_123"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	req, err := http.NewRequest(http.MethodGet, "https://video.example.com/v1/videos/task_123", nil)
	if err != nil {
		t.Fatal(err)
	}
	status, body, err := doVideoPoolHTTPRequest(req)
	if err != nil {
		t.Fatalf("doVideoPoolHTTPRequest() error = %v", err)
	}
	if attempts != 3 || status != http.StatusOK || string(body) != `{"id":"task_123"}` {
		t.Fatalf("attempts=%d status=%d body=%s", attempts, status, body)
	}
}

func TestVideoPoolPOSTDoesNotRetryUnexpectedEOF(t *testing.T) {
	originalClient := videoPoolHTTPClient
	defer func() { videoPoolHTTPClient = originalClient }()
	attempts := 0
	videoPoolHTTPClient = &http.Client{Transport: videoRoundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		return nil, io.ErrUnexpectedEOF
	})}
	req, err := http.NewRequest(http.MethodPost, "https://video.example.com/v1/videos", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := doVideoPoolHTTPRequest(req); !isUnexpectedEOF(err) {
		t.Fatalf("expected unexpected EOF, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("POST attempts = %d, want 1", attempts)
	}
}

func TestNormalizeVideoAccountPoolTestRequest(t *testing.T) {
	defaults, err := normalizeVideoAccountPoolTestRequest(VideoAccountPoolTestRequest{})
	if err != nil {
		t.Fatalf("default test request error = %v", err)
	}
	if defaults.Model != videoPoolTestModel || defaults.Prompt != videoPoolTestPrompt || defaults.Duration != 6 || defaults.AspectRatio != "16:9" || defaults.Resolution != "480p" {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}
	if _, err := normalizeVideoAccountPoolTestRequest(VideoAccountPoolTestRequest{Model: "bad model", Prompt: "x", Duration: 6, AspectRatio: "16:9", Resolution: "480p"}); err == nil {
		t.Fatal("expected invalid model to be rejected")
	}
	if _, err := normalizeVideoAccountPoolTestRequest(VideoAccountPoolTestRequest{Model: "seedance-2.0", Prompt: "x", Duration: 8, AspectRatio: "16:9", Resolution: "480p"}); err == nil {
		t.Fatal("expected invalid duration to be rejected")
	}
}

func TestVideoAccountPoolTestRejectsUnsafeTaskID(t *testing.T) {
	pool := VideoAccountPool{Name: "test", Format: videoPoolFormatOpenAIVideos, BaseURL: "https://example.com", APIKey: "secret"}
	if _, err := queryVideoAccountPoolTest(context.Background(), pool, "../settings"); err == nil {
		t.Fatal("expected unsafe task ID to be rejected")
	}
}

func TestNormalizeVideoAccountPoolTestResponseRejectsUnknownStatus(t *testing.T) {
	_, err := normalizeVideoAccountPoolTestResponse(openAIVideoTaskResponse{ID: "task_123", Status: "mystery"})
	if err == nil {
		t.Fatal("expected unknown status to be rejected")
	}
}

func TestNormalizeVideoAccountPoolRequestRejectsInvalidInput(t *testing.T) {
	tests := []VideoAccountPoolRequest{
		{Name: "", Format: videoPoolFormatOpenAIVideos, BaseURL: "https://example.com", APIKey: "key"},
		{Name: "pool", Format: "other", BaseURL: "https://example.com", APIKey: "key"},
		{Name: "pool", Format: videoPoolFormatOpenAIVideos, BaseURL: "file:///tmp/video", APIKey: "key"},
		{Name: "pool", Format: videoPoolFormatOpenAIVideos, BaseURL: "https://example.com?token=leak", APIKey: "key"},
		{Name: "pool", Format: videoPoolFormatOpenAIVideos, BaseURL: "https://example.com"},
	}
	for _, req := range tests {
		if _, err := normalizeVideoAccountPoolRequest(req, true); err == nil {
			t.Fatalf("expected invalid request to fail: %#v", req)
		}
	}
}

func TestVideoAccountPoolResponseHidesAPIKey(t *testing.T) {
	response := videoAccountPoolResponse(VideoAccountPool{APIKey: "secret"})
	if !response.APIKeySet {
		t.Fatal("expected APIKeySet to be true")
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "apiKey\"") {
		t.Fatalf("response exposed API key: %s", raw)
	}
}
