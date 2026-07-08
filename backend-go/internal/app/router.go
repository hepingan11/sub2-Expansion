package app

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func (app *App) router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     app.cfg.CorsAllowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"*"},
		AllowCredentials: false,
		MaxAge:           time.Hour,
	}))

	router.GET("/api/public/sub2api/group-rate-series", app.getPublicSub2APIGroupRateSeries)
	router.POST("/api/checkins", app.checkIn)

	user := router.Group("/api/user")
	user.POST("/login", app.userLogin)
	user.POST("/login/2fa", app.userLogin2FA)
	user.POST("/refresh", app.refreshUserToken)
	protectedUser := router.Group("/api/user", app.userAuth)
	protectedUser.GET("/me", app.getCurrentSub2APIUser)
	protectedUser.GET("/check-in", app.getUserCheckInStatus)
	protectedUser.POST("/check-in", app.userCheckIn)
	protectedUser.POST("/social-bindings", app.bindSocialAccount)
	protectedUser.GET("/recharge-rewards", app.listUserRechargeRewards)
	protectedUser.POST("/recharge-rewards/:activityId/tiers/:tierId/claim", app.claimRechargeReward)

	admin := router.Group("/api/admin")
	admin.POST("/login", app.login)
	protected := router.Group("/api/admin", app.adminAuth)
	protected.GET("/codes", app.listCodes)
	protected.GET("/codes/:id", app.getCode)
	protected.POST("/codes", app.createCode)
	protected.POST("/codes/batch-import", app.batchImportCodes)
	protected.PUT("/codes/:id", app.updateCode)
	protected.DELETE("/codes/:id", app.deleteCode)
	protected.GET("/favorite-sites", app.listFavoriteSites)
	protected.GET("/favorite-sites/groups", app.listFavoriteSiteGroups)
	protected.GET("/favorite-sites/:id", app.getFavoriteSite)
	protected.POST("/favorite-sites", app.createFavoriteSite)
	protected.PUT("/favorite-sites/:id", app.updateFavoriteSite)
	protected.DELETE("/favorite-sites/:id", app.deleteFavoriteSite)
	protected.GET("/stats", app.stats)
	protected.GET("/check-in-stats", app.getCheckInStats)
	protected.GET("/settings/check-in", app.getCheckInSettings)
	protected.PUT("/settings/check-in", app.updateCheckInSettings)
	protected.GET("/sub2api/group-rate-monitor", app.getSub2APIGroupRateMonitor)
	protected.PUT("/sub2api/group-rate-monitor", app.updateSub2APIGroupRateMonitor)
	protected.POST("/sub2api/group-rate-monitor/refresh", app.refreshSub2APIGroupRates)
	protected.GET("/recharge-activities", app.listRechargeActivities)
	protected.POST("/recharge-activities", app.createRechargeActivity)
	protected.PUT("/recharge-activities/:id", app.updateRechargeActivity)
	protected.DELETE("/recharge-activities/:id", app.deleteRechargeActivity)

	return router
}
