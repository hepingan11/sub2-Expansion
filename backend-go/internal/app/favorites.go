package app

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (app *App) listFavoriteSites(c *gin.Context) {
	page := max(queryInt(c, "page", 0), 0)
	size := min(max(queryInt(c, "size", 10), 1), 100)
	var total int64
	var sites []FavoriteSite

	query := app.applyFavoriteSiteFilters(app.db.Model(&FavoriteSite{}), c)
	if err := query.Count(&total).Error; err != nil {
		serverError(c, err)
		return
	}
	if err := query.Order("sort_order ASC, created_at DESC, id DESC").Limit(size).Offset(page * size).Find(&sites).Error; err != nil {
		serverError(c, err)
		return
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(size) - 1) / int64(size))
	}
	c.JSON(http.StatusOK, PageResponse[FavoriteSite]{
		Content:       sites,
		TotalElements: total,
		TotalPages:    totalPages,
		Number:        page,
		Size:          size,
	})
}

func (app *App) listFavoriteSiteGroups(c *gin.Context) {
	var groups []string
	if err := app.db.Model(&FavoriteSite{}).
		Where("group_name <> ''").
		Distinct().
		Order("group_name ASC").
		Pluck("group_name", &groups).Error; err != nil {
		serverError(c, err)
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (app *App) getFavoriteSite(c *gin.Context) {
	site, ok := app.findFavoriteSiteByID(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, site)
}

func (app *App) createFavoriteSite(c *gin.Context) {
	var req FavoriteSiteRequest
	if !bindJSON(c, &req) {
		return
	}
	site, err := normalizeFavoriteSiteRequest(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	if err := app.db.Create(&site).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, site)
}

func (app *App) updateFavoriteSite(c *gin.Context) {
	site, ok := app.findFavoriteSiteByID(c)
	if !ok {
		return
	}
	var req FavoriteSiteRequest
	if !bindJSON(c, &req) {
		return
	}
	normalized, err := normalizeFavoriteSiteRequest(req)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	site.Icon = normalized.Icon
	site.URL = normalized.URL
	site.Name = normalized.Name
	site.Description = normalized.Description
	site.SortOrder = normalized.SortOrder
	site.Group = normalized.Group
	if err := app.db.Save(&site).Error; err != nil {
		handleDBError(c, err)
		return
	}
	c.JSON(http.StatusOK, site)
}

func (app *App) deleteFavoriteSite(c *gin.Context) {
	site, ok := app.findFavoriteSiteByID(c)
	if !ok {
		return
	}
	if err := app.db.Delete(&site).Error; err != nil {
		serverError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (app *App) applyFavoriteSiteFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR url LIKE ? OR description LIKE ? OR group_name LIKE ?", pattern, pattern, pattern, pattern)
	}
	if group := strings.TrimSpace(c.Query("group")); group != "" {
		query = query.Where("group_name = ?", group)
	}
	return query
}

func (app *App) findFavoriteSiteByID(c *gin.Context) (FavoriteSite, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "invalid id")
		return FavoriteSite{}, false
	}
	var site FavoriteSite
	if err := app.db.First(&site, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, APIError{Message: "收藏网站不存在: " + c.Param("id")})
			return FavoriteSite{}, false
		}
		serverError(c, err)
		return FavoriteSite{}, false
	}
	return site, true
}

func normalizeFavoriteSiteRequest(req FavoriteSiteRequest) (FavoriteSite, error) {
	name := strings.TrimSpace(req.Name)
	rawURL := strings.TrimSpace(req.URL)
	icon := strings.TrimSpace(req.Icon)
	description := strings.TrimSpace(req.Description)
	group := strings.TrimSpace(req.Group)

	if name == "" {
		return FavoriteSite{}, errors.New("网站名称不能为空")
	}
	if len([]rune(name)) > 100 {
		return FavoriteSite{}, errors.New("网站名称不能超过 100 个字符")
	}
	if rawURL == "" {
		return FavoriteSite{}, errors.New("网站 URL 不能为空")
	}
	normalizedURL, err := normalizeHTTPURL(rawURL)
	if err != nil {
		return FavoriteSite{}, errors.New("网站 URL 必须是有效的 http/https 地址")
	}
	if icon != "" {
		if !isFavoriteSitePresetIcon(icon) {
			if _, err := normalizeHTTPURL(icon); err != nil {
				return FavoriteSite{}, errors.New("图标地址必须是有效的 http/https 地址或预设图标")
			}
		}
	}
	if len([]rune(icon)) > 500 {
		return FavoriteSite{}, errors.New("图标地址不能超过 500 个字符")
	}
	if len([]rune(description)) > 500 {
		return FavoriteSite{}, errors.New("简介不能超过 500 个字符")
	}
	if len([]rune(group)) > 100 {
		return FavoriteSite{}, errors.New("分组不能超过 100 个字符")
	}
	if req.Sort < 0 {
		return FavoriteSite{}, errors.New("排序必须是大于等于 0 的整数")
	}

	return FavoriteSite{
		Icon:        icon,
		URL:         normalizedURL,
		Name:        name,
		Description: description,
		SortOrder:   req.Sort,
		Group:       group,
	}, nil
}

func isFavoriteSitePresetIcon(value string) bool {
	if !strings.HasPrefix(value, "preset:") {
		return false
	}
	name := strings.TrimPrefix(value, "preset:")
	if name == "" || len(name) > 40 {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
			return false
		}
	}
	return true
}

func normalizeHTTPURL(value string) (string, error) {
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("invalid url scheme")
	}
	return parsed.String(), nil
}
