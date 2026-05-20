package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// search.go — two implementations, one switch.
//
// readme.io has two generations of search:
//
//   - v1 (POST https://dash.readme.com/api/v1/docs/search) — works against
//     ALL projects, including legacy (non-Refactored) ones. Basic auth.
//     No native section filter; each hit includes a `type` field
//     ("guide" / "reference") that we use to drop API-reference hits
//     client-side.
//
//   - v2 (GET https://api.readme.com/v2/search) — Bearer auth, has a
//     `section=guides` parameter that filters server-side. ONLY works
//     against "ReadMe Refactored" projects — legacy projects return
//     empty for every query (confirmed against Pennsieve's project,
//     2026-05-19).
//
// Pennsieve's readme.io project is on the legacy platform today. So we
// default to v1. When the project migrates to Refactored, flip
// README_SEARCH_API_VERSION=v2 in terraform and redeploy — no code
// change needed.
//
// To keep pennsieve-mcp and any other downstream consumer agnostic to
// which API version was used, BOTH implementations normalize to a
// single response shape:
//
//   {
//     "total":    <int>,
//     "returned": <int>,
//     "version":  "v1" | "v2",
//     "data": [
//       {
//         "title":    "...",
//         "slug":     "...",
//         "excerpt":  "...",          // present when readme provides one
//         "category": "..." or null,  // category slug if known
//         "url":      "https://docs.pennsieve.io/docs/<slug>"
//       },
//       ...
//     ]
//   }

const (
	defaultSearchLimit = 5
	maxSearchLimit     = 25

	searchAPIVersionEnvVar = "README_SEARCH_API_VERSION"
	searchAPIVersionV1     = "v1"
	searchAPIVersionV2     = "v2"

	docsPublicUrlPrefix = "https://docs.pennsieve.io/docs/"
)

// Mutable for testability (httptest swaps these in unit tests).
var (
	readmeSearchUrlV1 = "https://dash.readme.com/api/v1/docs/search"
	readmeSearchUrl   = "https://api.readme.com/v2/search"
)

// SearchGuides dispatches to v1 or v2 based on README_SEARCH_API_VERSION.
// Empty / unset defaults to v1 (current Pennsieve project tier).
func SearchGuides(ctx context.Context, apiKey, query string, limit int) *ReadmeResponse {
	version := strings.ToLower(os.Getenv(searchAPIVersionEnvVar))
	switch version {
	case searchAPIVersionV2:
		return searchGuidesV2(ctx, apiKey, query, limit)
	case "", searchAPIVersionV1:
		return searchGuidesV1(ctx, apiKey, query, limit)
	default:
		logger.Warn("unknown README_SEARCH_API_VERSION — defaulting to v1", "value", version)
		return searchGuidesV1(ctx, apiKey, query, limit)
	}
}

// normalizedHit is the per-result shape we emit to downstream consumers,
// regardless of which readme.io API version produced it.
type normalizedHit struct {
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Excerpt  string `json:"excerpt,omitempty"`
	Category string `json:"category,omitempty"`
	URL      string `json:"url"`
}

type normalizedResponse struct {
	Total    int             `json:"total"`
	Returned int             `json:"returned"`
	Version  string          `json:"version"`
	Data     []normalizedHit `json:"data"`
}

// ──────────────────────────────────────────────────────────────────────
// v1 implementation — used today against the legacy Pennsieve project.
// ──────────────────────────────────────────────────────────────────────

// v1SearchEnvelope is the top-level shape of readme.io's v1 search
// response. The actual hits live inside `results` — it's NOT a bare
// array as the legacy docs imply.
type v1SearchEnvelope struct {
	Results []v1SearchHit `json:"results"`
}

