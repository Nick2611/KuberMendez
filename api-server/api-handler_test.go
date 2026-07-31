package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	runtimecontract "kuberMendez/runtime"

	"github.com/gin-gonic/gin"
)

type fakeAPIRuntime struct {
	containers        []runtimecontract.ContainerState
	listErr           error
	removeErr         error
	listedDeployment  string
	removedDeployment string
}

func (f *fakeAPIRuntime) ListContainers(_ context.Context, deploymentName string) ([]runtimecontract.ContainerState, error) {
	f.listedDeployment = deploymentName
	return f.containers, f.listErr
}

func (f *fakeAPIRuntime) RemoveContainers(_ context.Context, deploymentName string) error {
	f.removedDeployment = deploymentName
	return f.removeErr
}

func TestCallReconcileReturnsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	eventStream := make(chan ApplyRequestDto, 1)
	router := setupRouter(eventStream, &fakeAPIRuntime{})
	handled := make(chan struct{})

	go func() {
		request := <-eventStream
		if request.Message.DeploymentName != "Nico.yaml" {
			t.Errorf("DeploymentName = %q, want %q", request.Message.DeploymentName, "Nico.yaml")
		}
		request.Reply <- ReconcileResultDto{
			DeploymentName: "Nico",
			Created:        true,
		}
		close(handled)
	}()

	response := performReconcileRequest(router, reconcileRequestBody(t))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}

	var body ApplyResponseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "reconciled" {
		t.Fatalf("status body = %q, want %q", body.Status, "reconciled")
	}
	if body.Deployment != "Nico" {
		t.Fatalf("deployment = %q, want %q", body.Deployment, "Nico")
	}

	<-handled
}

func TestCallReconcileRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter(make(chan ApplyRequestDto, 1), &fakeAPIRuntime{})

	response := performReconcileRequest(router, `{}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestCallReconcileReturnsBusyWhenQueueIsFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter(make(chan ApplyRequestDto), &fakeAPIRuntime{})

	response := performReconcileRequest(router, reconcileRequestBody(t))

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusConflict, response.Body.String())
	}
}

func TestCallReconcileReturnsReconcileError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	eventStream := make(chan ApplyRequestDto, 1)
	router := setupRouter(eventStream, &fakeAPIRuntime{})

	go func() {
		request := <-eventStream
		request.Reply <- ReconcileResultDto{
			DeploymentName: "Nico.yaml",
			Created:        false,
			Err:            errors.New("deployment file not found"),
		}
	}()

	response := performReconcileRequest(router, reconcileRequestBody(t))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusInternalServerError, response.Body.String())
	}

	var body ApplyResponseDTO
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Message != "deployment file not found" {
		t.Fatalf("message = %q, want %q", body.Message, "deployment file not found")
	}
}

func TestGetDeploymentStatusUsesRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	runtime := &fakeAPIRuntime{
		containers: []runtimecontract.ContainerState{
			{
				ID:    "one",
				Image: "nginx",
				Ports: []runtimecontract.PortState{{ContainerPort: 80, HostPort: true}},
			},
			{
				ID:    "two",
				Image: "nginx",
				Ports: []runtimecontract.PortState{{ContainerPort: 80, HostPort: true}},
			},
		},
	}
	router := setupRouter(make(chan ApplyRequestDto, 1), runtime)

	request := httptest.NewRequest(http.MethodGet, "/status?deploymentName=Nico", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if runtime.listedDeployment != "Nico" {
		t.Fatalf("listed deployment = %q, want %q", runtime.listedDeployment, "Nico")
	}

	var body GetDeploymentStatusResponseDto
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.DeploymentName != "Nico" || body.Replicas != 2 || body.Image != "nginx" {
		t.Fatalf("unexpected status response: %+v", body)
	}
}

func TestDeleteDeploymentUsesRuntime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join(".kubermendez", "deployments"), 0755); err != nil {
		t.Fatalf("create deployment directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(".kubermendez", "deployments", "Nico.yaml"), []byte("state"), 0644); err != nil {
		t.Fatalf("write deployment state: %v", err)
	}

	runtime := &fakeAPIRuntime{}
	router := setupRouter(make(chan ApplyRequestDto, 1), runtime)
	request := httptest.NewRequest(
		http.MethodPost,
		"/events/delete",
		bytes.NewBufferString(`{"deploymentName":"Nico"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	if runtime.removedDeployment != "Nico" {
		t.Fatalf("removed deployment = %q, want %q", runtime.removedDeployment, "Nico")
	}
}

func performReconcileRequest(router http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/events/reconcile", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func reconcileRequestBody(t *testing.T) string {
	t.Helper()

	payload, err := json.Marshal(SaveDeploymentRequestDto{
		DeploymentName: writeDeploymentManifest(t),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	return string(payload)
}

func writeDeploymentManifest(t *testing.T) string {
	t.Helper()

	filePath := filepath.Join(t.TempDir(), "deployment.yaml")
	content := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: Nico
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nico-nginx
  template:
    metadata:
      labels:
        app: nico-nginx
    spec:
      containers:
        - name: nginx
          image: nginx
`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write deployment manifest: %v", err)
	}

	return filePath
}
