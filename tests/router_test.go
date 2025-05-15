package tests

import (
	"backend/controllers"
	"backend/routes"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestPingRoute tests the ping endpoint
func TestPingRoute(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Setup the router as you would in your main function
	router := routes.SetupRouter()

	// Create a response recorder
	w := httptest.NewRecorder()

	// Create a request to send to the above route
	req, _ := http.NewRequest("GET", "/api/ping", nil)

	// Process the request through the router
	router.ServeHTTP(w, req)

	// Assert that the HTTP status code is 200 OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	// Assert that the unmarshalling didn't return an error
	assert.Nil(t, err)

	// Assert that the response has the expected format
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "pong", response["message"])
}

// TestNotFoundRoute tests the 404 response for non-existent routes
func TestNotFoundRoute(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Setup the router as you would in your main function
	router := routes.SetupRouter()

	// Create a response recorder
	w := httptest.NewRecorder()

	// Create a request for a non-existent endpoint
	req, _ := http.NewRequest("GET", "/non-existent-route", nil)

	// Process the request through the router
	router.ServeHTTP(w, req)

	// Assert that the HTTP status code is 404 Not Found
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Parse the response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)

	// Assert that the unmarshalling didn't return an error
	assert.Nil(t, err)

	// Assert that the response has the expected format for errors
	assert.Equal(t, "error", response["status"])
	assert.Equal(t, "Route not found", response["message"])
}

// TestUserRoutes tests the user-related routes
func TestUserRoutes(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Setup the router
	router := setupTestRouter()

	// Test cases
	t.Run("Get All Users - Initially Empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/users", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		
		// Initial user list should be empty
		data, ok := response["data"].([]interface{})
		assert.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("Create User", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		// Create user request payload
		userData := map[string]interface{}{
			"name":     "Test User",
			"email":    "test@example.com",
			"password": "password123",
		}
		jsonData, _ := json.Marshal(userData)
		
		req, _ := http.NewRequest("POST", "/api/users", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		
		// Verify the created user data
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Test User", data["name"])
		assert.Equal(t, "test@example.com", data["email"])
		assert.Equal(t, "1", data["id"])
		
		// Password should not be included in response
		_, passwordExists := data["password"]
		assert.False(t, passwordExists)
	})

	t.Run("Get User by ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/users/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		
		// Verify user data
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Test User", data["name"])
		assert.Equal(t, "test@example.com", data["email"])
		assert.Equal(t, "1", data["id"])
	})

	t.Run("Update User", func(t *testing.T) {
		w := httptest.NewRecorder()
		
		// Update user request payload
		userData := map[string]interface{}{
			"name":     "Updated User",
			"email":    "updated@example.com",
			"password": "newpassword123",
		}
		jsonData, _ := json.Marshal(userData)
		
		req, _ := http.NewRequest("PUT", "/api/users/1", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		
		// Verify the updated user data
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, "Updated User", data["name"])
		assert.Equal(t, "updated@example.com", data["email"])
		assert.Equal(t, "1", data["id"])
	})

	t.Run("Delete User", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/users/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		assert.Equal(t, "User deleted successfully", response["message"])
	})

	t.Run("Get Deleted User", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/users/1", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "error", response["status"])
		assert.Equal(t, "User not found", response["message"])
	})
}

// TestBasicAuthProtectedRoutes tests routes protected by Basic Auth
func TestBasicAuthProtectedRoutes(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	// Setup the router
	router := routes.SetupRouter()

	t.Run("Access Admin Stats Without Auth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/admin/stats", nil)
		router.ServeHTTP(w, req)

		// Should be unauthorized
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("Access Admin Stats With Auth", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/admin/stats", nil)
		
		// Add Basic Auth header
		req.SetBasicAuth("admin", "password")
		
		router.ServeHTTP(w, req)

		// Should be authorized and successful
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.Nil(t, err)
		
		assert.Equal(t, "success", response["status"])
		
		// Verify stats data
		data, ok := response["data"].(map[string]interface{})
		assert.True(t, ok)
		assert.Equal(t, float64(100), data["users"])
		assert.Equal(t, float64(50), data["products"])
		assert.Equal(t, float64(25), data["orders"])
	})
}

// setupTestRouter creates a test router with a fresh user controller
// This ensures tests start with a clean state
func setupTestRouter() *gin.Engine {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)
	
	// Create a new router
	router := gin.Default()
	
	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	})
	
	// Set up the 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Route not found",
		})
	})
	
	// Create a fresh user controller for testing
	userController := controllers.NewUserController()
	
	// Set up API routes
	api := router.Group("/api")
	{
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "pong",
			})
		})
		
		// Register user routes
		userGroup := api.Group("/users")
		{
			userGroup.GET("", userController.GetAllUsers)
			userGroup.GET("/:id", userController.GetUser)
			userGroup.POST("", userController.CreateUser)
			userGroup.PUT("/:id", userController.UpdateUser)
			userGroup.DELETE("/:id", userController.DeleteUser)
		}
	}
	
	return router
}