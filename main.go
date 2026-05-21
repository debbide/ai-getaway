package main

import (
	"log"

	"ai-gateway/config"
	"ai-gateway/database"
	"ai-gateway/router"
	"ai-gateway/service"
)

func main() {
	cfg := config.Load()
	log.Printf("using database dsn: %s", config.MaskDSN(cfg.DBDSN))

	db, err := database.InitMariaDB(cfg)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	redisClient := database.InitRedis(cfg)
	database.AutoMigrate(db)
	database.Seed(db, cfg)
	database.StartSlideCaptchaCleanup(cfg, db, redisClient)
	database.StartOrderTimeoutCleanup(cfg, db, redisClient)
	database.StartSubscriptionExpireEmailReminder(cfg, db, redisClient)
	service.StartChannelMonitorRunner(cfg, db, redisClient)

	r := router.New(cfg, db, redisClient)
	if err := r.Run(":" + cfg.AppPort); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
