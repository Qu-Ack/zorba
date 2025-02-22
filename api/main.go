package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
)

type apiConfig struct {
	DB *mongo.Database
}

func main() {

	var err error

	dockerCli, err = client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	err = godotenv.Load(".env")

	if err != nil {
		panic("enviornment couldn't be initialized")
	}

	PORT := os.Getenv("PORT")
	client := ConnectDB()

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	database := client.Database("zorba")
	config := apiConfig{
		DB: database,
	}

	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	serveMux.HandleFunc("POST /webhook", handleWebHook)
	serveMux.HandleFunc("POST /deploy/go", handleDeployGo)
	serveMux.Handle("POST /deploy/node", AuthMiddleware(http.HandlerFunc(config.handleDeployNode)))
	serveMux.HandleFunc("POSt /deploy/react", handleDeployReact)
	serveMux.HandleFunc("POST /login", config.handleLogin)
	serveMux.HandleFunc("POST /signup", config.handleSignUp)

	serveMux.Handle("/protected", AuthMiddleware(http.HandlerFunc(handleProtected)))

	corsHandler := enableCORS(serveMux)
	server := http.Server{
		Addr:    fmt.Sprintf(":%v", PORT),
		Handler: corsHandler,
	}

	log.Println(fmt.Sprintf("{SERVER}: listening on %v", PORT))
	server.ListenAndServe()
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

func handleProtected(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(userContextKey).(User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Hello, %s. You are authenticated!", user.Username)))
}
