package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"

	"site-a/backend/internal/auth"
)

const (
	// SessionCookieName is unique per site: cookies on localhost are shared
	// across ports, so the same name would let site-a and site-b overwrite each other.
	SessionCookieName = "site_a_session"
	// LoginFlowCookieName identifies the pending login between /login and /callback.
	LoginFlowCookieName = "site_a_login"

	loginFlowCookiePath = "/api/auth"
)

// Error codes sent to the frontend as /?auth_error=<code>.
const (
	authErrInvalidState = "invalid_state"
	authErrAccessDenied = "access_denied"
	authErrToken        = "token_error"
)

// silentNotSignedInErrors are the OIDC errors a prompt=none request returns
// when the user would have to interact with authentik to sign in.
var silentNotSignedInErrors = map[string]bool{
	"login_required":       true,
	"consent_required":     true,
	"interaction_required": true,
}

// MeResponse is the body of GET /api/me.
type MeResponse struct {
	Sub      string `json:"sub"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

// LogoutResponse is the body of POST /api/auth/logout.
type LogoutResponse struct {
	LogoutURL string `json:"logout_url"`
}

// AuthHandler serves the login, callback, logout and current-user endpoints.
type AuthHandler struct {
	provider auth.Provider
	store    *auth.Store
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(provider auth.Provider, store *auth.Store) *AuthHandler {
	return &AuthHandler{provider: provider, store: store}
}

// Login starts the Authorization Code Flow: it remembers state, nonce and the
// PKCE verifier server-side, then redirects the browser to authentik.
// With ?prompt=none it starts a silent login; other prompt values are ignored.
func (h *AuthHandler) Login(c *gin.Context) {
	silent := c.Query("prompt") == "none"
	pending := h.store.CreatePendingLogin(auth.RandomString(), auth.RandomString(), oauth2.GenerateVerifier(), silent)

	setCookie(c, LoginFlowCookieName, pending.ID, loginFlowCookiePath, int(auth.PendingLoginTTL.Seconds()))
	c.Redirect(http.StatusFound, h.provider.AuthCodeURL(pending.State, pending.Nonce, pending.Verifier, pending.Silent))
}

// Callback finishes the login after authentik redirects back with a code.
func (h *AuthHandler) Callback(c *gin.Context) {
	// The pending login is single-use: take it and clear its cookie whatever happens next.
	flowID, _ := c.Cookie(LoginFlowCookieName)
	pending, ok := h.store.TakePendingLogin(flowID)
	setCookie(c, LoginFlowCookieName, "", loginFlowCookiePath, -1)

	if !ok || c.Query("state") != pending.State {
		redirectWithAuthError(c, authErrInvalidState)
		return
	}
	if e := c.Query("error"); e != "" {
		// A silent attempt that fails because the user is not signed in at
		// authentik is expected: go home quietly and let the user click login.
		if pending.Silent && silentNotSignedInErrors[e] {
			c.Redirect(http.StatusFound, "/")
			return
		}
		log.Printf("login callback: authentik returned error %q: %s", e, c.Query("error_description"))
		redirectWithAuthError(c, authErrAccessDenied)
		return
	}

	idToken, err := h.provider.Exchange(c.Request.Context(), c.Query("code"), pending.Verifier)
	if err != nil {
		log.Printf("login callback: %v", err)
		redirectWithAuthError(c, authErrToken)
		return
	}
	if idToken.Nonce != pending.Nonce {
		log.Printf("login callback: nonce mismatch")
		redirectWithAuthError(c, authErrToken)
		return
	}

	sess := h.store.CreateSession(idToken.User, idToken.Raw)
	setCookie(c, SessionCookieName, sess.ID, "/", int(auth.SessionTTL.Seconds()))
	c.Redirect(http.StatusFound, "/")
}

// Me returns the signed-in user. It must run after RequireAuth.
func (h *AuthHandler) Me(c *gin.Context) {
	u, ok := CurrentUser(c)
	if !ok {
		abortUnauthorized(c)
		return
	}
	c.JSON(http.StatusOK, MeResponse{Sub: u.Sub, Username: u.Username, Name: u.Name, Email: u.Email})
}

// Logout ends this site's session and returns authentik's logout URL. It
// does not end the other site's session.
func (h *AuthHandler) Logout(c *gin.Context) {
	hint := ""
	if id, err := c.Cookie(SessionCookieName); err == nil {
		if sess, ok := h.store.GetSession(id); ok {
			hint = sess.IDToken
		}
		h.store.DeleteSession(id)
	}

	setCookie(c, SessionCookieName, "", "/", -1)
	c.JSON(http.StatusOK, LogoutResponse{LogoutURL: h.provider.EndSessionURL(hint)})
}

// setCookie writes an HttpOnly, SameSite=Lax cookie. Secure is off because the
// demo runs on plain HTTP; turn it on behind HTTPS. maxAge < 0 deletes the cookie.
func setCookie(c *gin.Context, name, value, path string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func redirectWithAuthError(c *gin.Context, code string) {
	c.Redirect(http.StatusFound, "/?auth_error="+code)
}
