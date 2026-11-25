package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"site-b/backend/internal/auth"
)

const userContextKey = "auth.user"

// ErrorResponse is the body of error responses such as 401.
type ErrorResponse struct {
	Error string `json:"error"`
}

// RequireAuth rejects requests without a valid session with 401, and otherwise
// stores the session user in the context for CurrentUser.
func RequireAuth(store *auth.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := c.Cookie(SessionCookieName)
		if err != nil {
			abortUnauthorized(c)
			return
		}
		sess, ok := store.GetSession(id)
		if !ok {
			abortUnauthorized(c)
			return
		}
		c.Set(userContextKey, sess.User)
		c.Next()
	}
}

// CurrentUser returns the user stored by RequireAuth.
func CurrentUser(c *gin.Context) (auth.User, bool) {
	v, ok := c.Get(userContextKey)
	if !ok {
		return auth.User{}, false
	}
	u, ok := v.(auth.User)
	return u, ok
}

func abortUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
}
