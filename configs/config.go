package configs

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Email    EmailConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Stock    StockAPIConfig
}

// ServerConfig holds configuration related to the HTTP server
type ServerConfig struct {
	Port         int
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Mode         string // "debug", "release", "test"
}

// EmailConfig holds configuration for email services
type EmailConfig struct {
	SMTPServer   string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	EmailFrom    string
	EmailTo      string
}

// DatabaseConfig holds database connection details
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// AuthConfig holds authentication settings
type AuthConfig struct {
	JWTSecret         string
	JWTExpiration     time.Duration
	RefreshSecret     string
	RefreshExpiration time.Duration
}

// StockAPIConfig holds configuration for stock data APIs and cache
type StockAPIConfig struct {
	ProviderType      string // e.g., "tushare", "tonghuashun"
	APIKey            string
	APISecret         string
	Endpoint          string
	TimeoutSeconds    int // API call timeout
	RateLimit         int // Requests per minute for the provider
	RetryCount        int // Number of retries for failed API calls
	RetryDelaySeconds int // Delay between retries in seconds

	// Cache settings
	CacheSize           int    // Max number of items in stock cache
	CacheTTLMinutes     int    // Default TTL for cache entries in minutes
	CacheEvictionPolicy string // "lru", "fifo", "ttl"
	CacheCleanupMinutes int    // Interval for cleaning up expired cache items in minutes
}

// ProviderType defines the available stock data provider types
type ProviderType string

const (
	// TuShareProvider represents the TuShare API provider
	TuShareProvider ProviderType = "tushare"

	// TonghuashunProvider represents the Tonghuashun API provider
	TonghuashunProvider ProviderType = "tonghuashun"

	// DefaultProvider defines the default provider to use
	DefaultProvider = TuShareProvider
)

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	config := &Config{
		Server: ServerConfig{
			Port:         getEnvAsInt("SERVER_PORT", 8080),
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:  time.Duration(getEnvAsInt("SERVER_READ_TIMEOUT", 10)) * time.Second,
			WriteTimeout: time.Duration(getEnvAsInt("SERVER_WRITE_TIMEOUT", 10)) * time.Second,
			Mode:         getEnv("GIN_MODE", "debug"),
		},
		Email: EmailConfig{
			SMTPServer:   getEnv("SMTP_SERVER", "smtp.163.com"),
			SMTPPort:     getEnvAsInt("SMTP_PORT", 25),
			SMTPUsername: getEnv("SMTP_USERNAME", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			EmailFrom:    getEnv("EMAIL_FROM", ""),
			EmailTo:      getEnv("EMAIL_TO", ""),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "app"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Auth: AuthConfig{
			JWTSecret:         getEnv("JWT_SECRET", "default_jwt_secret"),
			JWTExpiration:     time.Duration(getEnvAsInt("JWT_EXPIRATION_HOURS", 24)) * time.Hour,
			RefreshSecret:     getEnv("REFRESH_SECRET", "default_refresh_secret"),
			RefreshExpiration: time.Duration(getEnvAsInt("REFRESH_EXPIRATION_DAYS", 7)) * 24 * time.Hour,
		},
		Stock: StockAPIConfig{
			ProviderType:        getEnv("STOCK_PROVIDER_TYPE", string(DefaultProvider)),
			APIKey:              getEnv("STOCK_API_KEY", ""),
			APISecret:           getEnv("STOCK_API_SECRET", ""),
			Endpoint:            getEnv("STOCK_API_ENDPOINT", ""), // Provider-specific default if empty
			TimeoutSeconds:      getEnvAsInt("STOCK_API_TIMEOUT_SECONDS", 30),
			RateLimit:           getEnvAsInt("STOCK_API_RATE_LIMIT", 100), // General default
			RetryCount:          getEnvAsInt("STOCK_API_RETRY_COUNT", 3),
			RetryDelaySeconds:   getEnvAsInt("STOCK_API_RETRY_DELAY_SECONDS", 1),
			CacheSize:           getEnvAsInt("STOCK_CACHE_SIZE", 10000),
			CacheTTLMinutes:     getEnvAsInt("STOCK_CACHE_TTL_MINUTES", 24*60), // 24 hours
			CacheEvictionPolicy: getEnv("STOCK_CACHE_EVICTION_POLICY", "lru"),
			CacheCleanupMinutes: getEnvAsInt("STOCK_CACHE_CLEANUP_MINUTES", 10),
		},
	}

	// Validate critical configuration
	if config.Email.SMTPUsername == "" || config.Email.SMTPPassword == "" {
		return nil, fmt.Errorf("SMTP credentials (SMTP_USERNAME, SMTP_PASSWORD) are required")
	}

	// Validate Stock API configuration
	if config.Stock.ProviderType != "" {
		// Check if it's a known provider type for more specific validation
		isKnownProvider := false
		switch ProviderType(config.Stock.ProviderType) {
		case TuShareProvider, TonghuashunProvider:
			isKnownProvider = true
		}

		if isKnownProvider {
			if config.Stock.APIKey == "" {
				return nil, fmt.Errorf("STOCK_API_KEY is required when STOCK_PROVIDER_TYPE is '%s'", config.Stock.ProviderType)
			}
			if ProviderType(config.Stock.ProviderType) == TonghuashunProvider && config.Stock.APISecret == "" {
				return nil, fmt.Errorf("STOCK_API_SECRET is required for Tonghuashun provider (STOCK_PROVIDER_TYPE='tonghuashun')")
			}
		}
		// If not a known provider, we assume the user knows what they are doing, or it's effectively disabled.
	}

	return config, nil
}

// GetDSN returns a PostgreSQL connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.DBName,
		c.Database.SSLMode,
	)
}

// GetServerAddress returns the server address in the format "host:port"
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// Helper function to get an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Helper function to get an environment variable as an integer
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// Global application configuration instance
var AppConfig *Config

// InitConfig initializes the global configuration
func InitConfig() error {
	var err error
	AppConfig, err = LoadConfig()
	return err
}

// GetConfig returns the global configuration instance
func GetConfig() *Config {
	if AppConfig == nil {
		panic("Configuration not initialized. Call InitConfig() first.")
	}
	return AppConfig
}
