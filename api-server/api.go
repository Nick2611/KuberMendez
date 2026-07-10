package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Start(ctx context.Context) {
    port := ":8080"
    r := setupRouter()
	srv := &http.Server{
		Addr:    port,
		Handler: r.Handler(),
	}
	go func() { 
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	} ()

	<-ctx.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Println("Server Shutdown:", err)
	}
	fmt.Println("Server exiting")
}

func setupRouter() *gin.Engine {
    r := gin.Default()

    r.GET("/health", func(ctx *gin.Context) {
        ctx.JSON(http.StatusOK, gin.H{"status": "OK"})
    })

    return r
}