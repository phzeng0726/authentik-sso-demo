package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
)

// captured is what the test server received.
type captured struct {
	Method      string
	Path        string
	Query       string
	Auth        string
	ContentType string
	Body        string
}

type ClientSuite struct {
	suite.Suite
	got    captured
	status int
	reply  string
	server *httptest.Server
	client *Client
}

func TestClientSuite(t *testing.T) {
	suite.Run(t, new(ClientSuite))
}

func (s *ClientSuite) SetupTest() {
	s.got = captured{}
	s.status = http.StatusOK
	s.reply = `{}`
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.got = captured{
			Method:      r.Method,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			Auth:        r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"),
			Body:        string(body),
		}
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.reply)
	}))
	// Trailing slash checks that NewClient does not produce "//api".
	s.client = NewClient(s.server.URL+"/", "tok", s.server.Client())
}

func (s *ClientSuite) TearDownTest() {
	s.server.Close()
}

func (s *ClientSuite) TestFindUserByUsername_Exists_ReturnsUser() {
	s.reply = `{"results":[{"pk":7,"username":"alice"}]}`

	u, found, err := s.client.FindUserByUsername(context.Background(), "alice")

	s.Require().NoError(err)
	s.True(found)
	s.Equal(User{PK: 7, Username: "alice"}, u)
	s.Equal(captured{Method: "GET", Path: "/api/v3/core/users/", Query: "username=alice", Auth: "Bearer tok"}, s.got)
}

func (s *ClientSuite) TestFindUserByUsername_OnlyOtherUsernames_ReturnsNotFound() {
	s.reply = `{"results":[{"pk":8,"username":"alice2"}]}`

	u, found, err := s.client.FindUserByUsername(context.Background(), "alice")

	s.Require().NoError(err)
	s.False(found)
	s.Equal(User{}, u)
}

func (s *ClientSuite) TestFindUserByUsername_Forbidden_ReturnsAPIError() {
	s.status = http.StatusForbidden
	s.reply = "{\"detail\":\"no\"}\n"

	_, _, err := s.client.FindUserByUsername(context.Background(), "alice")

	apiErr, ok := errors.AsType[*APIError](err)
	s.Require().True(ok, "got %v", err)
	s.Equal(APIError{Method: "GET", Path: "/core/users/", StatusCode: 403, Body: `{"detail":"no"}`}, *apiErr)
	s.Equal(`GET /core/users/: HTTP 403: {"detail":"no"}`, err.Error())
}

func (s *ClientSuite) TestFindGroupByName_Exists_ReturnsGroup() {
	s.reply = `{"results":[{"pk":"uuid-1","name":"site a"}]}`

	g, found, err := s.client.FindGroupByName(context.Background(), "site a")

	s.Require().NoError(err)
	s.True(found)
	s.Equal(Group{PK: "uuid-1", Name: "site a"}, g)
	s.Equal(captured{Method: "GET", Path: "/api/v3/core/groups/", Query: "name=site+a", Auth: "Bearer tok"}, s.got)
}

func (s *ClientSuite) TestFindGroupByName_OnlyOtherNames_ReturnsNotFound() {
	s.reply = `{"results":[{"pk":"uuid-2","name":"site a admins"}]}`

	g, found, err := s.client.FindGroupByName(context.Background(), "site a")

	s.Require().NoError(err)
	s.False(found)
	s.Equal(Group{}, g)
}

func (s *ClientSuite) TestFindGroupByName_ServerError_ReturnsAPIError() {
	s.status = http.StatusInternalServerError
	s.reply = "boom"

	_, _, err := s.client.FindGroupByName(context.Background(), "site a")

	apiErr, ok := errors.AsType[*APIError](err)
	s.Require().True(ok, "got %v", err)
	s.Equal(500, apiErr.StatusCode)
	s.Equal("/core/groups/", apiErr.Path)
}

func (s *ClientSuite) TestCreateGroup_SendsNameAndReturnsGroup() {
	s.status = http.StatusCreated
	s.reply = `{"pk":"uuid-1","name":"site-a-users"}`

	g, err := s.client.CreateGroup(context.Background(), "site-a-users")

	s.Require().NoError(err)
	s.Equal(Group{PK: "uuid-1", Name: "site-a-users"}, g)
	s.Equal(captured{
		Method: "POST", Path: "/api/v3/core/groups/", Auth: "Bearer tok",
		ContentType: "application/json", Body: `{"name":"site-a-users"}`,
	}, s.got)
}

