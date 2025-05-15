package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"backend/controllers"
	"backend/middleware"
)

// SetupRouter initializes the gin router with all application routes
func SetupRouter() *gin.Engine {
	// Create default gin router with Logger and Recovery middleware
	router := gin.Default()

	// Setup global middleware
	router.Use(corsMiddleware())
	
	// Handle 404 not found
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Route not found",
		})
	})

	// API routes
	api := router.Group("/api")
	{
		// Public routes
		api.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "pong",
			})
		})

		// User routes - register the user controller
		userController := controllers.NewUserController()
		userController.RegisterRoutes(api)

		// Report routes - register the report controller
		reportController := controllers.NewReportController()
		reportController.RegisterRoutes(api)

		// Legacy report route
		api.POST("/report", controllers.ReportHandler)

		// Admin routes with authentication
		adminGroup := api.Group("/admin")
		adminGroup.Use(gin.BasicAuth(gin.Accounts{
			"admin": "password", // For demo purposes only
		}))
		{
			adminGroup.GET("/stats", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"status": "success",
					"data": gin.H{
						"users":     100,
						"products":  50,
						"orders":    25,
						"timestamp": time.Now(),
					},
				})
			})
		}

		// Report routes with authentication and logging middleware
		reportGroup := api.Group("/reports")
		reportGroup.Use(gin.BasicAuth(gin.Accounts{
			"reporter": "secret", // For demo purposes only
		}))
		reportGroup.Use(loggingMiddleware())
		{
			reportGroup.POST("/create", func(c *gin.Context) {
				// Sample handler to demonstrate route grouping
				var report struct {
					Title   string `json:"title" binding:"required"`
					Content string `json:"content" binding:"required"`
				}
				
				if err := c.ShouldBindJSON(&report); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"status":  "error",
						"message": "Invalid request data",
						"error":   err.Error(),
					})
					return
				}
				
				c.JSON(http.StatusCreated, gin.H{
					"status": "success",
					"data": gin.H{
						"message": "Report created successfully",
						"report":  report,
					},
				})
			})
		}
	}

	return router
}

// corsMiddleware handles Cross-Origin Resource Sharing (CORS)
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// loggingMiddleware logs details about the requests
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		startTime := time.Now()
		
		// Process request
		c.Next()
		
		// Calculate request time
		endTime := time.Now()
		latency := endTime.Sub(startTime)
		
		// Log request details
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		
		// You can integrate with a proper logging system here
		if statusCode >= 500 {
			// Log error level
			gin.DefaultErrorWriter.Write([]byte(
				"ERROR | " + method + " | " + path + " | " + 
				statusCode + " | " + latency.String() + "\n"))
		} else {
			// Log info level
			gin.DefaultWriter.Write([]byte(
				"INFO | " + method + " | " + path + " | " + 
				statusCode + " | " + latency.String() + "\n"))
		}
	}
}