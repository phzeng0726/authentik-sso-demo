package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
)

// fakeAuthentik is an in-memory stand-in for the authentik endpoints the tool uses.
type fakeAuthentik struct {
	users     map[string]int    // username -> pk
	groups    map[string]string // name -> UUID
	created   []NewUser
	passwords map[int]string // pk -> password
	calls     []string       // "METHOD path?query"

	// failures: "METHOD path" -> status code to return instead
	fail map[string]int
}

func newFakeAuthentik() *fakeAuthentik {
	return &fakeAuthentik{
		users:     map[string]int{},
		groups:    map[string]string{},
		passwords: map[int]string{},
		fail:      map[string]int{},
	}
}

func (f *fakeAuthentik) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v3/core/users/{$}", func(w http.ResponseWriter, r *http.Request) {
		var results []User
		if pk, ok := f.users[r.URL.Query().Get("username")]; ok {
			results = append(results, User{PK: pk, Username: r.URL.Query().Get("username")})
		}
		writeJSON(w, http.StatusOK, page[User]{Results: results})
	})
	mux.HandleFunc("POST /api/v3/core/users/{$}", func(w http.ResponseWriter, r *http.Request) {
		var u NewUser
		_ = json.NewDecoder(r.Body).Decode(&u)
		pk := 100 + len(f.users)
		f.users[u.Username] = pk
		f.created = append(f.created, u)
		writeJSON(w, http.StatusCreated, User{PK: pk, Username: u.Username})
	})
	mux.HandleFunc("POST /api/v3/core/users/{pk}/set_password/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		pk, _ := strconv.Atoi(r.PathValue("pk"))
		f.passwords[pk] = body["password"]
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/v3/core/groups/{$}", func(w http.ResponseWriter, r *http.Request) {
		var results []Group
		name := r.URL.Query().Get("name")
		if id, ok := f.groups[name]; ok {
			results = append(results, Group{PK: id, Name: name})
		}
		writeJSON(w, http.StatusOK, page[Group]{Results: results})
	})
	mux.HandleFunc("POST /api/v3/core/groups/{$}", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		id := fmt.Sprintf("uuid-%s", body["name"])
		f.groups[body["name"]] = id
		writeJSON(w, http.StatusCreated, Group{PK: id, Name: body["name"]})
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			call += "?" + r.URL.RawQuery
		}
		f.calls = append(f.calls, call)
		if status, ok := f.fail[r.Method+" "+r.URL.Path]; ok {
			http.Error(w, "forced failure", status)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type CreatorSuite struct {
	suite.Suite
	fake    *fakeAuthentik
	server  *httptest.Server
	creator *Creator
}

func TestCreatorSuite(t *testing.T) {
	suite.Run(t, new(CreatorSuite))
}

func (s *CreatorSuite) SetupTest() {
	s.fake = newFakeAuthentik()
	s.server = httptest.NewServer(s.fake.handler())
	n := 0
	newPassword := func() string {
		n++
		return fmt.Sprintf("generated-%d", n)
	}
	s.creator = NewCreator(NewClient(s.server.URL, "tok", s.server.Client()), newPassword)
}

func (s *CreatorSuite) TearDownTest() {
	s.server.Close()
}

func (s *CreatorSuite) TestCreateAll_MinimalRow_CreatesUserWithGeneratedPassword() {
	results := s.creator.CreateAll(context.Background(), []Row{{Username: "carol", Name: "carol"}})

	s.Equal([]Result{{Username: "carol", Status: StatusCreated, GeneratedPassword: "generated-1"}}, results)
	s.Equal([]NewUser{{
		Username: "carol", Name: "carol", IsActive: true, Path: "users", Type: "internal", Groups: []string{},
	}}, s.fake.created)
	s.Equal(map[int]string{100: "generated-1"}, s.fake.passwords)
}

func (s *CreatorSuite) TestCreateAll_FullRow_UsesCSVPasswordAndGroups() {
	s.fake.groups["site-a-users"] = "existing-uuid"

	results := s.creator.CreateAll(context.Background(), []Row{{
		Username: "bob", Name: "Bob Lin", Email: "bob@example.com", Password: "Change-Me-123",
		Groups: []string{"site-a-users", "site-b-users"},
	}})

	s.Equal([]Result{{Username: "bob", Status: StatusCreated}}, results)
	s.Equal([]NewUser{{
		Username: "bob", Name: "Bob Lin", Email: "bob@example.com", IsActive: true,
		Path: "users", Type: "internal", Groups: []string{"existing-uuid", "uuid-site-b-users"},
	}}, s.fake.created)
	s.Equal(map[int]string{100: "Change-Me-123"}, s.fake.passwords)
	s.Equal(map[string]string{"site-a-users": "existing-uuid", "site-b-users": "uuid-site-b-users"}, s.fake.groups)
}

func (s *CreatorSuite) TestCreateAll_SameGroupTwice_LooksUpAndCreatesItOnce() {
	rows := []Row{
		{Username: "alice", Name: "alice", Groups: []string{"team"}},
		{Username: "bob", Name: "bob", Groups: []string{"team"}},
	}

	results := s.creator.CreateAll(context.Background(), rows)

	s.Equal(StatusCreated, results[0].Status)
	s.Equal(StatusCreated, results[1].Status)
	s.Equal([]string{
		"GET /api/v3/core/users/?username=alice",
		"GET /api/v3/core/groups/?name=team",
		"POST /api/v3/core/groups/",
		"POST /api/v3/core/users/",
		"POST /api/v3/core/users/100/set_password/",
		"GET /api/v3/core/users/?username=bob",
		"POST /api/v3/core/users/",
		"POST /api/v3/core/users/101/set_password/",
	}, s.fake.calls)
	s.Equal([]string{"uuid-team"}, s.fake.created[1].Groups)
}

func (s *CreatorSuite) TestCreateAll_ExistingUser_SkipsWithoutChanges() {
	s.fake.users["alice"] = 5

	results := s.creator.CreateAll(context.Background(), []Row{{Username: "alice", Name: "alice", Groups: []string{"team"}}})

	s.Equal([]Result{{Username: "alice", Status: StatusSkipped}}, results)
	s.Empty(s.fake.created)
	s.Empty(s.fake.passwords)
	s.Empty(s.fake.groups)
}

func (s *CreatorSuite) TestCreateAll_LookupUserFails_ReportsFailure() {
	s.fake.fail["GET /api/v3/core/users/"] = http.StatusForbidden

	results := s.creator.CreateAll(context.Background(), []Row{{Username: "alice", Name: "alice"}})

	s.Require().Len(results, 1)
	s.Equal(StatusFailed, results[0].Status)
	apiErr, ok := errors.AsType[*APIError](results[0].Err)
	s.Require().True(ok, "got %v", results[0].Err)
	s.Equal(http.StatusForbidden, apiErr.StatusCode)
	s.Equal("look up user: GET /core/users/: HTTP 403: forced failure", results[0].Err.Error())
	s.Empty(s.fake.created)
}

func (s *CreatorSuite) TestCreateAll_LookupGroupFails_ReportsFailure() {
	s.fake.fail["GET /api/v3/core/groups/"] = http.StatusInternalServerError

	results := s.creator.CreateAll(context.Background(), []Row{{Username: "alice", Name: "alice", Groups: []string{"team"}}})

	s.Equal(StatusFailed, results[0].Status)
	s.Equal(`look up group "team": GET /core/groups/: HTTP 500: forced failure`, results[0].Err.Error())
	s.Empty(s.fake.created)
}

func (s *CreatorSuite) TestCreateAll_CreateGroupFails_ReportsFailure() {
	s.fake.fail["POST /api/v3/core/groups/"] = http.StatusBadRequest

	results := s.creator.CreateAll(context.Background(), []Row{{Username: "alice", Name: "alice", Groups: []string{"team"}}})

	s.Equal(StatusFailed, results[0].Status)
	apiErr, ok := errors.AsType[*APIError](results[0].Err)
	s.Require().True(ok, "got %v", results[0].Err)
	s.Equal("/core/groups/", apiErr.Path)
	s.Equal(`create group "team": POST /core/groups/: HTTP 400: forced failure`, results[0].Err.Error())
	s.Empty(s.fake.created)
}

func (s *CreatorSuite) TestCreateAll_CreateUserFails_ReportsFailureAndContinues() {
	s.fake.fail["POST /api/v3/core/users/"] = http.StatusBadRequest

	results := s.creator.CreateAll(context.Background(), []Row{
		{Username: "alice", Name: "alice"},
		{Username: "bob", Name: "bob"},
	})

	s.Require().Len(results, 2)
	for i, name := range []string{"alice", "bob"} {
		s.Equal(name, results[i].Username)
		s.Equal(StatusFailed, results[i].Status)
		s.Equal("", results[i].GeneratedPassword) // never usable, so not shown
		s.Equal("create user: POST /core/users/: HTTP 400: forced failure", results[i].Err.Error())
	}
}

func (s *CreatorSuite) TestCreateAll_SetPasswordFails_ReportsErrPasswordNotSet() {
	s.fake.fail["POST /api/v3/core/users/100/set_password/"] = http.StatusBadRequest

	results := s.creator.CreateAll(context.Background(), []Row{{Username: "alice", Name: "alice"}})

	s.Equal(StatusFailed, results[0].Status)
	s.Equal("", results[0].GeneratedPassword)
	s.True(errors.Is(results[0].Err, ErrPasswordNotSet), "got %v", results[0].Err)
	apiErr, ok := errors.AsType[*APIError](results[0].Err)
	s.Require().True(ok)
	s.Equal(http.StatusBadRequest, apiErr.StatusCode)
	s.Len(s.fake.created, 1) // the user exists, without a password
}

func (s *CreatorSuite) TestPrintResults_EveryStatus_PrintsTableAndCountsFailures() {
	var out bytes.Buffer

	failed := PrintResults(&out, []Result{
		{Username: "alice", Status: StatusCreated, GeneratedPassword: "GENPW"},
		{Username: "bob", Status: StatusCreated},
		{Username: "carol", Status: StatusSkipped},
		{Username: "dave", Status: StatusFailed, Err: errors.New("boom")},
		{Username: "erin", Status: StatusFailed, Err: errors.New("bang")},
	})

	s.Equal(2, failed)
	s.Equal(
		"USERNAME  STATUS   PASSWORD    NOTE\n"+
			"alice     created  GENPW       \n"+
			"bob       created  (from CSV)  \n"+
			"carol     skipped  -           already exists\n"+
			"dave      failed   -           boom\n"+
			"erin      failed   -           bang\n",
		out.String())
}

func (s *CreatorSuite) TestPrintResults_NoFailures_ReturnsZero() {
	var out bytes.Buffer

	failed := PrintResults(&out, []Result{{Username: "alice", Status: StatusSkipped}})

	s.Equal(0, failed)
}
