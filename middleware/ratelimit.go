package middleware

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"net/http"
)

// rateLimitMiddleware implements a request rate limiter
func RateLimitMiddleware(requestsPerPeriod int, periodInSeconds int) gin.HandlerFunc {
	// Create a rate limiter that allows requestsPerPeriod requests per periodInSeconds
	limiter := rate.NewLimiter(rate.Limit(float64(requestsPerPeriod)/float64(periodInSeconds)), requestsPerPeriod)

	return func(c *gin.Context) {
		// If rate limit exceeded, return 429 Too Many Requests
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"status":  "error",
				"message": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
