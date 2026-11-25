package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// APIError is a non-2xx response from the authentik API.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// User is the part of an authentik user this tool reads.
type User struct {
	PK       int    `json:"pk"`
	Username string `json:"username"`
}

// Group is the part of an authentik group this tool reads.
type Group struct {
	PK   string `json:"pk"` // UUID
	Name string `json:"name"`
}

// NewUser is the request body of POST /core/users/.
type NewUser struct {
	Username string   `json:"username"`
	Name     string   `json:"name"`
	Email    string   `json:"email,omitempty"`
	IsActive bool     `json:"is_active"`
	Path     string   `json:"path"`
	Type     string   `json:"type"`
	Groups   []string `json:"groups"` // group UUIDs
}

type page[T any] struct {
	Results []T `json:"results"`
}

// Client calls the authentik REST API (/api/v3) with an API token.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient returns a client for the authentik instance at baseURL,
// e.g. http://localhost:9000.
func NewClient(baseURL, token string, hc *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/") + "/api/v3",
		token:   token,
		http:    hc,
	}
}

// FindUserByUsername reports whether a user with exactly this username exists.
func (c *Client) FindUserByUsername(ctx context.Context, username string) (User, bool, error) {
	var p page[User]
	if err := c.do(ctx, http.MethodGet, "/core/users/", url.Values{"username": {username}}, nil, &p); err != nil {
		return User{}, false, err
	}
	for _, u := range p.Results {
		if u.Username == username {
			return u, true, nil
		}
	}
	return User{}, false, nil
}

// FindGroupByName reports whether a group with exactly this name exists.
func (c *Client) FindGroupByName(ctx context.Context, name string) (Group, bool, error) {
	var p page[Group]
	if err := c.do(ctx, http.MethodGet, "/core/groups/", url.Values{"name": {name}}, nil, &p); err != nil {
		return Group{}, false, err
	}
	for _, g := range p.Results {
		if g.Name == name {
			return g, true, nil
		}
	}
	return Group{}, false, nil
}

// CreateGroup creates a group and returns it.
func (c *Client) CreateGroup(ctx context.Context, name string) (Group, error) {
	var g Group
	err := c.do(ctx, http.MethodPost, "/core/groups/", nil, map[string]string{"name": name}, &g)
	return g, err
}

// CreateUser creates a user (without a password) and returns it.
func (c *Client) CreateUser(ctx context.Context, u NewUser) (User, error) {
	if u.Groups == nil {
		u.Groups = []string{} // authentik rejects null
	}
	var created User
	err := c.do(ctx, http.MethodPost, "/core/users/", nil, u, &created)
	return created, err
}

// SetPassword sets the password of the user with primary key pk.
func (c *Client) SetPassword(ctx context.Context, pk int, password string) error {
	path := fmt.Sprintf("/core/users/%d/set_password/", pk)
	return c.do(ctx, http.MethodPost, path, nil, map[string]string{"password": password}, nil)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode %s %s request: %w", method, path, err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// net/http never returns 1xx as a final response, so only check the top.
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &APIError{Method: method, Path: path, StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}
