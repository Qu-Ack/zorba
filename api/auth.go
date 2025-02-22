package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// handleSignUp converts your signup endpoint to Gin.
func (c *apiConfig) handleSignUp(ctx *gin.Context) {
	// Define a struct to match the expected JSON body.
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Bind the JSON body to the struct.
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error in reading body"})
		return
	}

	// Optionally log the username and password.
	fmt.Println(body.Username, body.Password)

	// Call your createUser method using the context from the request.
	if err := c.createUser(body.Username, body.Password, ctx.Request.Context()); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"status": "success"})
}

// handleLogin converts your login endpoint to Gin.
func (c *apiConfig) handleLogin(ctx *gin.Context) {
	// Define a struct to match the expected JSON body.
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Bind the JSON body to the struct.
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error in reading body"})
		return
	}

	// Retrieve the user by email (or username).
	user, err := c.getUserByEmail(body.Username, ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "No User"})
		return
	}

	// Check if the provided password matches.
	if user.Password != body.Password {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "wrong password"})
		return
	}

	// Create a new JWT token with claims.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":   time.Now().Add(time.Hour).Unix(),
		"id":    user.ID,
		"email": user.Username,
	})

	// Sign the token using your secret.
	tokenString, err := token.SignedString([]byte("tryandbruteforcethisbitch"))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	// Return the token and user information in the response.
	ctx.JSON(http.StatusOK, gin.H{
		"Token": tokenString,
		"User": gin.H{
			"ID":       user.ID,
			"username": user.Username,
		},
	})
}
