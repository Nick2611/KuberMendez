package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	runtimecontract "kuberMendez/runtime"

	"github.com/gin-gonic/gin"
)

type APIRuntime interface {
	ListContainers(ctx context.Context, deploymentName string) ([]runtimecontract.ContainerState, error)
	RemoveContainers(ctx context.Context, deploymentName string) error
}

func Start(ctx context.Context, eventStream chan<- ApplyRequestDto, runtime APIRuntime) {
	port := ":8080"
	r := setupRouter(eventStream, runtime)
	srv := &http.Server{
		Addr:    port,
		Handler: r.Handler(),
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown:", err)
	}
	fmt.Println("Server exiting")
}

func setupRouter(eventStream chan<- ApplyRequestDto, runtime APIRuntime) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	r.POST("/events/reconcile", CallReconcile(eventStream))

	r.GET("/status", GetDeploymentStatus(runtime))
	//TODO get /status/all endpoint (debug endpoint?)

	r.POST("/events/delete", DeleteDeployment(runtime))

	r.GET("/logs", StreamLogs())

	return r
}
