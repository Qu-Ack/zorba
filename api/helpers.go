package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func detectProjectType(projectPath string) (string, error) {
	// Check for React/Vite specific files
	if _, err := os.Stat(filepath.Join(projectPath, "vite.config.js")); err == nil {
		return "react", nil
	}
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
		// Read package.json to check for React dependencies
		data, _ := os.ReadFile(filepath.Join(projectPath, "package.json"))
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			if _, ok := pkg.Dependencies["react"]; ok {
				return "react", nil
			}
			if _, ok := pkg.DevDependencies["react"]; ok {
				return "react", nil
			}
		}
	}

	// Check for Node.js backend indicators
	if _, err := os.Stat(filepath.Join(projectPath, "server.js")); err == nil {
		return "node", nil
	}
	if _, err := os.Stat(filepath.Join(projectPath, "index.js")); err == nil {
		return "node", nil
	}
	if _, err := os.Stat(filepath.Join(projectPath, "src", "index.js")); err == nil {
		return "node", nil
	}

	// Fallback to directory structure analysis
	if isNodeProject(projectPath) {
		return "node", nil
	}

	return "", fmt.Errorf("could not determine project type")
}

func isNodeProject(path string) bool {
	if _, err := os.Stat(filepath.Join(path, "package.json")); err != nil {
		return false
	}

	// Check for common Node.js framework files
	frameworks := []string{
		"nest-cli.json", // NestJS
		"angular.json",  // Angular
		"express.js",    // Express
		"koa.js",        // Koa
	}

	for _, file := range frameworks {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			return true
		}
	}
	return false
}

func gitClone(repoURL, branch, targetDir string) error {
	if !strings.HasPrefix(repoURL, "http") && !strings.HasPrefix(repoURL, "git@") {
		return fmt.Errorf("invalid repository URL: %s", repoURL)
	}

	os.RemoveAll(targetDir)

	maxRetries := 3
	var err error

	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("git", "clone", "-b", branch, "--depth=1", repoURL, targetDir)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		if err = cmd.Run(); err == nil {
			return nil
		}

		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	return fmt.Errorf("failed to clone repository after %d attempts: %v", maxRetries, err)
}

func hasDockerfile(projectPath string) bool {
	dockerfileNames := []string{
		"Dockerfile",
		"dockerfile",
		"Dockerfile.prod",
		"Dockerfile.production",
	}

	for _, name := range dockerfileNames {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			return true
		}
	}
	return false
}

func generateDockerfile(projectType, projectPath string) error {
	var content string

	switch projectType {
	case "node":
		content = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
ENV PORT=3000
EXPOSE 3000
CMD ["node", "src/index.js"]`

	case "react":
		content = `# Build stage
FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build
`

	default:
		return fmt.Errorf("unsupported project type: %s", projectType)
	}

	dockerfilePath := filepath.Join(projectPath, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %v", err)
	}

	return nil
}
