package models

// LoginRequest represents a login request for the application
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents the response returned after a successful login
type LoginResponse struct {
	Token string `json:"token"`
}
