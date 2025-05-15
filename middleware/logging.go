package middleware

import (
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

// loggingMiddleware logs details about the requests
func LoggingMiddleware() gin.HandlerFunc {
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
					strconv.Itoa(statusCode) + " | " + latency.String() + "\n"))
		} else {
			// Log info level
			gin.DefaultWriter.Write([]byte(
				"INFO | " + method + " | " + path + " | " +
					strconv.Itoa(statusCode) + " | " + latency.String() + "\n"))
		}
	}
}