func (s *ClientSuite) TestCreateUser_AllFields_SendsEveryField() {
	s.status = http.StatusCreated
	s.reply = `{"pk":9,"username":"alice"}`

	u, err := s.client.CreateUser(context.Background(), NewUser{
		Username: "alice", Name: "Alice", Email: "a@example.com",
		IsActive: true, Path: "users", Type: "internal", Groups: []string{"g1", "g2"},
	})

	s.Require().NoError(err)
	s.Equal(User{PK: 9, Username: "alice"}, u)
	s.Equal("POST", s.got.Method)
	s.Equal("/api/v3/core/users/", s.got.Path)
	s.Equal("application/json", s.got.ContentType)
	s.JSONEq(`{"username":"alice","name":"Alice","email":"a@example.com","is_active":true,
		"path":"users","type":"internal","groups":["g1","g2"]}`, s.got.Body)
}

func (s *ClientSuite) TestCreateUser_NoGroupsNoEmail_SendsEmptyGroupsAndOmitsEmail() {
	s.status = http.StatusCreated
	s.reply = `{"pk":9,"username":"bob"}`

	_, err := s.client.CreateUser(context.Background(), NewUser{Username: "bob", Name: "bob"})

	s.Require().NoError(err)
	s.JSONEq(`{"username":"bob","name":"bob","is_active":false,"path":"","type":"","groups":[]}`, s.got.Body)
}

func (s *ClientSuite) TestCreateUser_BadRequest_ReturnsAPIError() {
	s.status = http.StatusBadRequest
	s.reply = `{"username":["already exists"]}`

	_, err := s.client.CreateUser(context.Background(), NewUser{Username: "bob", Name: "bob"})

	apiErr, ok := errors.AsType[*APIError](err)
	s.Require().True(ok, "got %v", err)
	s.Equal(APIError{Method: "POST", Path: "/core/users/", StatusCode: 400, Body: `{"username":["already exists"]}`}, *apiErr)
}

func (s *ClientSuite) TestSetPassword_NoContent_SendsPassword() {
	s.status = http.StatusNoContent
	s.reply = ""

	err := s.client.SetPassword(context.Background(), 9, "secret")

	s.Require().NoError(err)
	s.Equal(captured{
		Method: "POST", Path: "/api/v3/core/users/9/set_password/", Auth: "Bearer tok",
		ContentType: "application/json", Body: `{"password":"secret"}`,
	}, s.got)
}

func (s *ClientSuite) TestSetPassword_BadRequest_ReturnsAPIError() {
	s.status = http.StatusBadRequest
	s.reply = `{"password":["too short"]}`

	err := s.client.SetPassword(context.Background(), 9, "x")

	apiErr, ok := errors.AsType[*APIError](err)
	s.Require().True(ok, "got %v", err)
	s.Equal(APIError{Method: "POST", Path: "/core/users/9/set_password/", StatusCode: 400, Body: `{"password":["too short"]}`}, *apiErr)
}

func (s *ClientSuite) TestDo_Status299_IsSuccess() {
	s.status = 299
	s.reply = ""

	err := s.client.SetPassword(context.Background(), 1, "x")

	s.NoError(err)
}

func (s *ClientSuite) TestDo_Status300_IsAPIError() {
	s.status = 300
	s.reply = ""

	err := s.client.SetPassword(context.Background(), 1, "x")

	apiErr, ok := errors.AsType[*APIError](err)
	s.Require().True(ok, "got %v", err)
	s.Equal(300, apiErr.StatusCode)
}

func (s *ClientSuite) TestDo_InvalidJSON_ReturnsDecodeError() {
	s.reply = `not json`

	_, _, err := s.client.FindUserByUsername(context.Background(), "alice")

	_, ok := errors.AsType[*json.SyntaxError](err)
	s.True(ok, "got %v", err)
	s.Equal("decode GET /core/users/ response: invalid character 'o' in literal null (expecting 'u')", err.Error())
}

func (s *ClientSuite) TestDo_ServerUnreachable_ReturnsURLError() {
	s.server.Close()

	_, _, err := s.client.FindUserByUsername(context.Background(), "alice")

	_, ok := errors.AsType[*url.Error](err)
	s.True(ok, "got %v", err)
}

func (s *ClientSuite) TestDo_InvalidBaseURL_ReturnsURLError() {
	client := NewClient("http://bad host", "tok", s.server.Client())

	_, _, err := client.FindUserByUsername(context.Background(), "alice")

	_, ok := errors.AsType[*url.Error](err)
	s.True(ok, "got %v", err)
}
