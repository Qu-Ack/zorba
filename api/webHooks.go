package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

const (
	baseDomain    = "deploy.dakshsangal.xyz"
	projectsDir   = "/projects"
	traefikNet    = "traefik_traefik-net"
	webhookSecret = "your-secret-key"
)

var dockerCli *client.Client

type Deployment struct {
	ID        string
	RepoURL   string
	Branch    string
	Subdomain string
	Type      string
	Port      string
	Env       map[string]string
}

type GitHubPayload struct {
	Repository struct {
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Ref    string `json:"ref"`
	Pusher struct {
		Email string `json:"email"`
	} `json:"pusher"`
}

func verifySignature(body []byte, signatureHeader string) bool {
	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	expectedSignature := "sha256=" + hex.EncodeToString(expectedMAC)
	return hmac.Equal([]byte(expectedSignature), []byte(signatureHeader))
}

func handleWebHook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Unable to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	eventType := r.Header.Get("x-github-event")
	log.Printf("Received event: %s", eventType)

	switch eventType {
	case "push":
		processPushEvent(body)
	case "pull_request":
		processPullRequestEvent(body)
	default:
		log.Printf("Unhandled event type: %s", eventType)
	}

	// Respond with 200 OK
	w.WriteHeader(http.StatusOK)
}

func processPushEvent(body []byte) {
	var payload GitHubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing push event: %v", err)
		return
	}

	deployment := Deployment{
		ID:        uuid.New().String(),
		RepoURL:   payload.Repository.CloneURL,
		Branch:    strings.TrimPrefix(payload.Ref, "refs/heads/"),
		Subdomain: fmt.Sprintf("%s.%s", generateSubdomain(), baseDomain),
		Env:       make(map[string]string),
	}

	go safeDeploy(deployment)
}

func deployApplication(d Deployment) error {
	projectPath := filepath.Join(projectsDir, d.ID)

	if err := gitClone(d.RepoURL, d.Branch, projectPath); err != nil {
		return fmt.Errorf("git clone failed: %v", err)
	}

	if d.Type == "" {
		detectedType, err := detectProjectType(projectPath)
		if err != nil {
			return fmt.Errorf("project type detection failed: %v", err)
		}
		d.Type = detectedType
	}

	if !hasDockerfile(projectPath) {
		if err := generateDockerfile(d.Type, projectPath); err != nil {
			return fmt.Errorf("dockerfile generation failed: %v", err)
		}
	}

	imageName := fmt.Sprintf("%s-image", d.ID)
	if err := dockerBuild(imageName, projectPath); err != nil {
		return err
	}

	return dockerRun(imageName, d)
}

func generateSubdomain() string {
	return strings.Replace(uuid.New().String()[:8], "-", "", -1)
}

func dockerBuild(imageName, contextPath string) error {
	cmd := exec.Command("docker", "build", "-t", imageName, contextPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerRun(imageName string, d Deployment) error {
	labels := map[string]string{
		"traefik.enable":                                     "true",
		"traefik.http.routers." + d.ID + ".rule":             fmt.Sprintf("Host(`%s`)", d.Subdomain),
		"traefik.http.routers." + d.ID + ".tls.certresolver": "letsencrypt",
	}

	if d.Type == "node" {
		labels["traefik.http.services."+d.ID+".loadbalancer.server.port"] = "3000"
	} else {
		labels["traefik.http.services."+d.ID+".loadbalancer.server.port"] = "80"
	}

	_, err := dockerCli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:  imageName,
			Labels: labels,
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				traefikNet: {},
			},
		},
		nil,
		d.ID,
	)

	return err
}

func processPullRequestEvent(body []byte) {
	var payload struct {
		Action      string `json:"action"`
		PullRequest struct {
			Merged bool   `json:"merged"`
			Number int    `json:"number"`
			Title  string `json:"title"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing pull_request event: %v", err)
		return
	}
	log.Printf("PR event on repo %s, action %s", payload.Repository.FullName, payload.Action)
	if payload.Action == "closed" && payload.PullRequest.Merged {
		log.Printf("PR #%d merged: %s", payload.PullRequest.Number, payload.PullRequest.Title)
	}
}

func safeDeploy(deployment Deployment) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in deployment %s: %v", deployment.ID, r)
		}
	}()

	if err := deployApplication(deployment); err != nil {
		log.Printf("Deployment failed: %v", err)
		cleanupFailedDeployment(deployment.ID)
	}
}

func cleanupFailedDeployment(deploymentID string) {
	projectPath := filepath.Join(projectsDir, deploymentID)
	os.RemoveAll(projectPath)

	exec.Command("docker", "rmi", "-f", fmt.Sprintf("%s-image", deploymentID)).Run()
}
