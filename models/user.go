package models

import (
	"database/sql"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
)

// User represents a user in the system
type User struct {
	ID            string     `json:"id" validate:"required"`
	Username      string     `json:"username" validate:"required,min=3,max=30"`
	Email         string     `json:"email" validate:"required,email"`
	Password      string     `json:"-" validate:"required,min=8"` // Not included in JSON output
	FirstName     string     `json:"firstName" validate:"max=50"`
	LastName      string     `json:"lastName" validate:"max=50"`
	Role          string     `json:"role" validate:"oneof=admin user guest"`
	Active        bool       `json:"active"`
	EmailVerified bool       `json:"emailVerified"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

// Repository interface represents the expected behavior for user storage
type Repository interface {
	FindByID(id string) (*User, error)
	FindByEmail(email string) (*User, error)
	FindByUsername(username string) (*User, error)
	FindAll() ([]User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(id string) error
}

// Service interface represents the expected behavior for user business logic
type Service interface {
	GetByID(id string) (*User, error)
	GetByEmail(email string) (*User, error)
	GetByUsername(username string) (*User, error)
	GetAll() ([]User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(id string) error
	Validate(user *User) error
	Authenticate(email, password string) (*User, error)
}

// NewUser creates a new user with default values
func NewUser() *User {
	now := time.Now()
	return &User{
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the user data is valid
func (u *User) Validate() error {
	validate := validator.New()
	return validate.Struct(u)
}

// HashPassword replaces the password with a secure hash
// In a real application, you would use a package like bcrypt
func (u *User) HashPassword() error {
	if u.Password == "" {
		return errors.New("password cannot be empty")
	}
	
	// In production code, replace this with proper password hashing:
	// hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	// if err != nil {
	//     return err
	// }
	// u.Password = string(hashedPassword)
	
	// This is just a placeholder and NOT secure!
	// Just to demonstrate the concept
	u.Password = "hashed_" + u.Password
	
	return nil
}

// CheckPassword verifies if the provided password matches the stored hash
// In a real application, you would use a package like bcrypt
func (u *User) CheckPassword(password string) bool {
	// In production code, use proper password verification:
	// err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	// return err == nil

	// This is just a placeholder and NOT secure!
	return u.Password == "hashed_"+password
}

// BeforeCreate prepares a user for creation
func (u *User) BeforeCreate() error {
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return u.HashPassword()
}

// BeforeUpdate prepares a user for update
func (u *User) BeforeUpdate() {
	u.UpdatedAt = time.Now()
}

// SoftDelete marks a user as deleted without removing from the database
func (u *User) SoftDelete() {
	now := time.Now()
	u.DeletedAt = &now
	u.Active = false
}

// IsDeleted checks if a user has been soft-deleted
func (u *User) IsDeleted() bool {
	return u.DeletedAt != nil
}

// ToPublicUser returns a copy of the user with sensitive data removed
func (u *User) ToPublicUser() User {
	return User{
		ID:            u.ID,
		Username:      u.Username,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Role:          u.Role,
		Active:        u.Active,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
		DeletedAt:     u.DeletedAt,
	}
}

// FullName returns the user's full name
func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return u.Username
	}
	return u.FirstName + " " + u.LastName
}