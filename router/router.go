package router

import (
	"ai-gateway/config"
	"ai-gateway/controller"
	"ai-gateway/middleware"
	"ai-gateway/service"
	"ai-gateway/upstream"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	r.Use(cors(cfg))

	authController := controller.NewAuthController(cfg, db, redisClient)
	planController := controller.NewPlanController(db)
	orderController := controller.NewOrderController(cfg, db)
	redeemCodeController := controller.NewRedeemCodeController(db)
	apiKeyController := controller.NewAPIKeyController(cfg, db)
	adminController := controller.NewAdminController(db)
	modelController := controller.NewModelController(db)
	settingsController := controller.NewSettingsController(db)
	emailTemplateController := controller.NewEmailTemplateController(db)
	captchaController := controller.NewCaptchaController(redisClient)
	usageController := controller.NewUsageController(db)
	announcementController := controller.NewAnnouncementController(db)
	docsController := controller.NewDocsController(db)
	logHub := service.NewLogHub()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.GET("/settings/public", settingsController.Public)
		api.POST("/captcha/slide", captchaController.CreateSlide)
		api.POST("/auth/email-code", authController.SendEmailCode)
		api.POST("/auth/register", authController.Register)
		api.POST("/auth/login", authController.Login)
		api.GET("/auth/oauth/:provider/start", authController.StartOAuthLogin)
		api.GET("/auth/oauth/:provider/callback", authController.OAuthCallback)
		api.GET("/plans", planController.List)
		api.GET("/models", modelController.List)
		api.GET("/docs", docsController.PublicList)
		api.GET("/docs/:slug", docsController.PublicBySlug)
		api.GET("/announcements", announcementController.PublicList)
		api.GET("/payment/manual", settingsController.ManualPayment)
		api.Any("/payment/epay/notify", orderController.EpayNotify)

		authed := api.Group("", middleware.Auth(cfg, db))
		{
			authed.GET("/auth/me", authController.Me)
			authed.PATCH("/auth/password", authController.ChangePassword)
			authed.GET("/auth/oauth/accounts", authController.OAuthAccounts)
			authed.GET("/auth/oauth/:provider/bind", authController.StartOAuthBind)
			authed.DELETE("/auth/oauth/:provider", authController.UnbindOAuthAccount)
			authed.POST("/orders", orderController.Create)
			authed.POST("/redeem-codes/redeem", redeemCodeController.Redeem)
			authed.GET("/orders", orderController.ListMine)
			authed.POST("/orders/:id/pay", orderController.Pay)
			authed.POST("/orders/:id/manual-payment", orderController.SubmitManualPayment)
			authed.PATCH("/orders/:id/paid", orderController.MarkPaid)
			authed.GET("/keys/secret", apiKeyController.Secret)
			authed.GET("/keys", apiKeyController.List)
			authed.POST("/keys", apiKeyController.Create)
			authed.POST("/keys/rotate", apiKeyController.Rotate)
			authed.PATCH("/keys/:id/disable", apiKeyController.Disable)
			authed.PATCH("/keys/:id/enable", apiKeyController.Enable)
			authed.GET("/usage/logs", usageController.List)
		}

		admin := api.Group("/admin", middleware.Auth(cfg, db), middleware.AdminOnly())
		{
			admin.GET("/users", adminController.Users)
			admin.POST("/users", adminController.CreateUser)
			admin.GET("/users/:id/upstream", adminController.UserUpstream)
			admin.PATCH("/users/:id", adminController.UpdateUser)
			admin.DELETE("/users/:id", adminController.DeleteUser)
			admin.GET("/orders", adminController.Orders)
			admin.PUT("/orders/:id", adminController.UpdateOrder)
			admin.POST("/orders/:id/complete-payment", adminController.CompleteOrderPayment)
			admin.POST("/orders/:id/approve", adminController.ApproveOrder)
			admin.POST("/orders/:id/reject", adminController.RejectOrder)
			admin.POST("/orders/:id/close", adminController.CloseOrder)
			admin.DELETE("/orders/:id", adminController.DeleteOrder)
			admin.GET("/plans", adminController.Plans)
			admin.POST("/plans", adminController.CreatePlan)
			admin.PUT("/plans/:id", adminController.UpdatePlan)
			admin.POST("/plans/:id/draw-lottery", adminController.DrawLotteryPlan)
			admin.DELETE("/plans/:id", adminController.DeletePlan)
			admin.GET("/redeem-codes", adminController.RedeemCodes)
			admin.POST("/redeem-codes", adminController.CreateRedeemCodes)
			admin.PATCH("/redeem-codes/:id/disable", adminController.DisableRedeemCode)
			admin.GET("/settings", settingsController.Get)
			admin.PUT("/settings", settingsController.Update)
			admin.POST("/settings/test-smtp", settingsController.TestSMTP)
			admin.GET("/email-templates", emailTemplateController.List)
			admin.PUT("/email-templates/:type", emailTemplateController.Update)
			admin.GET("/docs", docsController.AdminList)
			admin.POST("/docs", docsController.Create)
			admin.PUT("/docs/:id", docsController.Update)
			admin.DELETE("/docs/:id", docsController.Delete)
			admin.GET("/announcements", announcementController.AdminList)
			admin.POST("/announcements", announcementController.Create)
			admin.PUT("/announcements/:id", announcementController.Update)
			admin.DELETE("/announcements/:id", announcementController.Delete)
			admin.GET("/upstreams", adminController.Upstreams)
			admin.GET("/models", adminController.ModelPricings)
			admin.POST("/models", adminController.CreateModelPricing)
			admin.PUT("/models/:id", adminController.UpdateModelPricing)
			admin.DELETE("/models/:id", adminController.DeleteModelPricing)
			admin.POST("/models/sync-official", adminController.SyncOfficialModelPricings)
			admin.GET("/upstream-channels", adminController.UpstreamChannels)
			admin.POST("/upstream-channels", adminController.CreateUpstreamChannel)
			admin.PUT("/upstream-channels/:id", adminController.UpdateUpstreamChannel)
			admin.DELETE("/upstream-channels/:id", adminController.DeleteUpstreamChannel)
			admin.GET("/public-channels", adminController.PublicChannels)
			admin.POST("/public-channels", adminController.CreatePublicChannel)
			admin.PUT("/public-channels/:id", adminController.UpdatePublicChannel)
			admin.DELETE("/public-channels/:id", adminController.DeletePublicChannel)
			admin.GET("/polling-pools", adminController.PollingPools)
			admin.POST("/polling-pools", adminController.CreatePollingPool)
			admin.PUT("/polling-pools/:id", adminController.UpdatePollingPool)
			admin.DELETE("/polling-pools/:id", adminController.DeletePollingPool)
			admin.GET("/keys", adminController.APIKeys)
			admin.PATCH("/keys/:id", apiKeyController.AdminUpdate)
			admin.DELETE("/keys/:id", apiKeyController.AdminDelete)
			admin.GET("/usage/logs", usageController.AdminList)
			admin.GET("/stats", adminController.Stats)
			admin.GET("/logs/ws", logHub.Serve)
		}
	}

	r.Any("/v1/*path", middleware.APIKeyAuth(db, redisClient), upstream.ProxyHandler(db, logHub))
	r.Any("/messages", middleware.APIKeyAuth(db, redisClient), upstream.ProxyHandler(db, logHub))
	return r
}

func cors(cfg config.Config) gin.HandlerFunc {
	allowedOrigins := map[string]bool{}
	allowWildcard := cfg.AppEnv != "production"
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			allowWildcard = true
			continue
		}
		allowedOrigins[origin] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		switch {
		case allowWildcard:
			c.Header("Access-Control-Allow-Origin", "*")
		case origin != "" && allowedOrigins[origin]:
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key,Anthropic-Version,Anthropic-Beta")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
