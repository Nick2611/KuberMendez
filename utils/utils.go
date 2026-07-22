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

func StreamFileTail(writer io.Writer, fileName string, lines int) error {
	if lines <= 0 {
		return nil
	}

	absPath, err := filepath.Abs(fileName)
	if err != nil {
		return fmt.Errorf("Bad filepath %q: %w", fileName, err)
	}

	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("open file %q: %w", absPath, err)
	}
	defer file.Close()

	start, err := tailStartOffset(file, lines)
	if err != nil {
		return fmt.Errorf("tail file %q: %w", absPath, err)
	}

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("seek file %q: %w", absPath, err)
	}

	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("stream file %q: %w", absPath, err)
	}

	return nil
}

type fileChunk struct {
	offset int64
	data   []byte
}

func tailStartOffset(file *os.File, lines int) (int64, error) {
	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}

	size := stat.Size()
	if size == 0 {
		return 0, nil
	}

	const chunkSize int64 = 4096
	targetNewlines := lines
	if lastByteIsNewline(file, size) {
		targetNewlines++
	}

	var chunks []fileChunk
	var newlines int

	for remaining := size; remaining > 0 && newlines < targetNewlines; {
		readSize := chunkSize
		if remaining < chunkSize {
			readSize = remaining
		}
		remaining -= readSize

		buffer := make([]byte, readSize)
		if _, err := file.ReadAt(buffer, remaining); err != nil {
			return 0, err
		}

		chunks = append(chunks, fileChunk{offset: remaining, data: buffer})
		newlines += bytes.Count(buffer, []byte{'\n'})
	}

	seen := 0
	for _, chunk := range chunks {
		for i := len(chunk.data) - 1; i >= 0; i-- {
			if chunk.data[i] == '\n' {
				seen++
				if seen == targetNewlines {
					return chunk.offset + int64(i) + 1, nil
				}
			}
		}
	}

	return 0, nil
}

func lastByteIsNewline(file *os.File, size int64) bool {
	buffer := []byte{0}
	if _, err := file.ReadAt(buffer, size-1); err != nil {
		return false
	}
	return buffer[0] == '\n'
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
			if err != nil {
				panic(err)
			}
			fileNames = append(fileNames, file)
		}
	}

	return fileNames, nil
}

func DeleteFile(fileName string) (string, error) {
	fullpath := fmt.Sprintf("%v/%v.yaml", DefaultDeploymentsDirectory, fileName)

	err := os.Remove(fullpath)
	if os.IsNotExist(err) {
		return "File does not exist", err
	} else if err != nil {
		return "Unexpected error", err
	}

	return "Deployment removed successfully", nil
}
