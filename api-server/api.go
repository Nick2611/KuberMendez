package apiserver

import (
	"context"
	"fmt"
	"kuberMendez/events"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Start(ctx context.Context, eventStream chan<- events.Message) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown:", err)
	}
	fmt.Println("Server exiting")
}

func setupRouter(eventStream chan<- events.Message) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "OK"})
	})

	r.POST("/events/reconcile", func(ctx *gin.Context) {
		var message events.Message
		if err := ctx.ShouldBindJSON(&message); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		select {
		case eventStream <- message:
			ctx.JSON(http.StatusAccepted, gin.H{"status": "queued"})
		default:
			ctx.JSON(http.StatusAccepted, gin.H{"status": "already queued"})
		}
	})

	return r
}
