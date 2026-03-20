package models

import (
	"time"

	"github.com/gocql/gocql"
)

// CreateUserRequest represents a user request for the application
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// User represents a user from the database
type User struct {
	ID        gocql.UUID `json:"id"`
	Username  string     `json:"username"`
	Password  []byte     `json:"-"`
	CreatedOn time.Time  `json:"created_on"`
}
