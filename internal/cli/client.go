package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/billcobbler/golinks/internal/models"
)

// Client is an HTTP client for the golinks REST API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewClient creates a new API client from the given config.
func NewClient(cfg *Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.Server, "/"),
		token:   cfg.Token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}

func decodeJSON[T any](resp *http.Response) (T, error) {
	defer func() { _ = resp.Body.Close() }()
	var zero T
	if resp.StatusCode >= 400 {
		var e struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return zero, fmt.Errorf("%s", e.Error)
		}
		return zero, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return zero, fmt.Errorf("decoding response: %w", err)
	}
	return v, nil
}

// ListLinks queries the server for links with optional search and pagination.
func (c *Client) ListLinks(search string, offset, limit int) (*models.ListResult, error) {
	p := url.Values{}
	if search != "" {
		p.Set("q", search)
	}
	if offset > 0 {
		p.Set("offset", fmt.Sprint(offset))
	}
	if limit > 0 {
		p.Set("limit", fmt.Sprint(limit))
	}
	path := "/-/api/links"
	if len(p) > 0 {
		path += "?" + p.Encode()
	}
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[*models.ListResult](resp)
}

// GetLink retrieves a single link by shortname.
func (c *Client) GetLink(shortname string) (*models.Link, error) {
	resp, err := c.do("GET", "/-/api/links/"+shortname, nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[*models.Link](resp)
}

// CreateLinkInput is the payload for creating a new link.
type CreateLinkInput struct {
	Shortname   string `json:"shortname"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description,omitempty"`
	IsPattern   bool   `json:"is_pattern,omitempty"`
}

// CreateLink creates a new golink.
func (c *Client) CreateLink(input CreateLinkInput) (*models.Link, error) {
	resp, err := c.do("POST", "/-/api/links", input)
	if err != nil {
		return nil, err
	}
	return decodeJSON[*models.Link](resp)
}

// UpdateLinkInput is the payload for updating an existing link.
// All fields are replaced (not partial), so populate from the existing link first.
type UpdateLinkInput struct {
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
	IsPattern   bool   `json:"is_pattern"`
}

// UpdateLink updates an existing golink.
func (c *Client) UpdateLink(shortname string, input UpdateLinkInput) (*models.Link, error) {
	resp, err := c.do("PUT", "/-/api/links/"+shortname, input)
	if err != nil {
		return nil, err
	}
	return decodeJSON[*models.Link](resp)
}

// DeleteLink deletes a golink by shortname.
func (c *Client) DeleteLink(shortname string) error {
	resp, err := c.do("DELETE", "/-/api/links/"+shortname, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		var e struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return fmt.Errorf("%s", e.Error)
		}
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetStats returns aggregate usage statistics.
func (c *Client) GetStats() (*models.Stats, error) {
	resp, err := c.do("GET", "/-/api/stats", nil)
	if err != nil {
		return nil, err
	}
	return decodeJSON[*models.Stats](resp)
}

// Export downloads all links in the given format ("json" or "csv").
func (c *Client) Export(format string) ([]byte, error) {
	path := "/-/api/export"
	if format != "" {
		path += "?format=" + url.QueryEscape(format)
	}
	resp, err := c.do("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		var e struct{ Error string }
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return nil, fmt.Errorf("%s", e.Error)
		}
		return nil, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Import sends link data to the server. contentType must be "application/json" or "text/csv".
func (c *Client) Import(data []byte, contentType string, overwrite bool) (string, error) {
	path := "/-/api/import"
	if overwrite {
		path += "?overwrite=true"
	}
	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var result struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		if result.Error != "" {
			return "", fmt.Errorf("%s", result.Error)
		}
		return "", fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	return result.Message, nil
}
