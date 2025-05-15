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
	JWTSecret        string
	JWTExpiration    time.Duration
	RefreshSecret    string
	RefreshExpiration time.Duration
}

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
			JWTSecret:        getEnv("JWT_SECRET", "default_jwt_secret"),
			JWTExpiration:    time.Duration(getEnvAsInt("JWT_EXPIRATION_HOURS", 24)) * time.Hour,
			RefreshSecret:    getEnv("REFRESH_SECRET", "default_refresh_secret"),
			RefreshExpiration: time.Duration(getEnvAsInt("REFRESH_EXPIRATION_DAYS", 7)) * 24 * time.Hour,
		},
	}

	// Validate critical configuration
	if config.Email.SMTPUsername == "" || config.Email.SMTPPassword == "" {
		return nil, fmt.Errorf("SMTP credentials are required")
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