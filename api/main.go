package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/docker/docker/client"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
)

type apiConfig struct {
	DB *mongo.Database
}

func main() {
	var err error

	// Initialize Docker client
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}
	_ = dockerCli // Use dockerCli as needed

	// Load environment variables
	err = godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Environment variables couldn't be initialized: %v", err)
	}

	// Get PORT from environment (default to 8080 if not set)
	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "8080"
	}

	// Connect to the database
	clientDB := ConnectDB()
	defer func() {
		if err := clientDB.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	database := clientDB.Database("zorba")
	config := apiConfig{
		DB: database,
	}

	// Initialize Gin router
	router := gin.Default()

	// CORS middleware
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://example.com", "*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})) // Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Public endpoints
	router.POST("/webhook", config.handleWebHook)
	router.POST("/login", config.handleLogin)
	router.POST("/signup", config.handleSignUp)

	// Protected routes (requires authentication)
	authorized := router.Group("/")
	authorized.Use(GinAuthMiddleware()) // This should be a Gin middleware
	{
		authorized.GET("/protected", handleProtected)
		authorized.POST("/deploy", config.handleDeploy)
	}

	log.Printf("Server listening on port %s", PORT)
	router.Run(fmt.Sprintf(":%s", PORT))
}

func (a apiConfig) handleDeploy(c *gin.Context) {
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	user, ok := userInterface.(User)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user type in context"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		http.Error(c.Writer, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer c.Request.Body.Close()

	dep := a.FirstTimeDeploy(c.Request.Context(), body, user.ID)

	c.JSON(200, gin.H{
		"deployment": dep,
	})

}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Adjust to your needs
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleProtected(c *gin.Context) {
	user, ok := c.Request.Context().Value(userContextKey).(User)
	if !ok {
		http.Error(c.Writer, "User not found in context", http.StatusUnauthorized)
		return
	}
	c.Writer.Write([]byte(fmt.Sprintf("you are authenticated %s", user.ID.Hex())))
}
