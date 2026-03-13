package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents an authorized user within up system.
type User struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedOn time.Time `json:"created_on"`
}
