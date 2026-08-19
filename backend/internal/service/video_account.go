package service

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// VideoAccountFormatOpenAIVideos is the first video account wire format.
const VideoAccountFormatOpenAIVideos = "openai_videos"
const VideoAccountFormatComfyUI = "comfyui"

var videoAccountModelPattern = regexp.MustCompile(`^[A-Za-z0-9_./:-]{1,100}$`)
var videoAccountTaskIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)
var videoAccountWorkflowIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,200}$`)

func isVideoAccount(account *Account) bool {
	return account != nil && account.Platform == PlatformVideo
}

func videoAccountBaseURL(account *Account) string {
	if account == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(account.GetCredential("base_url")), "/")
}

func videoAccountBaseURLIsComplete(account *Account) bool {
	if account == nil || account.Credentials == nil {
		return false
	}
	value, ok := account.Credentials["base_url_is_complete"].(bool)
	return ok && value
}

func videoAccountVideosURL(account *Account) string {
	base := videoAccountBaseURL(account)
	if videoAccountBaseURLIsComplete(account) {
		return base
	}
	return base + "/v1/videos"
}

func videoAccountFormat(account *Account) string {
	if account == nil {
		return VideoAccountFormatOpenAIVideos
	}
	format, _ := account.Credentials["format"].(string)
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		return VideoAccountFormatOpenAIVideos
	}
	return format
}

func videoAccountComfyWorkflowID(account *Account, requestedModel string) string {
	if account == nil {
		return ""
	}
	if mapped := strings.TrimSpace(account.GetMappedModel(requestedModel)); mapped != "" {
		return mapped
	}
	id, _ := account.Credentials["workflow_id"].(string)
	return strings.TrimSpace(id)
}

func videoAccountComfyCreateURL(account *Account, workflowID string) string {
	return videoAccountBaseURL(account) + "/api/v1/comfyui/comfyui_workflow/" + url.PathEscape(workflowID)
}

func videoAccountComfyResultURL(account *Account, taskID string) string {
	return videoAccountBaseURL(account) + "/api/v1/comfyui/comfyui_workflow/result/" + url.PathEscape(taskID)
}

func videoAccountModelsURL(account *Account) string {
	base := videoAccountBaseURL(account)
	parsed, err := url.Parse(base)
	if err == nil {
		if strings.HasSuffix(parsed.Path, "/videos") {
			parsed.Path = strings.TrimSuffix(parsed.Path, "/videos") + "/models"
			return parsed.String()
		}
		if strings.HasSuffix(parsed.Path, "/v1") {
			return strings.TrimRight(parsed.String(), "/") + "/models"
		}
	}
	if videoAccountBaseURLIsComplete(account) {
		return base + "/models"
	}
	return base + "/v1/models"
}

func validateVideoAccountBaseURL(raw string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("video Base URL must be a valid HTTP/HTTPS URL without credentials, query parameters, or fragments")
	}
	return nil
}

// NormalizeVideoAccountCredentials validates and normalizes Video account credentials.
// It intentionally leaves non-video accounts untouched so existing providers keep their
// historical permissive credential behavior.
func NormalizeVideoAccountCredentials(platform, accountType string, credentials map[string]any, creating bool) error {
	if platform != PlatformVideo {
		return nil
	}
	if accountType != AccountTypeAPIKey {
		return fmt.Errorf("video accounts must use type %s", AccountTypeAPIKey)
	}
	if credentials == nil {
		return errors.New("video account credentials are required")
	}
	baseURL, _ := credentials["base_url"].(string)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return errors.New("video Base URL is required")
	}
	if err := validateVideoAccountBaseURL(baseURL); err != nil {
		return err
	}
	credentials["base_url"] = baseURL

	format, _ := credentials["format"].(string)
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		format = VideoAccountFormatOpenAIVideos
	}
	if format != VideoAccountFormatOpenAIVideos && format != VideoAccountFormatComfyUI {
		return fmt.Errorf("unsupported video account format: %s", format)
	}
	credentials["format"] = format
	if complete, ok := credentials["base_url_is_complete"]; ok {
		if _, ok := complete.(bool); !ok {
			return errors.New("video base_url_is_complete must be a boolean")
		}
	} else {
		credentials["base_url_is_complete"] = false
	}
	if format == VideoAccountFormatComfyUI {
		credentials["base_url_is_complete"] = false
		workflow, _ := credentials["workflow_id"].(string)
		workflow = strings.TrimSpace(workflow)
		if workflow != "" && !videoAccountWorkflowIDPattern.MatchString(workflow) {
			return errors.New("invalid ComfyUI workflow ID")
		}
		if workflow == "" {
			mapping, _ := credentials["model_mapping"].(map[string]any)
			if len(mapping) == 0 {
				return errors.New("ComfyUI workflow ID is required")
			}
		}
		if workflow != "" {
			credentials["workflow_id"] = workflow
		}
	}

	apiKey, _ := credentials["api_key"].(string)
	apiKey = strings.TrimSpace(apiKey)
	if creating && apiKey == "" {
		return errors.New("video API Key is required")
	}
	if apiKey != "" {
		credentials["api_key"] = apiKey
	}
	if !creating && apiKey == "" {
		// Update paths merge sensitive credentials before validation. A missing key
		// here means the account already has a redacted key, so leave it intact.
		if _, exists := credentials["api_key"]; exists {
			return errors.New("video API Key cannot be cleared")
		}
	}

	if mapping, ok := credentials["model_mapping"].(map[string]any); ok {
		for requested, rawMapped := range mapping {
			if !videoAccountModelPattern.MatchString(strings.TrimSpace(requested)) {
				return fmt.Errorf("invalid video model ID: %s", requested)
			}
			mapped, ok := rawMapped.(string)
			if !ok || !videoAccountModelPattern.MatchString(strings.TrimSpace(mapped)) {
				return fmt.Errorf("invalid mapped video model for %s", requested)
			}
		}
	}
	return nil
}
