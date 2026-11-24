package handler

import (
	"github.com/gin-gonic/gin"

	"site-a/backend/internal/config"
)

// NewRouter builds the Gin engine with all API routes registered under /api.
func NewRouter(cfg config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	h := New(cfg.SiteName)

	api := r.Group("/api")
	api.GET("/health", h.Health)
	api.GET("/info", h.Info)

	return r
}
