package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
)

// ErrPasswordNotSet means the user was created but has no usable password.
var ErrPasswordNotSet = errors.New("user created but password not set")

type Status string

const (
	StatusCreated Status = "created"
	StatusSkipped Status = "skipped" // username already exists; left untouched
	StatusFailed  Status = "failed"
)

// Result is the outcome of one CSV row.
type Result struct {
	Username          string
	Status            Status
	GeneratedPassword string // set only when created with a generated password
	Err               error
}

func (r Result) fail(err error) Result {
	r.Status = StatusFailed
	r.GeneratedPassword = ""
	r.Err = err
	return r
}

// Creator creates users one by one, creating missing groups on the way.
type Creator struct {
	client      *Client
	newPassword func() string
	groupIDs    map[string]string // group name -> UUID
}

func NewCreator(client *Client, newPassword func() string) *Creator {
	return &Creator{client: client, newPassword: newPassword, groupIDs: map[string]string{}}
}

// CreateAll processes every row; a failed row does not stop the others.
func (c *Creator) CreateAll(ctx context.Context, rows []Row) []Result {
	results := make([]Result, 0, len(rows))
	for _, row := range rows {
		results = append(results, c.create(ctx, row))
	}
	return results
}

func (c *Creator) create(ctx context.Context, row Row) Result {
	res := Result{Username: row.Username}

	_, exists, err := c.client.FindUserByUsername(ctx, row.Username)
	if err != nil {
		return res.fail(fmt.Errorf("look up user: %w", err))
	}
	if exists {
		res.Status = StatusSkipped
		return res
	}

	groupIDs, err := c.resolveGroups(ctx, row.Groups)
	if err != nil {
		return res.fail(err)
	}

	password := row.Password
	if password == "" {
		password = c.newPassword()
		res.GeneratedPassword = password
	}

	user, err := c.client.CreateUser(ctx, NewUser{
		Username: row.Username,
		Name:     row.Name,
		Email:    row.Email,
		IsActive: true,
		Path:     "users",
		Type:     "internal",
		Groups:   groupIDs,
	})
	if err != nil {
		return res.fail(fmt.Errorf("create user: %w", err))
	}

	if err := c.client.SetPassword(ctx, user.PK, password); err != nil {
		return res.fail(fmt.Errorf("%w: %w", ErrPasswordNotSet, err))
	}

	res.Status = StatusCreated
	return res
}

// resolveGroups returns the UUIDs of the named groups, creating missing ones.
func (c *Creator) resolveGroups(ctx context.Context, names []string) ([]string, error) {
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, ok := c.groupIDs[name]
		if !ok {
			g, found, err := c.client.FindGroupByName(ctx, name)
			if err != nil {
				return nil, fmt.Errorf("look up group %q: %w", name, err)
			}
			if !found {
				if g, err = c.client.CreateGroup(ctx, name); err != nil {
					return nil, fmt.Errorf("create group %q: %w", name, err)
				}
			}
			id = g.PK
			c.groupIDs[name] = id
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// PrintResults writes a table of results and returns the number of failures.
func PrintResults(w io.Writer, results []Result) int {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "USERNAME\tSTATUS\tPASSWORD\tNOTE")
	failed := 0
	for _, r := range results {
		password, note := "-", ""
		switch r.Status {
		case StatusCreated:
			password = "(from CSV)"
			if r.GeneratedPassword != "" {
				password = r.GeneratedPassword
			}
		case StatusSkipped:
			note = "already exists"
		case StatusFailed:
			failed++
			note = r.Err.Error()
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Username, r.Status, password, note)
	}
	tw.Flush()
	return failed
}
