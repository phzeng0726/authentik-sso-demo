package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"site-b/backend/internal/auth"
	"site-b/backend/internal/auth/mock"
	"site-b/backend/internal/config"
	"site-b/backend/internal/handler"
)

var alice = auth.User{Sub: "abc", Username: "alice", Name: "Alice", Email: "alice@example.com"}

type AuthSuite struct {
	suite.Suite
	ctrl     *gomock.Controller
	provider *mock.MockProvider
	now      time.Time
	store    *auth.Store
	router   *gin.Engine
}

func TestAuthSuite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	suite.Run(t, new(AuthSuite))
}

func (s *AuthSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.provider = mock.NewMockProvider(s.ctrl)
	s.now = time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	s.store = auth.NewStore(func() time.Time { return s.now })
	s.router = handler.NewRouter(config.Config{SiteName: "Site B"}, s.provider, s.store)
}

func (s *AuthSuite) do(method, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	return rec
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// startLogin runs GET /api/auth/login and returns the login-flow cookie and
// the state, nonce and verifier passed to the provider.
func (s *AuthSuite) startLogin() (flow *http.Cookie, state, nonce, verifier string) {
	return s.startLoginAt("/api/auth/login", false)
}

// startLoginAt runs a login request at target and checks that the provider
// is asked for a silent login exactly when wantSilent is true.
func (s *AuthSuite) startLoginAt(target string, wantSilent bool) (flow *http.Cookie, state, nonce, verifier string) {
	s.provider.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any(), wantSilent).
		DoAndReturn(func(st, n, v string, _ bool) string {
			state, nonce, verifier = st, n, v
			return "http://idp/authorize?x=1"
		})
	rec := s.do(http.MethodGet, target)
	s.Require().Equal(http.StatusFound, rec.Code)
	flow = cookieNamed(rec, "site_b_login")
	s.Require().NotNil(flow)
	return flow, state, nonce, verifier
}

func (s *AuthSuite) callback(query url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	return s.do(http.MethodGet, "/api/auth/callback?"+query.Encode(), cookies...)
}

// --- login ---

func (s *AuthSuite) TestLogin_RedirectsToProviderAndStoresPendingLogin() {
	var gotState, gotNonce, gotVerifier string
	s.provider.EXPECT().AuthCodeURL(gomock.Any(), gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(state, nonce, verifier string, _ bool) string {
			gotState, gotNonce, gotVerifier = state, nonce, verifier
			return "http://idp/authorize?state=" + state
		})

	rec := s.do(http.MethodGet, "/api/auth/login")

	s.Equal(http.StatusFound, rec.Code)
	s.Equal("http://idp/authorize?state="+gotState, rec.Header().Get("Location"))
	s.Len(gotState, 43)
	s.Len(gotNonce, 43)
	s.Len(gotVerifier, 43)
	s.NotEqual(gotState, gotNonce)

	flow := cookieNamed(rec, "site_b_login")
	s.Require().NotNil(flow)
	s.True(flow.HttpOnly)
	s.Equal(http.SameSiteLaxMode, flow.SameSite)
	s.Equal("/api/auth", flow.Path)
	s.Equal(600, flow.MaxAge)

	pending, ok := s.store.TakePendingLogin(flow.Value)
	s.Require().True(ok)
	s.Equal(gotState, pending.State)
	s.Equal(gotNonce, pending.Nonce)
	s.Equal(gotVerifier, pending.Verifier)
	s.False(pending.Silent)
}

func (s *AuthSuite) TestLogin_PromptNone_StartsSilentLogin() {
	flow, state, _, _ := s.startLoginAt("/api/auth/login?prompt=none", true)

	pending, ok := s.store.TakePendingLogin(flow.Value)
	s.Require().True(ok)
	s.True(pending.Silent)
	s.Equal(state, pending.State)
}

func (s *AuthSuite) TestLogin_OtherPromptValues_AreIgnored() {
	for _, target := range []string{"/api/auth/login?prompt=login", "/api/auth/login?prompt=", "/api/auth/login?prompt=NONE"} {
		s.Run(target, func() {
			flow, _, _, _ := s.startLoginAt(target, false)

			pending, ok := s.store.TakePendingLogin(flow.Value)
			s.Require().True(ok)
			s.False(pending.Silent)
		})
	}
}

// --- callback ---

