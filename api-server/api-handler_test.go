package apiserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCallReconcileReturnsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	eventStream := make(chan ApplyRequestDto, 1)
	router := setupRouter(eventStream)
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

	response := performReconcileRequest(router, `{"deploymentName":"Nico.yaml"}`)

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
	router := setupRouter(make(chan ApplyRequestDto, 1))

	response := performReconcileRequest(router, `{}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestCallReconcileReturnsBusyWhenQueueIsFull(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter(make(chan ApplyRequestDto))

	response := performReconcileRequest(router, `{"deploymentName":"Nico.yaml"}`)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusConflict, response.Body.String())
	}
}

func TestCallReconcileReturnsReconcileError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	eventStream := make(chan ApplyRequestDto, 1)
	router := setupRouter(eventStream)

	go func() {
		request := <-eventStream
		request.Reply <- ReconcileResultDto{
			DeploymentName: "Nico.yaml",
			Created:        false,
			Err:            errors.New("deployment file not found"),
		}
	}()

	response := performReconcileRequest(router, `{"deploymentName":"Nico.yaml"}`)

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

func performReconcileRequest(router http.Handler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/events/reconcile", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
