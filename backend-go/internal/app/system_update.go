package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SystemUpdateCheckResponse struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseName     string `json:"releaseName"`
	PublishedAt     string `json:"publishedAt"`
	Repository      string `json:"repository"`
	UpdateEnabled   bool   `json:"updateEnabled"`
	UpdateCommand   string `json:"updateCommand,omitempty"`
	Message         string `json:"message"`
}

type SystemUpdateRunResponse struct {
	Started bool   `json:"started"`
	Output  string `json:"output"`
	Message string `json:"message"`
}

type githubLatestRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

func (app *App) getSystemUpdateCheck(c *gin.Context) {
	result, err := app.checkLatestRelease(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (app *App) runSystemUpdate(c *gin.Context) {
	command := strings.TrimSpace(app.cfg.SystemUpdateCommand)
	if command == "" {
		conflict(c, "SYSTEM_UPDATE_COMMAND is not configured")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Minute)
	defer cancel()

	updateInfo, err := app.checkLatestRelease(ctx)
	if err != nil {
		serverError(c, err)
		return
	}

	cmd := shellCommand(ctx, command)
	cmd.Env = append(cmd.Environ(),
		"CURRENT_VERSION="+updateInfo.CurrentVersion,
		"LATEST_VERSION="+updateInfo.LatestVersion,
		"RELEASE_URL="+updateInfo.ReleaseURL,
		"GITHUB_REPOSITORY="+updateInfo.Repository,
	)
	output, err := cmd.CombinedOutput()
	limitedOutput := limitString(string(output), 12000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, SystemUpdateRunResponse{
			Started: false,
			Output:  limitedOutput,
			Message: fmt.Sprintf("update command failed: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, SystemUpdateRunResponse{
		Started: true,
		Output:  limitedOutput,
		Message: "update command completed",
	})
}

func (app *App) checkLatestRelease(ctx context.Context) (SystemUpdateCheckResponse, error) {
	repository := strings.TrimSpace(app.cfg.GitHubRepository)
	if repository == "" {
		repository = "hepingan11/sub2-Expansion"
	}
	current := strings.TrimSpace(app.cfg.AppVersion)
	if current == "" {
		current = "dev"
	}

	release, err := fetchGitHubLatestRelease(ctx, repository)
	if err != nil {
		return SystemUpdateCheckResponse{}, err
	}
	latest := strings.TrimSpace(release.TagName)
	return SystemUpdateCheckResponse{
		CurrentVersion:  current,
		LatestVersion:   latest,
		UpdateAvailable: releaseIsNewer(current, latest),
		ReleaseURL:      release.HTMLURL,
		ReleaseName:     release.Name,
		PublishedAt:     release.PublishedAt,
		Repository:      repository,
		UpdateEnabled:   strings.TrimSpace(app.cfg.SystemUpdateCommand) != "",
		UpdateCommand:   strings.TrimSpace(app.cfg.SystemUpdateCommand),
		Message:         updateCheckMessage(current, latest),
	}, nil
}

func fetchGitHubLatestRelease(ctx context.Context, repository string) (githubLatestRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	endpoint := "https://api.github.com/repos/" + strings.Trim(repository, "/") + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return githubLatestRelease{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sub2-expansion-update-checker")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubLatestRelease{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubLatestRelease{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return githubLatestRelease{}, fmt.Errorf("GitHub release check failed: %s", resp.Status)
	}

	var release githubLatestRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return githubLatestRelease{}, err
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubLatestRelease{}, fmt.Errorf("GitHub release check failed: latest release has no tag")
	}
	return release, nil
}

func updateCheckMessage(current, latest string) string {
	if latest == "" {
		return "No release version detected"
	}
	if releaseIsNewer(current, latest) {
		return "New version available"
	}
	return "Already up to date"
}

func releaseIsNewer(current, latest string) bool {
	current = strings.TrimSpace(current)
	latest = strings.TrimSpace(latest)
	if latest == "" {
		return false
	}
	if normalizeVersionTag(current) == normalizeVersionTag(latest) {
		return false
	}
	currentParts, currentOK := semverParts(current)
	latestParts, latestOK := semverParts(latest)
	if currentOK && latestOK {
		for index := 0; index < len(latestParts); index++ {
			if latestParts[index] != currentParts[index] {
				return latestParts[index] > currentParts[index]
			}
		}
		return false
	}
	return true
}

func normalizeVersionTag(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimPrefix(value, "refs/tags/")
	return strings.TrimPrefix(value, "v")
}

var versionPattern = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)(?:\.([0-9]+))?`)

func semverParts(value string) ([3]int, bool) {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 4 {
		return [3]int{}, false
	}
	var parts [3]int
	for index := 0; index < 3; index++ {
		if matches[index+1] == "" {
			continue
		}
		parsed, err := strconv.Atoi(matches[index+1])
		if err != nil {
			return [3]int{}, false
		}
		parts[index] = parsed
	}
	return parts, true
}

func shellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-c", command)
}

func limitString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
