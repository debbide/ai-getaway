package config

import (
	"bufio"
	"net/url"
	"os"
	"strconv"
	"strings"
)

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
	loadDotEnv(".env")
	dbDSN := normalizeMySQLDSN(getEnv("DB_DSN", "root:password@tcp(127.0.0.1:3306)/ai_gateway?charset=utf8mb4&parseTime=True&loc=Local"))

	return Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		AppPort:          getEnv("APP_PORT", "8080"),
		DBDSN:            dbDSN,
		RedisAddr:        getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvInt("REDIS_DB", 0),
		JWTSecret:        getEnv("JWT_SECRET", "change-this-secret"),
		DefaultAdminMail: getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"),
		DefaultAdminPass: getEnv("DEFAULT_ADMIN_PASSWORD", "admin123456"),
		UpstreamTimeout:  getEnv("UPSTREAM_TIMEOUT", "120s"),
	}
}

func normalizeMySQLDSN(dsn string) string {
	base, rawQuery, ok := strings.Cut(dsn, "?")
	if !ok || rawQuery == "" {
		return dsn
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return dsn
	}
	return base + "?" + values.Encode()
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		os.Setenv(key, value)
	}
}

func MaskDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	colon := strings.Index(dsn, ":")
	if at <= 0 || colon <= 0 || colon > at {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}
