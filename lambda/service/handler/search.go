package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SearchGuides queries readme.io's v2 search endpoint scoped to the
// `guides` section (so API reference content is excluded server-side).
//
// readme.io exposes two API versions today:
//
//   - v1: https://dash.readme.com/api/v1 — used by GetDocument for legacy
//     get-by-slug. Basic auth (key as username, empty password).
//   - v2: https://api.readme.com/v2 — what we use here. Bearer auth, and
//     critically, a `section=guides` query parameter that filters out the
//     auto-generated API reference docs without any post-processing.
//
// readme.io project API keys work with BOTH v1 and v2 — only the
// authorization scheme differs. Same `README_API_KEY` env var serves both
// handlers. (Confirmed against readme.io's API upgrade guide.)
const (
	defaultSearchLimit = 5
	maxSearchLimit     = 25
)

var readmeSearchUrl = "https://api.readme.com/v2/search"

func SearchGuides(ctx context.Context, apiKey, query string, limit int) *ReadmeResponse {
	params := url.Values{}
	params.Set("query", query)
	params.Set("section", "guides")
	params.Set("limit", fmt.Sprintf("%d", limit))

	target := fmt.Sprintf("%s?%s", readmeSearchUrl, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error creating request to %s: %v", target, err)
	}
	req.Header.Set("accept", applicationJson)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error while calling %s %s: %v", http.MethodGet, target, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("error closing response body", "url", target, "error", err)
		}
	}()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error reading body of response from %s %s: %v", http.MethodGet, target, err)
	}
	return &ReadmeResponse{
		Body:   string(respBytes),
		Status: resp.StatusCode,
	}
}
