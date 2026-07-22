package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Start(ctx context.Context, eventStream chan<- ApplyRequestDto) {
	port := ":8080"
	r := setupRouter(eventStream)
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

func setupRouter(eventStream chan<- ApplyRequestDto) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	r.POST("/events/reconcile", CallReconcile(eventStream))

	r.GET("/status", GetDeploymentStatus())
	//TODO get /status/all endpoint (debug endpoint?)

	r.POST("events/delete", DeleteDeployment())

	r.GET("/logs", StreamLogs())

	return r
}
