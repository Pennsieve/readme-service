package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────
// v1 search — the default path against legacy ReadMe (Pennsieve today).
// ─────────────────────────────────────────────────────────────────────

// buildV1Hit assembles a v1SearchHit fixture in the actual readme.io
// shape — isReference bool + excerpt nested inside _snippetResult.
func buildV1Hit(title, slug, excerpt string, isReference bool) v1SearchHit {
	h := v1SearchHit{Title: title, Slug: slug, IsReference: isReference}
	h.SnippetResult.Excerpt.Value = excerpt
	return h
}

func TestSearchGuides_V1_DefaultsAndFiltersReferenceHits(t *testing.T) {
	// Fake readme.io v1 search endpoint. Verifies our request shape and
	// returns a mixed list of guide + reference hits to confirm
	// client-side filtering on isReference.
	envelope := v1SearchEnvelope{
		Results: []v1SearchHit{
			buildV1Hit("Uploading data", "uploading-data", "How to upload files", false),
			buildV1Hit("POST /upload (API)", "post-upload", "API endpoint", true),
			buildV1Hit("Bulk upload", "bulk-upload", "Large dataset uploads", false),
			buildV1Hit("Pennsieve agent", "installing-the-pennsieve-agent", "CLI tool for uploads", false),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "upload", r.URL.Query().Get("search"))
		// v1 uses Basic auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-key", user)
		assert.Empty(t, pass)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(envelope)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrlV1 = u }(readmeSearchUrlV1)
	readmeSearchUrlV1 = srv.URL

	// Make sure no v2 override is set in env
	t.Setenv(searchAPIVersionEnvVar, "")

	resp := SearchGuides(context.Background(), "test-key", "upload", 10)
	require.Equal(t, http.StatusOK, resp.Status)

	var got normalizedResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))

	assert.Equal(t, searchAPIVersionV1, got.Version)
	assert.Equal(t, 3, got.Returned)
	// isReference=true was filtered; the three guide hits remain
	assert.Len(t, got.Data, 3)
	for _, hit := range got.Data {
		assert.NotEqual(t, "post-upload", hit.Slug, "API reference hit leaked into results")
		assert.Equal(t, docsPublicUrlPrefix+hit.Slug, hit.URL)
	}
	// Excerpt comes through from _snippetResult.excerpt.value
	assert.Equal(t, "How to upload files", got.Data[0].Excerpt)
}

func TestSearchGuides_V1_PropagatesReadmeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"message":"invalid api key"}`)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrlV1 = u }(readmeSearchUrlV1)
	readmeSearchUrlV1 = srv.URL

	t.Setenv(searchAPIVersionEnvVar, "")
	resp := SearchGuides(context.Background(), "bad-key", "upload", 5)
	assert.Equal(t, http.StatusUnauthorized, resp.Status)
	assert.Contains(t, resp.Body, "invalid api key")
}

func TestSearchGuides_V1_RespectsLimit(t *testing.T) {
	envelope := v1SearchEnvelope{Results: []v1SearchHit{
		buildV1Hit("A", "a", "", false),
		buildV1Hit("B", "b", "", false),
		buildV1Hit("C", "c", "", false),
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(envelope)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrlV1 = u }(readmeSearchUrlV1)
	readmeSearchUrlV1 = srv.URL

	t.Setenv(searchAPIVersionEnvVar, "")
	resp := SearchGuides(context.Background(), "test-key", "q", 2)

	var got normalizedResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))
	assert.Equal(t, 2, got.Returned)
}

// ─────────────────────────────────────────────────────────────────────
// v2 search — opt-in via README_SEARCH_API_VERSION=v2.
// ─────────────────────────────────────────────────────────────────────

func TestSearchGuides_V2_OptIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		// v2 uses `query`, `section`, native server-side guides filter
		assert.Equal(t, "upload", r.URL.Query().Get("query"))
		assert.Equal(t, "guides", r.URL.Query().Get("section"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"total":2,"data":[
			{"title":"Uploading data","slug":"uploading-data","excerpt":"How to upload","category":{"slug":"getting-started"}},
			{"title":"Bulk upload","slug":"bulk-upload","excerpt":"Large datasets","category":{"slug":"tutorials"}}
		]}`)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrl = u }(readmeSearchUrl)
	readmeSearchUrl = srv.URL

	t.Setenv(searchAPIVersionEnvVar, "v2")
	resp := SearchGuides(context.Background(), "test-key", "upload", 5)
	require.Equal(t, http.StatusOK, resp.Status)

	var got normalizedResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))

	assert.Equal(t, searchAPIVersionV2, got.Version)
	assert.Equal(t, 2, got.Total)
	assert.Equal(t, 2, got.Returned)
	assert.Equal(t, "uploading-data", got.Data[0].Slug)
	assert.Equal(t, docsPublicUrlPrefix+"uploading-data", got.Data[0].URL)
	assert.Equal(t, "getting-started", got.Data[0].Category)
}

func TestSearchGuides_DefaultsToV1WhenEnvUnknown(t *testing.T) {
	// readme.io v1 endpoint receives the call regardless of env junk
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `[]`)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrlV1 = u }(readmeSearchUrlV1)
	readmeSearchUrlV1 = srv.URL

	// nonsense env value should fall back to v1
	t.Setenv(searchAPIVersionEnvVar, "v3-beta")
	_ = SearchGuides(context.Background(), "test-key", "q", 5)
	assert.True(t, hit, "v1 endpoint should have been called for unknown env value")
}

func TestSearchGuides_V1_HandlesEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"results":[]}`)
	}))
	defer srv.Close()
	defer func(u string) { readmeSearchUrlV1 = u }(readmeSearchUrlV1)
	readmeSearchUrlV1 = srv.URL

	t.Setenv(searchAPIVersionEnvVar, "")
	resp := SearchGuides(context.Background(), "test-key", "nomatch", 5)
	require.Equal(t, http.StatusOK, resp.Status)

	var got normalizedResponse
	require.NoError(t, json.Unmarshal([]byte(resp.Body), &got))
	assert.Equal(t, 0, got.Returned)
	assert.Equal(t, searchAPIVersionV1, got.Version)
}

// Ensure tests don't leak env state into the rest of the suite.
func TestMain(m *testing.M) {
	_ = os.Unsetenv(searchAPIVersionEnvVar)
	code := m.Run()
	os.Exit(code)
}
