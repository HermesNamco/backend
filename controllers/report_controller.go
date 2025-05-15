package controllers

import (
	"net/http"
	"backend/service/reporter"
	"github.com/gin-gonic/gin"
	"log"
)

// ReportRequest represents the data structure for a report request
type ReportRequest struct {
	Text string `json:"text" binding:"required"`
}

// ReportController handles report-related HTTP requests
type ReportController struct {
	reporter reporter.Reporter
}

// NewReportController creates a new instance of ReportController
func NewReportController() *ReportController {
	return &ReportController{
		reporter: reporter.NewReporter(),
	}
}

// CreateReport handles POST requests to create and send a new report
func (rc *ReportController) CreateReport(c *gin.Context) {
	var req ReportRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}
	
	if err := rc.reporter.Send(req.Text); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to send report",
			"error":   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Report sent successfully",
	})
}

// GetReportStatus handles GET requests to check the status of a report
func (rc *ReportController) GetReportStatus(c *gin.Context) {
	id := c.Param("id")
	
	// In a real application, you would fetch the report status from a database
	// For now we just return a mock status
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"reportId": id,
			"status":   "processed",
		},
	})
}

// RegisterRoutes registers the report controller routes on the provided router group
func (rc *ReportController) RegisterRoutes(router *gin.RouterGroup) {
	reports := router.Group("/reports")
	{
		reports.POST("/send", rc.CreateReport)
		reports.GET("/:id/status", rc.GetReportStatus)
	}
}

// ReportHandler is the legacy handler function, maintained for backward compatibility
// It's recommended to use ReportController methods instead
func ReportHandler(c *gin.Context) {
	var req ReportRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
		})
		return
	}
	
	if err := reporter.NewReporter().Send(req.Text); err != nil {
		log.Printf("Error sending report: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to send report",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Report sent successfully",
	})
}