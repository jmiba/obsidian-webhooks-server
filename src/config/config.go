package config

import (
	cryptoRand "crypto/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration
type Config struct {
	Port                               int
	DatabaseURL                        string
	JWTSecret                          string
	EventTTL                           time.Duration
	EnableAutoCleanup                  bool
	ExternalHost                       string
	WebhookSecret                      string
	EnableWebhookSignatureVerification bool
	AllowedOrigins                     string
	LogLevel                           string
	LogFormat                          string
	AuthRateLimitPerMinute             int
	AuthRateLimitBurst                 int

	// PostHog Analytics settings
	PostHogAPIKey  string
	PostHogHost    string
	PostHogEnabled bool

	// Email Authentication settings
	SMTPHost               string
	SMTPPort               int
	SMTPUsername           string
	SMTPPassword           string
	SMTPFromEmail          string
	SMTPFromName           string
	SMTPTLSMode            string
	MailerLiteAPIKey       string
	MailerLiteGroupSignups string
	MailerLiteGroupActive  string
	MagicLinkExpiry        int // seconds
	MagicLinkBaseURL       string
	CookieSecure           bool

	// Encryption at rest
	EncryptionKey string // 64 hex chars = 32 bytes AES-256 key; empty = disabled

	// Admin auto-seed (first run only)
	AdminUsername string
	AdminPassword string
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Port:                               getEnvInt("PORT", 8080),
		DatabaseURL:                        getEnv("DATABASE_URL", "postgres://user:password@localhost/obsidian_webhooks"),
		JWTSecret:                          getEnv("JWT_SECRET", ""),
		EventTTL:                           time.Duration(getEnvInt("EVENT_TTL_DAYS", 30)) * 24 * time.Hour,
		EnableAutoCleanup:                  getEnvBool("ENABLE_AUTO_CLEANUP", true),
		ExternalHost:                       getEnv("EXTERNAL_HOST", "http://localhost:8080"),
		WebhookSecret:                      getEnv("WEBHOOK_SECRET", ""),
		EnableWebhookSignatureVerification: getEnvBool("ENABLE_WEBHOOK_SIGNATURE_VERIFICATION", false),
		AllowedOrigins:                     getEnv("ALLOWED_ORIGINS", ""),
		LogLevel:                           getEnv("LOG_LEVEL", "info"),
		LogFormat:                          getEnv("LOG_FORMAT", "json"),
		AuthRateLimitPerMinute:             getEnvInt("AUTH_RATE_LIMIT_PER_MINUTE", 3),
		AuthRateLimitBurst:                 getEnvInt("AUTH_RATE_LIMIT_BURST", 3),

		// PostHog Analytics
		PostHogAPIKey:  getEnv("POSTHOG_API_KEY", ""),
		PostHogHost:    getEnv("POSTHOG_HOST", "https://eu.i.posthog.com"),
		PostHogEnabled: getEnvBool("POSTHOG_ENABLED", false),

		// Email Authentication
		SMTPHost:               getEnv("SMTP_HOST", ""),
		SMTPPort:               getEnvInt("SMTP_PORT", 587),
		SMTPUsername:           getEnvWithFallback("SMTP_USERNAME", "SMTP_USER", ""),
		SMTPPassword:           getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:          getEnvWithFallback("SMTP_FROM_EMAIL", "FROM_EMAIL", "noreply@obsidian-webhooks.khabaroff.studio"),
		SMTPFromName:           getEnvWithFallback("SMTP_FROM_NAME", "FROM_NAME", "Khabaroff Studio: Obsidian Webhooks"),
		SMTPTLSMode:            getEnv("SMTP_TLS_MODE", "starttls"),
		MailerLiteAPIKey:       getEnv("MAILERLITE_API_KEY", ""),
		MailerLiteGroupSignups: getEnv("MAILERLITE_GROUP_SIGNUPS", ""),
		MailerLiteGroupActive:  getEnv("MAILERLITE_GROUP_ACTIVE", ""),
		MagicLinkExpiry:        getEnvInt("MAGIC_LINK_EXPIRY", 3600), // 1 hour default
		MagicLinkBaseURL:       getEnv("MAGIC_LINK_BASE_URL", "http://localhost:8080"),

		// Encryption
		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),

		// Admin auto-seed
		AdminUsername: getEnv("ADMIN_USERNAME", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
	}
	cfg.CookieSecure = getEnvBool("COOKIE_SECURE", isHTTPSURL(cfg.MagicLinkBaseURL))

	// Generate JWT secret if not provided
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = generateRandomSecret(32)
	}

	return cfg
}

func isHTTPSURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && strings.EqualFold(parsed.Scheme, "https")
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvWithFallback(key, fallbackKey, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return getEnv(fallbackKey, defaultValue)
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// generateRandomSecret generates a cryptographically secure random secret for JWT signing
func generateRandomSecret(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	if _, err := cryptoRand.Read(result); err != nil {
		panic("failed to generate random secret: " + err.Error())
	}
	for i := range result {
		result[i] = charset[result[i]%byte(len(charset))]
	}
	return string(result)
}
