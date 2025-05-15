package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"backend/configs"
	"backend/models"
	"backend/utils"
)

// JWTAuthMiddleware provides JWT authentication for routes
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from the Authorization header
		// Format: "Bearer {token}"
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "Authorization header required", nil)
			c.Abort()
			return
		}

		// Check if the header has the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.Unauthorized(c, "Invalid authorization format, expected 'Bearer {token}'", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := validateToken(tokenString)
		if err != nil {
			utils.Unauthorized(c, "Invalid or expired token", err)
			c.Abort()
			return
		}

		// Store user info in the context so handlers can access it
		c.Set("userID", claims.UserID)
		c.Set("userName", claims.Username)
		c.Set("userRole", claims.Role)
		c.Set("userClaims", claims)
		
		c.Next()
	}
}

// JWTClaims represents the custom JWT claims
type JWTClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// validateToken verifies the JWT token and returns the claims if valid
func validateToken(tokenString string) (*JWTClaims, error) {
	// Get the JWT secret from configs
	config := configs.GetConfig()
	if config == nil {
		return nil, errors.New("application config not initialized")
	}

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing algorithm
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		
		// Return the secret key for validation
		return []byte(config.Auth.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RequireRole middleware ensures user has the required role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("userRole")
		if !exists {
			utils.Unauthorized(c, "Authentication required", nil)
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			utils.InternalServerError(c, errors.New("invalid role type in context"))
			c.Abort()
			return
		}

		// Check if user's role is in the required roles list
		hasPermission := false
		for _, role := range roles {
			if role == roleStr {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			utils.Forbidden(c, "Insufficient permissions", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

// GenerateToken creates a new JWT token for a user
func GenerateToken(user *models.User) (string, error) {
	config := configs.GetConfig()
	if config == nil {
		return "", errors.New("application config not initialized")
	}

	// Set expiration time
	expirationTime := time.Now().Add(config.Auth.JWTExpiration)
	
	// Create claims with user data
	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "gin-app",
			Subject:   user.ID,
		},
	}
	
	// Create the token with the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(config.Auth.JWTSecret))
	if err != nil {
		return "", err
	}
	
	return tokenString, nil
}

// GetCurrentUser extracts the current user from the context
func GetCurrentUser(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	
	if id, ok := userID.(string); ok {
		return id, true
	}
	
	return "", false
}