func (s *AuthSuite) TestCallback_Success_CreatesSession() {
	flow, state, nonce, verifier := s.startLogin()
	s.provider.EXPECT().Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, code, v string) (auth.IDToken, error) {
			s.Equal("code-1", code)
			s.Equal(verifier, v)
			return auth.IDToken{Raw: "raw-id-token", User: alice, Nonce: nonce}, nil
		})

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.Equal(http.StatusFound, rec.Code)
	s.Equal("/", rec.Header().Get("Location"))

	sc := cookieNamed(rec, "site_b_session")
	s.Require().NotNil(sc)
	s.True(sc.HttpOnly)
	s.Equal(http.SameSiteLaxMode, sc.SameSite)
	s.Equal("/", sc.Path)
	s.Equal(8*60*60, sc.MaxAge)

	sess, ok := s.store.GetSession(sc.Value)
	s.Require().True(ok)
	s.Equal(alice, sess.User)
	s.Equal("raw-id-token", sess.IDToken)

	// The login-flow cookie is cleared and the pending login cannot be reused.
	cleared := cookieNamed(rec, "site_b_login")
	s.Require().NotNil(cleared)
	s.Equal(-1, cleared.MaxAge)
	_, reusable := s.store.TakePendingLogin(flow.Value)
	s.False(reusable)
}

func (s *AuthSuite) assertAuthError(rec *httptest.ResponseRecorder, code string) {
	s.Equal(http.StatusFound, rec.Code)
	s.Equal("/?auth_error="+code, rec.Header().Get("Location"))
	s.Nil(cookieNamed(rec, "site_b_session"))
}

