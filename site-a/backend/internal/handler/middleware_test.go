package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"

	"site-a/backend/internal/auth"
	"site-a/backend/internal/handler"
)

type MiddlewareSuite struct {
	suite.Suite
	now    time.Time
	store  *auth.Store
	router *gin.Engine
}

func TestMiddlewareSuite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suite.Run(t, new(MiddlewareSuite))
}

func (s *MiddlewareSuite) SetupTest() {
	s.now = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	s.store = auth.NewStore(func() time.Time { return s.now })

	s.router = gin.New()
	s.router.GET("/protected", handler.RequireAuth(s.store), func(c *gin.Context) {
		u, ok := handler.CurrentUser(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, u)
	})
}

func (s *MiddlewareSuite) get(cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func (s *MiddlewareSuite) assertUnauthorized(rec *httptest.ResponseRecorder) {
	s.Equal(http.StatusUnauthorized, rec.Code)
	var body handler.ErrorResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(handler.ErrorResponse{Error: "unauthorized"}, body)
}

func (s *MiddlewareSuite) TestValidSession_PassesUserToHandler() {
	sess := s.store.CreateSession(alice, "raw-id-token")

	rec := s.get(&http.Cookie{Name: "site_a_session", Value: sess.ID})

	s.Equal(http.StatusOK, rec.Code)
	var got auth.User
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(alice, got)
}

func (s *MiddlewareSuite) TestNoCookie_ReturnsUnauthorized() {
	s.assertUnauthorized(s.get(nil))
}

func (s *MiddlewareSuite) TestUnknownSession_ReturnsUnauthorized() {
	s.assertUnauthorized(s.get(&http.Cookie{Name: "site_a_session", Value: "unknown"}))
}

func (s *MiddlewareSuite) TestOtherSiteCookie_ReturnsUnauthorized() {
	sess := s.store.CreateSession(alice, "raw-id-token")

	// A valid session ID under the other site's cookie name must not be accepted.
	s.assertUnauthorized(s.get(&http.Cookie{Name: "site_b_session", Value: sess.ID}))
}

func (s *MiddlewareSuite) TestExpiredSession_ReturnsUnauthorized() {
	sess := s.store.CreateSession(alice, "raw-id-token")
	s.now = s.now.Add(8 * time.Hour)

	s.assertUnauthorized(s.get(&http.Cookie{Name: "site_a_session", Value: sess.ID}))
}
