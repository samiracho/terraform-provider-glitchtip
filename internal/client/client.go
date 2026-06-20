// Copyright (c) samiracho
// SPDX-License-Identifier: MIT

// Package client is a thin HTTP client for the GlitchTip API
// (https://app.glitchtip.com/api/docs). It authenticates with a bearer token
// and exposes a single generic Do method that resources use to perform typed
// JSON requests.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultBaseURL is the hosted GlitchTip instance, used when no endpoint is configured.
const DefaultBaseURL = "https://app.glitchtip.com"

// Client is a GlitchTip API client.
type Client struct {
	baseURL    string
	token      string
	userAgent  string
	httpClient *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient sets the underlying *http.Client (useful for tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// New builds a Client. If baseURL is empty, DefaultBaseURL is used. A trailing
// slash on baseURL is trimmed so callers can pass absolute API paths beginning
// with "/api/0/...".
func New(baseURL, token string, opts ...Option) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// APIError is returned for any non-2xx HTTP response.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("glitchtip API %s %s returned status %d: %s", e.Method, e.Path, e.StatusCode, e.Body)
}

// IsNotFound reports whether err is (or wraps) an APIError with a 404 status.
// Resources use this in Read to detect out-of-band deletion and drop the
// resource from state.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// Do performs an HTTP request against the GlitchTip API.
//
//   - method is the HTTP verb (http.MethodGet, http.MethodPost, ...).
//   - path is the request path relative to the configured base URL and must
//     begin with a slash, e.g. "/api/0/organizations/". Callers are responsible
//     for URL-escaping any path segments.
//   - body, if non-nil, is JSON-encoded and sent as the request body.
//   - out, if non-nil, receives the JSON-decoded response body.
//
// A non-2xx response yields an *APIError (see IsNotFound).
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	_, err := c.doWithResponse(ctx, method, path, body, out)
	return err
}

// doWithResponse is Do plus the response headers, which List uses to follow
// GlitchTip's cursor-pagination Link header.
func (c *Client) doWithResponse(ctx context.Context, method, path string, body, out any) (http.Header, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.Header, &APIError{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Body:       strings.TrimSpace(string(respBody)),
		}
	}

	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.Header, fmt.Errorf("decoding response body: %w", err)
		}
	}
	return resp.Header, nil
}

// nextLinkRe matches the `rel="next"` entry of GlitchTip's RFC 5988-style Link
// header, capturing the URL and the `results` flag. GlitchTip wraps the header
// value in {'...'} and always emits results="true"/"false"; following the next
// link only when results="true" is what terminates pagination.
var nextLinkRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next";\s*results="(true|false)"`)

// nextPagePath extracts the relative path+query of the next page from a Link
// header, or "" when there is no further page. The returned path is relative to
// the client base URL so it survives a GLITCHTIP_DOMAIN that differs from the
// configured endpoint.
func nextPagePath(linkHeader string) string {
	m := nextLinkRe.FindStringSubmatch(linkHeader)
	if m == nil || m[2] != "true" {
		return ""
	}
	u, err := url.Parse(m[1])
	if err != nil {
		return ""
	}
	rel := u.EscapedPath()
	if u.RawQuery != "" {
		rel += "?" + u.RawQuery
	}
	return rel
}

// List performs a paginated GET, following GlitchTip's cursor Link header until
// it is exhausted, and returns every item across all pages. path is the initial
// list path (e.g. "/api/0/organizations/acme/projects/"). It always returns a
// non-nil slice on success.
func List[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	all := []T{}
	next := path
	seen := map[string]bool{}
	for next != "" {
		if seen[next] {
			break // safety: never re-request the same page
		}
		seen[next] = true

		var page []T
		hdr, err := c.doWithResponse(ctx, http.MethodGet, next, nil, &page)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		next = nextPagePath(hdr.Get("Link"))
	}
	return all, nil
}