func (s *AuthSuite) TestCallback_StateMismatch_ReturnsInvalidState() {
	flow, _, _, _ := s.startLogin()

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {"forged"}}, flow)

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_NoFlowCookie_ReturnsInvalidState() {
	_, state, _, _ := s.startLogin()

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}})

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_UnknownPendingLogin_ReturnsInvalidState() {
	rec := s.callback(url.Values{"code": {"code-1"}, "state": {""}},
		&http.Cookie{Name: "site_b_login", Value: "unknown"})

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_ExpiredPendingLogin_ReturnsInvalidState() {
	flow, state, _, _ := s.startLogin()
	s.now = s.now.Add(10 * time.Minute)

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_PendingLoginReused_ReturnsInvalidState() {
	flow, state, nonce, _ := s.startLogin()
	s.provider.EXPECT().Exchange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(auth.IDToken{Raw: "raw-id-token", User: alice, Nonce: nonce}, nil)
	_ = s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_ProviderError_ReturnsAccessDenied() {
	flow, state, _, _ := s.startLogin()

	rec := s.callback(url.Values{"error": {"access_denied"}, "state": {state}}, flow)

	s.assertAuthError(rec, "access_denied")
}

func (s *AuthSuite) TestCallback_SilentNotSignedIn_GoesHomeWithoutError() {
	for _, e := range []string{"login_required", "consent_required", "interaction_required"} {
		s.Run(e, func() {
			flow, state, _, _ := s.startLoginAt("/api/auth/login?prompt=none", true)

			rec := s.callback(url.Values{"error": {e}, "state": {state}}, flow)

			s.Equal(http.StatusFound, rec.Code)
			s.Equal("/", rec.Header().Get("Location"))
			s.Nil(cookieNamed(rec, "site_b_session"))
			cleared := cookieNamed(rec, "site_b_login")
			s.Require().NotNil(cleared)
			s.Equal(-1, cleared.MaxAge)
		})
	}
}

func (s *AuthSuite) TestCallback_SilentOtherError_ReturnsAccessDenied() {
	flow, state, _, _ := s.startLoginAt("/api/auth/login?prompt=none", true)

	rec := s.callback(url.Values{"error": {"server_error"}, "state": {state}}, flow)

	s.assertAuthError(rec, "access_denied")
}

func (s *AuthSuite) TestCallback_InteractiveLoginRequired_ReturnsAccessDenied() {
	flow, state, _, _ := s.startLogin()

	rec := s.callback(url.Values{"error": {"login_required"}, "state": {state}}, flow)

	s.assertAuthError(rec, "access_denied")
}

func (s *AuthSuite) TestCallback_SilentStateMismatch_ReturnsInvalidState() {
	flow, _, _, _ := s.startLoginAt("/api/auth/login?prompt=none", true)

	rec := s.callback(url.Values{"error": {"login_required"}, "state": {"forged"}}, flow)

	s.assertAuthError(rec, "invalid_state")
}

func (s *AuthSuite) TestCallback_SilentSuccess_CreatesSession() {
	flow, state, nonce, _ := s.startLoginAt("/api/auth/login?prompt=none", true)
	s.provider.EXPECT().Exchange(gomock.Any(), "code-1", gomock.Any()).
		Return(auth.IDToken{Raw: "raw-id-token", User: alice, Nonce: nonce}, nil)

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.Equal(http.StatusFound, rec.Code)
	s.Equal("/", rec.Header().Get("Location"))
	sc := cookieNamed(rec, "site_b_session")
	s.Require().NotNil(sc)
	sess, ok := s.store.GetSession(sc.Value)
	s.Require().True(ok)
	s.Equal(alice, sess.User)
}

func (s *AuthSuite) TestCallback_ExchangeFails_ReturnsTokenError() {
	flow, state, _, _ := s.startLogin()
	s.provider.EXPECT().Exchange(gomock.Any(), "code-1", gomock.Any()).
		Return(auth.IDToken{}, auth.ErrTokenExchange)

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.assertAuthError(rec, "token_error")
}

func (s *AuthSuite) TestCallback_InvalidIDToken_ReturnsTokenError() {
	flow, state, _, _ := s.startLogin()
	s.provider.EXPECT().Exchange(gomock.Any(), "code-1", gomock.Any()).
		Return(auth.IDToken{}, errors.Join(auth.ErrInvalidIDToken, errors.New("bad signature")))

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.assertAuthError(rec, "token_error")
}

func (s *AuthSuite) TestCallback_NonceMismatch_ReturnsTokenError() {
	flow, state, _, _ := s.startLogin()
	s.provider.EXPECT().Exchange(gomock.Any(), "code-1", gomock.Any()).
		Return(auth.IDToken{Raw: "raw-id-token", User: alice, Nonce: "replayed-nonce"}, nil)

	rec := s.callback(url.Values{"code": {"code-1"}, "state": {state}}, flow)

	s.assertAuthError(rec, "token_error")
}

// --- me ---

func (s *AuthSuite) TestMe_LoggedIn_ReturnsUser() {
	sess := s.store.CreateSession(alice, "raw-id-token")

	rec := s.do(http.MethodGet, "/api/me", &http.Cookie{Name: "site_b_session", Value: sess.ID})

	s.Equal(http.StatusOK, rec.Code)
	var body handler.MeResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(handler.MeResponse{Sub: "abc", Username: "alice", Name: "Alice", Email: "alice@example.com"}, body)
}

func (s *AuthSuite) TestMe_NotLoggedIn_ReturnsUnauthorized() {
	rec := s.do(http.MethodGet, "/api/me")

	s.Equal(http.StatusUnauthorized, rec.Code)
	s.JSONEq(`{"error":"unauthorized"}`, rec.Body.String())
}

// --- logout ---

func (s *AuthSuite) TestLogout_WithSession_DeletesSessionAndReturnsHintedURL() {
	sess := s.store.CreateSession(alice, "raw-id-token")
	other := s.store.CreateSession(alice, "other-token")
	s.provider.EXPECT().EndSessionURL("raw-id-token").
		Return("http://idp/end-session/?id_token_hint=raw-id-token")

	rec := s.do(http.MethodPost, "/api/auth/logout", &http.Cookie{Name: "site_b_session", Value: sess.ID})

	s.Equal(http.StatusOK, rec.Code)
	var body handler.LogoutResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal(handler.LogoutResponse{LogoutURL: "http://idp/end-session/?id_token_hint=raw-id-token"}, body)

	_, ok := s.store.GetSession(sess.ID)
	s.False(ok)
	_, otherOK := s.store.GetSession(other.ID)
	s.True(otherOK)

	cleared := cookieNamed(rec, "site_b_session")
	s.Require().NotNil(cleared)
	s.Equal(-1, cleared.MaxAge)
	s.Equal("/", cleared.Path)
}

func (s *AuthSuite) TestLogout_WithoutSession_ReturnsPlainURL() {
	s.provider.EXPECT().EndSessionURL("").Return("http://idp/end-session/")

	rec := s.do(http.MethodPost, "/api/auth/logout")

	s.Equal(http.StatusOK, rec.Code)
	s.JSONEq(`{"logout_url":"http://idp/end-session/"}`, rec.Body.String())
}

func (s *AuthSuite) TestLogout_GetIsNotAllowed() {
	sess := s.store.CreateSession(alice, "raw-id-token")

	rec := s.do(http.MethodGet, "/api/auth/logout", &http.Cookie{Name: "site_b_session", Value: sess.ID})

	s.Equal(http.StatusNotFound, rec.Code)
	_, ok := s.store.GetSession(sess.ID)
	s.True(ok)
}

// --- public routes ---

func (s *AuthSuite) TestPublicRoutes_NoSessionNeeded() {
	for _, path := range []string{"/api/health", "/api/info"} {
		s.Run(path, func() {
			s.Equal(http.StatusOK, s.do(http.MethodGet, path).Code)
		})
	}
}
