package models

import (
	"time"
)

// User represents a user of the application
type User struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedOn time.Time `json:"created_on"`
}

// User_DB represents a user from the database
type User_DB struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Password  []byte    `json:"-"`
	CreatedOn time.Time `json:"created_on"`
}

type LoginResponse struct {
	Token string `json:"token"`
}
