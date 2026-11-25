package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MainSuite struct {
	suite.Suite
	fake   *fakeAuthentik
	server *httptest.Server
	dir    string
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func TestMainSuite(t *testing.T) {
	suite.Run(t, new(MainSuite))
}

func (s *MainSuite) SetupTest() {
	s.fake = newFakeAuthentik()
	s.server = httptest.NewServer(s.fake.handler())
	s.dir = s.T().TempDir()
	s.stdout.Reset()
	s.stderr.Reset()
	s.T().Setenv("AUTHENTIK_TOKEN", "tok")
	s.T().Setenv("AUTHENTIK_URL", s.server.URL)
}

func (s *MainSuite) TearDownTest() {
	s.server.Close()
}

// writeCSV writes content to name in the test's temp dir and returns its path.
func (s *MainSuite) writeCSV(name, content string) string {
	path := filepath.Join(s.dir, name)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o600))
	return path
}

func (s *MainSuite) run(args ...string) int {
	return run(args, &s.stdout, &s.stderr)
}

func (s *MainSuite) TestRun_AllRowsCreated_PrintsTableAndExits0() {
	path := s.writeCSV("users.csv", "username,password\nalice,pw1\nbob,pw2\n")

	code := s.run("-csv", path)

	s.Equal(0, code)
	s.Equal(
		"USERNAME  STATUS   PASSWORD    NOTE\n"+
			"alice     created  (from CSV)  \n"+
			"bob       created  (from CSV)  \n",
		s.stdout.String())
	s.Equal("", s.stderr.String())
	s.Equal(map[int]string{100: "pw1", 101: "pw2"}, s.fake.passwords)
}

func (s *MainSuite) TestRun_OnlySkippedRows_Exits0() {
	s.fake.users["alice"] = 5
	path := s.writeCSV("users.csv", "username\nalice\n")

	code := s.run("-csv", path)

	s.Equal(0, code)
	s.Equal(
		"USERNAME  STATUS   PASSWORD  NOTE\n"+
			"alice     skipped  -         already exists\n",
		s.stdout.String())
}

func (s *MainSuite) TestRun_OneRowFails_ContinuesAndExits1() {
	s.fake.fail["POST /api/v3/core/users/100/set_password/"] = http.StatusBadRequest
	path := s.writeCSV("users.csv", "username,password\nalice,pw1\nbob,pw2\n")

	code := s.run("-csv", path)

	s.Equal(1, code)
	s.Equal(
		"USERNAME  STATUS   PASSWORD    NOTE\n"+
			"alice     failed   -           user created but password not set: POST /core/users/100/set_password/: HTTP 400: forced failure\n"+
			"bob       created  (from CSV)  \n",
		s.stdout.String())
	s.Equal(map[int]string{101: "pw2"}, s.fake.passwords)
}

func (s *MainSuite) TestRun_NoCSVFlag_ReadsUsersCSVInWorkingDir() {
	s.writeCSV("users.csv", "username,password\nalice,pw1\n")
	s.T().Chdir(s.dir)

	code := s.run()

	s.Equal(0, code)
	s.Equal([]string{"alice"}, createdUsernames(s.fake))
}

func (s *MainSuite) TestRun_URLFlag_OverridesAuthentikURL() {
	s.T().Setenv("AUTHENTIK_URL", "http://127.0.0.1:1") // nothing listens here
	path := s.writeCSV("users.csv", "username,password\nalice,pw1\n")

	code := s.run("-csv", path, "-url", s.server.URL)

	s.Equal(0, code)
	s.Equal([]string{"alice"}, createdUsernames(s.fake))
}

func (s *MainSuite) TestRun_TokenMissing_Exits2WithoutCallingAPI() {
	s.T().Setenv("AUTHENTIK_TOKEN", "")
	path := s.writeCSV("users.csv", "username\nalice\n")

	code := s.run("-csv", path)

	s.Equal(2, code)
	s.Equal("AUTHENTIK_TOKEN is not set; put it in .env or the environment\n", s.stderr.String())
	s.Equal("", s.stdout.String())
	s.Empty(s.fake.calls)
}

func (s *MainSuite) TestRun_CSVFileMissing_Exits2() {
	path := filepath.Join(s.dir, "missing.csv")

	code := s.run("-csv", path)

	s.Equal(2, code)
	s.True(strings.HasPrefix(s.stderr.String(), "open "+path+": "), "got %q", s.stderr.String())
	s.Empty(s.fake.calls)
}

func (s *MainSuite) TestRun_InvalidCSV_Exits2WithoutCallingAPI() {
	path := s.writeCSV("users.csv", "name\nAlice\n")

	code := s.run("-csv", path)

	s.Equal(2, code)
	s.Equal(path+": missing required column: username\n", s.stderr.String())
	s.Empty(s.fake.calls)
}

func (s *MainSuite) TestRun_UnknownFlag_Exits2() {
	code := s.run("-nope")

	s.Equal(2, code)
	s.True(strings.HasPrefix(s.stderr.String(), "flag provided but not defined: -nope\n"), "got %q", s.stderr.String())
	s.Empty(s.fake.calls)
}

func (s *MainSuite) TestEnvOr_Set_ReturnsValue() {
	s.T().Setenv("CREATE_USERS_TEST_VAR", "value")

	s.Equal("value", envOr("CREATE_USERS_TEST_VAR", "fallback"))
}

func (s *MainSuite) TestEnvOr_Empty_ReturnsFallback() {
	s.T().Setenv("CREATE_USERS_TEST_VAR", "")

	s.Equal("fallback", envOr("CREATE_USERS_TEST_VAR", "fallback"))
}

func (s *MainSuite) TestEnvOr_Unset_ReturnsFallback() {
	s.T().Setenv("CREATE_USERS_TEST_VAR", "x") // restores the variable after the test
	s.Require().NoError(os.Unsetenv("CREATE_USERS_TEST_VAR"))

	s.Equal("fallback", envOr("CREATE_USERS_TEST_VAR", "fallback"))
}

func createdUsernames(f *fakeAuthentik) []string {
	names := make([]string, 0, len(f.created))
	for _, u := range f.created {
		names = append(names, u.Username)
	}
	return names
}
