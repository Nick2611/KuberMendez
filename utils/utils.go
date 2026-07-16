package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	apiserver "kuberMendez/api-server"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func Post(client *http.Client, requestUrl, userAgent string, message apiserver.ChannelMessageDto) (io.ReadCloser, string, error) {
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