package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UserController handles user-related HTTP requests
type UserController struct {
	// In a real application, you would have a service or repository dependency here
	// For example: userService service.UserService
	
	// This is a simple in-memory user store for demonstration purposes
	users map[string]User
}

// User represents a user in the system
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"` // "-" means this field will not be included in JSON output
}

// NewUserController creates a new instance of UserController
func NewUserController() *UserController {
	return &UserController{
		users: make(map[string]User),
	}
}

// GetUser handles GET requests to retrieve a user by ID
func (uc *UserController) GetUser(c *gin.Context) {
	id := c.Param("id")
	
	user, exists := uc.users[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   user,
	})
}

// GetAllUsers handles GET requests to retrieve all users
func (uc *UserController) GetAllUsers(c *gin.Context) {
	users := make([]User, 0, len(uc.users))
	for _, user := range uc.users {
		users = append(users, user)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   users,
	})
}

// CreateUser handles POST requests to create a new user
func (uc *UserController) CreateUser(c *gin.Context) {
	var newUser User
	
	if err := c.ShouldBindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}
	
	// Generate ID for new user
	newUser.ID = strconv.Itoa(len(uc.users) + 1)
	
	// Store the new user
	uc.users[newUser.ID] = newUser
	
	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"data":   newUser,
	})
}

// UpdateUser handles PUT requests to update an existing user
func (uc *UserController) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	
	_, exists := uc.users[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	
	var updatedUser User
	if err := c.ShouldBindJSON(&updatedUser); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}
	
	// Preserve the ID from the URL parameter
	updatedUser.ID = id
	
	// Update the user in the store
	uc.users[id] = updatedUser
	
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   updatedUser,
	})
}

// DeleteUser handles DELETE requests to remove a user
func (uc *UserController) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	
	_, exists := uc.users[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	
	// Remove the user from the store
	delete(uc.users, id)
	
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "User deleted successfully",
	})
}

// RegisterRoutes registers the user controller routes on the provided router group
func (uc *UserController) RegisterRoutes(router *gin.RouterGroup) {
	users := router.Group("/users")
	{
		users.GET("", uc.GetAllUsers)
		users.GET("/:id", uc.GetUser)
		users.POST("", uc.CreateUser)
		users.PUT("/:id", uc.UpdateUser)
		users.DELETE("/:id", uc.DeleteUser)
	}
}