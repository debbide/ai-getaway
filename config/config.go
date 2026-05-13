package config

import "os"

type Config struct {
	AppEnv           string
	AppPort          string
	DBDSN            string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	JWTSecret        string
	DefaultAdminMail string
	DefaultAdminPass string
	UpstreamTimeout  string
}

func Load() Config {
	return Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		AppPort:          getEnv("APP_PORT", "8080"),
		DBDSN:            getEnv("DB_DSN", "root:password@tcp(127.0.0.1:3306)/ai_gateway?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisAddr:        getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          0,
		JWTSecret:        getEnv("JWT_SECRET", "change-this-secret"),
		DefaultAdminMail: getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"),
		DefaultAdminPass: getEnv("DEFAULT_ADMIN_PASSWORD", "admin123456"),
		UpstreamTimeout:  getEnv("UPSTREAM_TIMEOUT", "120s"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
