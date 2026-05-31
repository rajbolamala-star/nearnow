package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rajbolamala-star/nearnow/internal/handler"
)

func main() {
	port := getEnv("PORT", "8080")
	templatesPath := getEnv("TEMPLATES_PATH", "web/templates/*.html")
	staticPath := getEnv("STATIC_PATH", "web/static")

	h, err := handler.New(templatesPath)
	if err != nil {
		log.Fatalf("handler init: %v", err)
	}

	gin.SetMode(getEnv("GIN_MODE", "release"))
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(cors.Default())

	r.GET("/", h.Home)
	r.GET("/api/events", h.Events)
	r.GET("/health", h.Health)
	r.Static("/static", staticPath)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("nearnow listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
