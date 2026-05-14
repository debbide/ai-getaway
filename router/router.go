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
	r.Use(cors())

	authController := controller.NewAuthController(cfg, db)
	planController := controller.NewPlanController(db)
	orderController := controller.NewOrderController(db)
	apiKeyController := controller.NewAPIKeyController(cfg, db)
	adminController := controller.NewAdminController(db)
	settingsController := controller.NewSettingsController(db)
	captchaController := controller.NewCaptchaController(db)
	usageController := controller.NewUsageController(db)
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
		api.GET("/plans", planController.List)
		api.GET("/docs", docsController.PublicList)
		api.GET("/docs/:slug", docsController.PublicBySlug)
		api.Any("/payment/epay/notify", orderController.EpayNotify)

		authed := api.Group("", middleware.Auth(cfg, db))
		{
			authed.GET("/auth/me", authController.Me)
			authed.PATCH("/auth/password", authController.ChangePassword)
			authed.POST("/orders", orderController.Create)
			authed.GET("/orders", orderController.ListMine)
			authed.POST("/orders/:id/pay", orderController.Pay)
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
			admin.PATCH("/users/:id", adminController.UpdateUser)
			admin.DELETE("/users/:id", adminController.DeleteUser)
			admin.GET("/orders", adminController.Orders)
			admin.PUT("/orders/:id", adminController.UpdateOrder)
			admin.POST("/orders/:id/approve", adminController.ApproveOrder)
			admin.POST("/orders/:id/reject", adminController.RejectOrder)
			admin.GET("/plans", adminController.Plans)
			admin.POST("/plans", adminController.CreatePlan)
			admin.PUT("/plans/:id", adminController.UpdatePlan)
			admin.DELETE("/plans/:id", adminController.DeletePlan)
			admin.GET("/settings", settingsController.Get)
			admin.PUT("/settings", settingsController.Update)
			admin.GET("/docs", docsController.AdminList)
			admin.POST("/docs", docsController.Create)
			admin.PUT("/docs/:id", docsController.Update)
			admin.DELETE("/docs/:id", docsController.Delete)
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
			admin.GET("/keys", adminController.APIKeys)
			admin.GET("/stats", adminController.Stats)
			admin.GET("/logs/ws", logHub.Serve)
		}
	}

	r.Any("/v1/*path", middleware.APIKeyAuth(db, redisClient), upstream.ProxyHandler(db, logHub))
	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
