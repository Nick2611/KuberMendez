package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	parser "kuberMendez/deployment-parser"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultDeploymentsDirectory = ".kubermendez/deployments"
)

//HTTP HELPER FUNCTIONS

func doRequest(client *http.Client, req *http.Request, userAgent string) (io.ReadCloser, string, error) {
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode > 399 {
		resp.Body.Close()
		return nil, "", fmt.Errorf("error: %q %s", resp.Request.URL.String(), resp.Status)
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

func Get(client *http.Client, requestUrl, userAgent string) (io.ReadCloser, string, error) {
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return nil, "", err
	}
	return doRequest(client, req, userAgent)
}

func Post(client *http.Client, requestUrl, userAgent string, message map[string]string) (io.ReadCloser, string, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequest("POST", requestUrl, bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	return doRequest(client, req, userAgent)
}

// FILE HANDLERS

func GetFile(fileName string) ([]byte, error) {
	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return nil, fmt.Errorf("Bad filepath %q: %w", fileName, err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("Read file %q: %w", absPath, err)
	}

	return data, nil
}

func DeploymentStatePath(deploymentName string, deploymentsDirectory string) (string, error) {
	if deploymentName == "" {
		return "", fmt.Errorf("deployment metadata.name is required")
	}
	if filepath.Base(deploymentName) != deploymentName {
		return "", fmt.Errorf("deployment metadata.name %q cannot contain path separators", deploymentName)
	}

	return filepath.Join(deploymentsDirectory, deploymentName+".yaml"), nil
}

func SaveStateFile(fileName string, deploymentsDirectory string) (string, error) {
	file, err := GetFile(fileName)
	if err != nil {
		return "", err
	}

	manifest, err := parser.Parser(file)
	if err != nil {
		return "", err
	}

	statePath, err := DeploymentStatePath(manifest.Metadata.Name, deploymentsDirectory)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(deploymentsDirectory, 0755); err != nil {
		return "", fmt.Errorf("create deployments state directory: %w", err)
	}
	if err := os.WriteFile(statePath, file, 0644); err != nil {
		return "", fmt.Errorf("write deployment state: %w", err)
	}

	fmt.Printf("Deployment %q saved to %s\n", manifest.Metadata.Name, statePath)
	return filepath.Base(statePath), nil
}


func FileToInt(daemonPidPath string) (int, error) {
	bytes, err := os.ReadFile(daemonPidPath)
	if err != nil {
		return -1, fmt.Errorf("failed to read file: %v", err)
	}

	content := strings.TrimSpace(string(bytes))

	// Parse string to int
	num, err := strconv.Atoi(content)
	if err != nil {
		return -1, fmt.Errorf("failed to parse int: %v", err)
	}

	return num, nil
}

func GetFiles(dirPath string) ([][]byte, error) {
	var fileNames [][]byte

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			file, err := os.ReadFile(fmt.Sprintf("%v/%v", dirPath, entry.Name()))
			if err != nil{
				panic(err)
			}
			fileNames = append(fileNames, file)
		}
	}

	return fileNames, nil
}