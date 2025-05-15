package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response represents a standardized API response structure
type Response struct {
	Status  string      `json:"status"`  // "success" or "error"
	Message string      `json:"message"` // Description of the response
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

// ErrorInfo contains detailed information about an error
type ErrorInfo struct {
	Code    string `json:"code,omitempty"`    // Internal error code
	Details string `json:"details,omitempty"` // Additional error details
}

// ResponseBuilder provides a fluent interface for building responses
type ResponseBuilder struct {
	response Response
	code     int
}

// NewResponseBuilder creates a new response builder
func NewResponseBuilder() *ResponseBuilder {
	return &ResponseBuilder{
		response: Response{
			Status: "success",
		},
		code: http.StatusOK,
	}
}

// Success creates a success response with default status code 200
func Success(c *gin.Context, message string, data interface{}) {
	NewResponseBuilder().
		WithMessage(message).
		WithData(data).
		Send(c)
}

// Error creates an error response with provided status code
func Error(c *gin.Context, statusCode int, message string, err error) {
	builder := NewResponseBuilder().
		WithStatus("error").
		WithCode(statusCode).
		WithMessage(message)

	if err != nil {
		builder.WithErrorDetails(err.Error())
	}

	builder.Send(c)
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string, err error) {
	Error(c, http.StatusBadRequest, message, err)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string, err error) {
	if message == "" {
		message = "Authentication required"
	}
	Error(c, http.StatusUnauthorized, message, err)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string, err error) {
	if message == "" {
		message = "Access denied"
	}
	Error(c, http.StatusForbidden, message, err)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string, err error) {
	if message == "" {
		message = "Resource not found"
	}
	Error(c, http.StatusNotFound, message, err)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, err error) {
	Error(c, http.StatusInternalServerError, "Internal server error", err)
}

// ValidationError sends a 422 Unprocessable Entity response
func ValidationError(c *gin.Context, message string, err error) {
	Error(c, http.StatusUnprocessableEntity, message, err)
}

// WithStatus sets the status of the response
func (rb *ResponseBuilder) WithStatus(status string) *ResponseBuilder {
	rb.response.Status = status
	return rb
}

// WithMessage sets the message of the response
func (rb *ResponseBuilder) WithMessage(message string) *ResponseBuilder {
	rb.response.Message = message
	return rb
}

// WithData sets the data of the response
func (rb *ResponseBuilder) WithData(data interface{}) *ResponseBuilder {
	rb.response.Data = data
	return rb
}

// WithCode sets the HTTP status code for the response
func (rb *ResponseBuilder) WithCode(code int) *ResponseBuilder {
	rb.code = code
	return rb
}

// WithErrorCode sets the error code for error responses
func (rb *ResponseBuilder) WithErrorCode(code string) *ResponseBuilder {
	if rb.response.Error == nil {
		rb.response.Error = &ErrorInfo{}
	}
	rb.response.Error.Code = code
	return rb
}

// WithErrorDetails sets additional details for error responses
func (rb *ResponseBuilder) WithErrorDetails(details string) *ResponseBuilder {
	if rb.response.Error == nil {
		rb.response.Error = &ErrorInfo{}
	}
	rb.response.Error.Details = details
	return rb
}

// Send sends the response to the client
func (rb *ResponseBuilder) Send(c *gin.Context) {
	c.JSON(rb.code, rb.response)
}

// NoContent sends a 204 No Content response
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Created sends a 201 Created response with the provided data
func Created(c *gin.Context, message string, data interface{}) {
	NewResponseBuilder().
		WithCode(http.StatusCreated).
		WithMessage(message).
		WithData(data).
		Send(c)
}

// Accepted sends a 202 Accepted response
func Accepted(c *gin.Context, message string) {
	NewResponseBuilder().
		WithCode(http.StatusAccepted).
		WithMessage(message).
		Send(c)
}

// CustomResponse allows sending a custom response with any status code and data
func CustomResponse(c *gin.Context, statusCode int, status string, message string, data interface{}) {
	NewResponseBuilder().
		WithCode(statusCode).
		WithStatus(status).
		WithMessage(message).
		WithData(data).
		Send(c)
}

// StreamResponse sends a streaming response
func StreamResponse(c *gin.Context, contentType string, data []byte) {
	c.Data(http.StatusOK, contentType, data)
}

// FileResponse sends a file as response
func FileResponse(c *gin.Context, filepath string) {
	c.File(filepath)
}