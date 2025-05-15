package routes

import (
	"github.com/gin-gonic/gin"
	"net/http"

	"backend/controllers"
	"backend/middleware"
)

// SetupRouter initializes the gin router with all application routes
func SetupRouter() *gin.Engine {
	// Create default gin router with Logger and Recovery middleware
	router := gin.Default()

	// Setup global middleware
	router.Use(middleware.CorsMiddleware())

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

		// Stock routes - register the user controller
		stockController := controllers.NewStockController()
		stockController.RegisterRoutes(api)

		//// Admin routes with authentication
		//adminGroup := api.Group("/admin")
		//adminGroup.Use(gin.BasicAuth(gin.Accounts{
		//	"admin": "password", // For demo purposes only
		//}))
		//{
		//	adminGroup.GET("/stats", func(c *gin.Context) {
		//		c.JSON(http.StatusOK, gin.H{
		//			"status": "success",
		//			"data": gin.H{
		//				"users":     100,
		//				"products":  50,
		//				"orders":    25,
		//				"timestamp": time.Now(),
		//			},
		//		})
		//	})
		//}
	}

	return router
}
