package handler

import (
	"github.com/gin-gonic/gin"

	"site-b/backend/internal/auth"
	"site-b/backend/internal/config"
)

// NewRouter builds the Gin engine with all API routes registered under /api.
func NewRouter(cfg config.Config, provider auth.Provider, store *auth.Store) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	h := New(cfg.SiteName)
	ah := NewAuthHandler(provider, store)

	api := r.Group("/api")

	// Public routes.
	api.GET("/health", h.Health)
	api.GET("/info", h.Info)
	api.GET("/auth/login", ah.Login)
	api.GET("/auth/callback", ah.Callback)
	api.POST("/auth/logout", ah.Logout)

	// Routes that require a signed-in user.
	protected := api.Group("", RequireAuth(store))
	protected.GET("/me", ah.Me)

	return r
}
