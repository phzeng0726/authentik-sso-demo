package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"

	"site-b/backend/internal/config"
	"site-b/backend/internal/handler"
)

type HandlerSuite struct {
	suite.Suite
	router *gin.Engine
}

func TestHandlerSuite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suite.Run(t, new(HandlerSuite))
}

func (s *HandlerSuite) SetupTest() {
	s.router = handler.NewRouter(config.Config{Port: 8082, SiteName: "Test Site"})
}

func (s *HandlerSuite) do(method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func (s *HandlerSuite) TestHealth_ReturnsOK() {
	rec := s.do(http.MethodGet, "/api/health")

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body handler.HealthResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(handler.HealthResponse{Status: "ok"}, body)
}

func (s *HandlerSuite) TestInfo_ReturnsConfiguredSiteName() {
	rec := s.do(http.MethodGet, "/api/info")

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json; charset=utf-8", rec.Header().Get("Content-Type"))

	var body handler.InfoResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(handler.InfoResponse{Name: "Test Site"}, body)
}

func (s *HandlerSuite) TestInfo_UsesSiteNameFromConfig() {
	router := handler.NewRouter(config.Config{Port: 8082, SiteName: "Another Site"})
	req := httptest.NewRequest(http.MethodGet, "/api/info", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.JSONEq(`{"name":"Another Site"}`, rec.Body.String())
}

func (s *HandlerSuite) TestUndefinedRoute_ReturnsNotFound() {
	cases := []string{"/api/does-not-exist", "/health", "/"}

	for _, path := range cases {
		s.Run(path, func() {
			rec := s.do(http.MethodGet, path)

			s.Equal(http.StatusNotFound, rec.Code)
		})
	}
}
