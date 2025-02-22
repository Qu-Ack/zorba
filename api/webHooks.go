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
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	ID            string
	RepoURL       string
	Branch        string
	Subdomain     string
	Type          string
	Port          string
	ContainerName string
	ImageName     string
	Env           map[string]string
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

func findExistingContainer(repoURL, branch string) (string, error) {
	containers, err := dockerCli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		if container.Labels["com.deployer.repo"] == repoURL &&
			container.Labels["com.deployer.branch"] == branch {
			return container.ID, nil
		}
	}
	return "", nil
}

func cleanupExistingDeployment(repoURL, branch string) error {
	containerID, err := findExistingContainer(repoURL, branch)
	if err != nil {
		return err
	}
	if containerID == "" {
		return nil
	}
	// Stop and remove the existing container
	timeout := int(time.Second * 30)
	if err := dockerCli.ContainerStop(context.Background(), containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return err
	}

	if err := dockerCli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{}); err != nil {
		return err
	}

	// Inspect the container to get its image name
	ctr, err := dockerCli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return err
	}
	imageName := ctr.Config.Image

	// Remove the old image
	_, err = dockerCli.ImageRemove(context.Background(), imageName, image.RemoveOptions{})
	return err
}

// processPushEvent handles push events
func processPushEvent(body []byte) {
	var payload GitHubPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Error parsing push event: %v", err)
		return
	}
	hash := sha256.Sum256([]byte(payload.Repository.CloneURL + payload.Ref))
	deploymentID := hex.EncodeToString(hash[:])[:12]

	deployment := Deployment{
		ID:            uuid.New().String(),
		RepoURL:       payload.Repository.CloneURL,
		Branch:        strings.TrimPrefix(payload.Ref, "refs/heads/"),
		Subdomain:     fmt.Sprintf("%s.%s", generateSubdomain(), baseDomain),
		Env:           make(map[string]string),
		ContainerName: fmt.Sprintf("%s-%s", deploymentID, strings.ToLower(payload.Repository.CloneURL)),
		ImageName:     fmt.Sprintf("%s-image", deploymentID),
	}

	if err := cleanupExistingDeployment(deployment.RepoURL, deployment.Branch); err != nil {
		log.Printf("Error cleaning up existing deployment: %v", err)
	}

	go safeDeploy(deployment)
}

func deployApplication(d Deployment) error {
	log.Printf("Starting deployment %s for %s", d.ID, d.RepoURL)
	defer func(start time.Time) {
		log.Printf("Deployment %s completed in %v", d.ID, time.Since(start))
	}(time.Now())

	projectPath := filepath.Join(projectsDir, d.ID)
	log.Printf("Project path: %s", projectPath)

	// Add logging to each step
	log.Println("Cloning repository...")
	if err := gitClone(d.RepoURL, d.Branch, projectPath); err != nil {
		log.Printf("Clone failed: %v", err)
		return err
	}

	log.Println("Detecting project type...")
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

// Helper functions
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

	serviceName := d.ID + "-service"
	routerName := d.ID + "-router"

	labels := map[string]string{
		"com.deployer.repo":   d.RepoURL,
		"com.deployer.branch": d.Branch,
		"traefik.enable":      "true",

		"traefik.http.routers." + routerName + ".rule":             fmt.Sprintf("Host(`%s`)", d.Subdomain),
		"traefik.http.routers." + routerName + ".entrypoints":      "websecure",
		"traefik.http.routers." + routerName + ".tls.certresolver": "letsencrypt",
		"traefik.http.routers." + routerName + ".service":          serviceName,

		// Service configuration
		"traefik.http.services." + serviceName + ".loadbalancer.server.port": "3000",
	}

	if d.Type == "node" {
		labels["traefik.http.services."+d.ID+".loadbalancer.server.port"] = "3000"
	} else {
		labels["traefik.http.services."+d.ID+".loadbalancer.server.port"] = "80"
	}

	log.Println("creating container...")
	resp, err := dockerCli.ContainerCreate(
		context.Background(),
		&container.Config{
			Image:  d.ImageName,
			Labels: labels,
			ExposedPorts: nat.PortSet{
				"3000/tcp": struct{}{},
			},
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

	if err != nil {
		log.Printf("container creation failed", err)
	}

	log.Printf("Starting container %s", resp.ID)
	if err := dockerCli.ContainerStart(
		context.Background(),
		resp.ID,
		container.StartOptions{},
	); err != nil {
		log.Printf("Container start failed: %v", err)
		return err
	}

	log.Printf("Container %s started successfully", resp.ID)
	log.Printf("http://%s.deploy.dakshsangal.xyz", d.Subdomain)
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
