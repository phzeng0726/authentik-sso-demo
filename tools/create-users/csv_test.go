package main

import (
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type CSVSuite struct {
	suite.Suite
}

func TestCSVSuite(t *testing.T) {
	suite.Run(t, new(CSVSuite))
}

func (s *CSVSuite) TestParseUsers_UsernameOnly_DefaultsNameToUsername() {
	rows, err := ParseUsers(strings.NewReader("username\nalice\n"))

	s.Require().NoError(err)
	s.Equal([]Row{{Username: "alice", Name: "alice"}}, rows)
}

func (s *CSVSuite) TestParseUsers_AllColumns_ParsesEveryField() {
	in := "username,name,email,password,groups\n" +
		" alice , Alice Chen , alice@example.com , p@ss word ,site-a; site-b;;\n" +
		"bob,,,,\n"

	rows, err := ParseUsers(strings.NewReader(in))

	s.Require().NoError(err)
	s.Equal([]Row{
		{
			Username: "alice",
			Name:     "Alice Chen",
			Email:    "alice@example.com",
			Password: " p@ss word ", // kept as written
			Groups:   []string{"site-a", "site-b"},
		},
		{Username: "bob", Name: "bob"},
	}, rows)
}

func (s *CSVSuite) TestParseUsers_HeaderWithBOMAndMixedCase_IsRecognized() {
	rows, err := ParseUsers(strings.NewReader(byteOrderMark + " Username ,EMAIL\nalice,a@example.com\n"))

	s.Require().NoError(err)
	s.Equal([]Row{{Username: "alice", Name: "alice", Email: "a@example.com"}}, rows)
}

func (s *CSVSuite) TestParseUsers_HeaderOnly_ReturnsNoRows() {
	rows, err := ParseUsers(strings.NewReader("username,name\n"))

	s.Require().NoError(err)
	s.Empty(rows)
}

func (s *CSVSuite) TestParseUsers_EmptyInput_ReturnsErrEmptyCSV() {
	_, err := ParseUsers(strings.NewReader(""))

	s.True(errors.Is(err, ErrEmptyCSV), "got %v", err)
}

func (s *CSVSuite) TestParseUsers_NoUsernameColumn_ReturnsErrMissingColumn() {
	_, err := ParseUsers(strings.NewReader("name,email\nAlice,a@example.com\n"))

	s.True(errors.Is(err, ErrMissingColumn), "got %v", err)
	s.Equal("missing required column: username", err.Error())
}

func (s *CSVSuite) TestParseUsers_BlankUsername_ReturnsErrMissingUsernameWithLine() {
	_, err := ParseUsers(strings.NewReader("username,name\nalice,Alice\n  ,Nobody\n"))

	s.True(errors.Is(err, ErrMissingUsername), "got %v", err)
	s.Equal("line 3: username is required", err.Error())
}

func (s *CSVSuite) TestParseUsers_DuplicateUsername_ReturnsErrDuplicateUsername() {
	_, err := ParseUsers(strings.NewReader("username\nalice\nbob\nalice\n"))

	s.True(errors.Is(err, ErrDuplicateUsername), "got %v", err)
	s.Equal(`line 4: duplicate username "alice", first seen on line 2`, err.Error())
}

func (s *CSVSuite) TestParseUsers_WrongFieldCount_ReturnsCSVError() {
	_, err := ParseUsers(strings.NewReader("username,name\nalice,Alice,extra\n"))

	s.True(errors.Is(err, csv.ErrFieldCount), "got %v", err)
}

func (s *CSVSuite) TestParseUsers_MalformedHeader_ReturnsCSVError() {
	_, err := ParseUsers(strings.NewReader("\"username\n"))

	s.True(errors.Is(err, csv.ErrQuote), "got %v", err)
}
