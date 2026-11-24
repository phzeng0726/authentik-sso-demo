package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrEmptyCSV          = errors.New("CSV file is empty")
	ErrMissingColumn     = errors.New("missing required column")
	ErrMissingUsername   = errors.New("username is required")
	ErrDuplicateUsername = errors.New("duplicate username")
)

// byteOrderMark is the UTF-8 BOM that Excel puts at the start of a CSV file.
const byteOrderMark = "\xEF\xBB\xBF"

// Row is one user to create, read from the CSV file.
type Row struct {
	Username string
	Name     string // defaults to Username
	Email    string
	Password string // empty means generate one
	Groups   []string
}

// ParseUsers reads a CSV file whose header names its columns. Only "username"
// is required; "name", "email", "password" and "groups" (names separated by
// ";") are optional. Header names are case-insensitive, and a UTF-8 BOM (as
// saved by Excel) is ignored.
func ParseUsers(r io.Reader) ([]Row, error) {
	cr := csv.NewReader(r)

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, ErrEmptyCSV
	}
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	cols := make(map[string]int, len(header))
	for i, h := range header {
		if i == 0 {
			h = strings.TrimPrefix(h, byteOrderMark)
		}
		cols[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := cols["username"]; !ok {
		return nil, fmt.Errorf("%w: username", ErrMissingColumn)
	}

	// raw returns the field as written; passwords may start or end with spaces.
	raw := func(rec []string, col string) string {
		i, ok := cols[col]
		if !ok {
			return ""
		}
		return rec[i]
	}
	field := func(rec []string, col string) string {
		return strings.TrimSpace(raw(rec, col))
	}

	var rows []Row
	firstLine := map[string]int{}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		line, _ := cr.FieldPos(0)

		row := Row{
			Username: field(rec, "username"),
			Name:     field(rec, "name"),
			Email:    field(rec, "email"),
			Password: raw(rec, "password"),
			Groups:   splitGroups(field(rec, "groups")),
		}
		if row.Username == "" {
			return nil, fmt.Errorf("line %d: %w", line, ErrMissingUsername)
		}
		if first, dup := firstLine[row.Username]; dup {
			return nil, fmt.Errorf("line %d: %w %q, first seen on line %d",
				line, ErrDuplicateUsername, row.Username, first)
		}
		firstLine[row.Username] = line
		if row.Name == "" {
			row.Name = row.Username
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// splitGroups splits "a; b;;c" into [a b c].
func splitGroups(s string) []string {
	var groups []string
	for g := range strings.SplitSeq(s, ";") {
		if g = strings.TrimSpace(g); g != "" {
			groups = append(groups, g)
		}
	}
	return groups
}
