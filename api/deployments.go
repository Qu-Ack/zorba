package main

import (
	"encoding/json"
	"net/http"
)

func handleDeployGo(w http.ResponseWriter, r *http.Request) {

	type body struct {
		GithubRepo string `json:"github_repo"`
	}

	var b body
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&b)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error while reading the body"))
	}

	w.Write([]byte("go deployment"))

}

func (c apiConfig) handleDeployNode(w http.ResponseWriter, r *http.Request) {
	user, ok := r.Context().Value(userContextKey).(User)
	if !ok {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	type body struct {
		ProjectName string `json:"projectName"`
		GithubRepo  string `json:"githubRepo"`
		FrameWork   string `json:"framework"`
	}

	var b body
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&b)

	if err != nil {
		WriteJSONError(w, 400, "couldn't read body")
		return
	}
	projectId, err := c.handleCreateProject(r.Context(), b.ProjectName, b.GithubRepo, b.FrameWork)
	if err != nil {
		WriteJSONError(w, 500, "Internal Server Error")
		return
	}

	err = c.addProject(r.Context(), projectId, user.ID)

	if err != nil {
		WriteJSONError(w, 500, "Internal Server Error")
		return
	}

	WriteJSON(w, 201, map[string]string{"status": "sucess"})
}

func handleDeployReact(w http.ResponseWriter, r *http.Request) {
	type body struct {
		GithubRepo string `json:"github_repo"`
	}

	var b body
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&b)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("error while reading the body"))
	}
	w.Write([]byte("react deployment"))
}
