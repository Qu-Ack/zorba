package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (c apiConfig) handleSignUp(w http.ResponseWriter, r *http.Request) {
	type body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var b body

	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&b)

	if err != nil {
		WriteJSONError(w, 500, "Error in reading body")
		return
	}
	fmt.Print(b.Username)
	fmt.Print(b.Password)
	fmt.Print("user")

	err = c.createUser(b.Username, b.Password, r.Context())

	if err != nil {
		WriteJSONError(w, 500, "Internal Server Error")
		return
	}

	WriteJSON(w, 201, map[string]string{"status": "success"})
}

func (c apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {
	type body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var b body

	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&b)

	if err != nil {
		WriteJSONError(w, 500, "Error in reading body")
		return
	}

	user, err := c.getUserByEmail(b.Username, r.Context())

	if err != nil {
		WriteJSONError(w, 500, "No User")
		return
	}

	if user.Password != b.Password {
		WriteJSONError(w, 401, "wrong password")
		return
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp":   time.Now().Add(time.Hour).Unix(),
		"id":    user.ID,
		"email": user.Username,
	})
	s, err := t.SignedString([]byte("tryandbruteforcethisbitch"))

	if err != nil {
		WriteJSONError(w, 500, "Internal Server Error")
		return
	}

	WriteJSON(w, 200, map[string]any{
		"Token": s,
		"User": map[string]any{
			"ID":       user.ID,
			"username": user.Username,
		},
	})

}
