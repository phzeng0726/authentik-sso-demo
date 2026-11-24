// Package handler defines the HTTP handlers and router of the backend.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthResponse is the body of GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// InfoResponse is the body of GET /api/info.
type InfoResponse struct {
	Name string `json:"name"`
}

// Handler serves the API endpoints.
type Handler struct {
	siteName string
}

// New creates a Handler for the given site name.
func New(siteName string) *Handler {
	return &Handler{siteName: siteName}
}

// Health reports that the service is running.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}

// Info returns the site identity.
func (h *Handler) Info(c *gin.Context) {
	c.JSON(http.StatusOK, InfoResponse{Name: h.siteName})
}