// v1SearchHit is the (loosely-typed) projection of one v1 search result.
// Fields are best-effort: readme.io has evolved its v1 response over the
// years and a few fields can be absent. We decode what we use and treat
// missing fields as empty.
//
// Guide vs reference is determined by the `isReference` boolean (NOT a
// `type` field as the legacy docs suggest). `excerpt` lives nested
// inside `_snippetResult.excerpt.value`, not at the top level.
type v1SearchHit struct {
	Title         string `json:"title"`
	Slug          string `json:"slug"`
	IsReference   bool   `json:"isReference"`
	InternalLink  string `json:"internalLink,omitempty"` // e.g. "docs/getting-started" or "reference/upload"
	SnippetResult struct {
		Excerpt struct {
			Value string `json:"value"`
		} `json:"excerpt"`
	} `json:"_snippetResult"`
}

func searchGuidesV1(ctx context.Context, apiKey, query string, limit int) *ReadmeResponse {
	params := url.Values{}
	params.Set("search", query) // v1 uses `search`, NOT `query`

	target := fmt.Sprintf("%s?%s", readmeSearchUrlV1, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, nil)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error creating request to %s: %v", target, err)
	}
	req.Header.Set("accept", applicationJson)
	req.SetBasicAuth(apiKey, "") // v1 uses Basic auth, key as username

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error while calling %s %s: %v", http.MethodPost, target, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("error closing response body", "url", target, "error", err)
		}
	}()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error reading body of response from %s %s: %v", http.MethodPost, target, err)
	}

	// Non-2xx from readme: pass through the raw body + status so callers
	// see the original error.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ReadmeResponse{Body: string(respBytes), Status: resp.StatusCode}
	}

	// 200: decode, filter, normalize.
	var env v1SearchEnvelope
	if err := json.Unmarshal(respBytes, &env); err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "decoding v1 search response: %v", err)
	}

	out := normalizedResponse{Version: searchAPIVersionV1, Data: []normalizedHit{}}
	for _, h := range env.Results {
		// Filter out API reference content — keep only guides.
		// readme.io v1 marks reference docs via `isReference: true`.
		if h.IsReference {
			continue
		}
		out.Data = append(out.Data, normalizedHit{
			Title:   h.Title,
			Slug:    h.Slug,
			Excerpt: h.SnippetResult.Excerpt.Value,
			URL:     docsPublicUrlPrefix + h.Slug,
		})
		if len(out.Data) >= limit {
			break
		}
	}
	out.Total = len(out.Data) // v1 doesn't surface a separate total; report what we returned
	out.Returned = len(out.Data)

	body, err := json.Marshal(out)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "marshaling normalized response: %v", err)
	}
	return &ReadmeResponse{Body: string(body), Status: http.StatusOK}
}

// ──────────────────────────────────────────────────────────────────────
// v2 implementation — kept for when Pennsieve migrates to ReadMe Refactored.
// ──────────────────────────────────────────────────────────────────────

// v2SearchResponse matches readme.io's v2 search envelope.
type v2SearchResponse struct {
	Total int `json:"total"`
	Data  []struct {
		Title    string `json:"title"`
		Slug     string `json:"slug"`
		Excerpt  string `json:"excerpt,omitempty"`
		Category struct {
			Slug string `json:"slug"`
		} `json:"category,omitempty"`
	} `json:"data"`
}

func searchGuidesV2(ctx context.Context, apiKey, query string, limit int) *ReadmeResponse {
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &ReadmeResponse{Body: string(respBytes), Status: resp.StatusCode}
	}

	var v2 v2SearchResponse
	if err := json.Unmarshal(respBytes, &v2); err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "decoding v2 search response: %v", err)
	}

	out := normalizedResponse{Version: searchAPIVersionV2, Total: v2.Total, Data: []normalizedHit{}}
	for _, h := range v2.Data {
		out.Data = append(out.Data, normalizedHit{
			Title:    h.Title,
			Slug:     h.Slug,
			Excerpt:  h.Excerpt,
			Category: h.Category.Slug,
			URL:      docsPublicUrlPrefix + h.Slug,
		})
		if len(out.Data) >= limit {
			break
		}
	}
	out.Returned = len(out.Data)

	body, err := json.Marshal(out)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "marshaling normalized response: %v", err)
	}
	return &ReadmeResponse{Body: string(body), Status: http.StatusOK}
}
