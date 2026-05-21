package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv            string
	AppPort           string
	DBDSN             string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	JWTSecret         string
	DefaultAdminMail  string
	DefaultAdminPass  string
	UpstreamTimeout   string
	AllowedOrigins    []string
	PublicBaseURL     string
	ClusterMode       bool
	InstanceID        string
	RunBackgroundJobs bool
}

func Load() Config {
	loadDotEnv(".env")
	dbDSN := normalizeMySQLDSN(getEnv("DB_DSN", "root:password@tcp(127.0.0.1:3306)/ai_gateway?charset=utf8mb4&parseTime=True&loc=Local"))

	cfg := Config{
		AppEnv:            getEnv("APP_ENV", "development"),
		AppPort:           getEnv("APP_PORT", "8080"),
		DBDSN:             dbDSN,
		RedisAddr:         getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvInt("REDIS_DB", 0),
		JWTSecret:         getEnv("JWT_SECRET", "change-this-secret"),
		DefaultAdminMail:  getEnv("DEFAULT_ADMIN_EMAIL", "admin@example.com"),
		DefaultAdminPass:  getEnv("DEFAULT_ADMIN_PASSWORD", "admin123456"),
		UpstreamTimeout:   getEnv("UPSTREAM_TIMEOUT", "120s"),
		AllowedOrigins:    splitCSV(getEnv("ALLOWED_ORIGINS", "*")),
		PublicBaseURL:     strings.TrimRight(getEnv("PUBLIC_BASE_URL", ""), "/"),
		ClusterMode:       getEnvBool("CLUSTER_MODE", false),
		InstanceID:        getEnv("INSTANCE_ID", defaultInstanceID()),
		RunBackgroundJobs: getEnvBool("RUN_BACKGROUND_JOBS", true),
	}
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	return cfg
}

func (c Config) Validate() error {
	if c.AppEnv != "production" {
		return nil
	}
	if c.JWTSecret == "" || c.JWTSecret == "change-this-secret" || c.JWTSecret == "replace-with-a-long-random-secret" {
		return fmt.Errorf("JWT_SECRET must be set to a strong non-default value in production")
	}
	if c.DefaultAdminPass == "" || c.DefaultAdminPass == "admin123456" {
		return fmt.Errorf("DEFAULT_ADMIN_PASSWORD must be set to a strong non-default value in production")
	}
	if c.PublicBaseURL == "" {
		return fmt.Errorf("PUBLIC_BASE_URL must be configured in production")
	}
	if len(c.AllowedOrigins) == 0 || contains(c.AllowedOrigins, "*") {
		return fmt.Errorf("ALLOWED_ORIGINS must not be wildcard in production")
	}
	return nil
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

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func defaultInstanceID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	return sanitizeInstanceID(fmt.Sprintf("%s-%d", hostname, os.Getpid()))
}

func sanitizeInstanceID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown-instance"
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
	return re.ReplaceAllString(value, "-")
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
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
