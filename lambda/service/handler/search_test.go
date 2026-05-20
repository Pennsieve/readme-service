package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchGuides(t *testing.T) {
	expectedStatus := http.StatusOK
	expectedResponseBody := `{"data": [{"title": "Uploading data", "slug": "uploading-data", "section": "guides"}]}`

	expectedApiKey := "test-readme-api-key"
	expectedQuery := "uploading"
	expectedLimit := 3

	testReadmeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer "+expectedApiKey, r.Header.Get("Authorization"))
		// Verify the section filter is being sent — that's the headline reason
		// we're on v2 of the readme.io API rather than v1.
		assert.Equal(t, expectedQuery, r.URL.Query().Get("query"))
		assert.Equal(t, "guides", r.URL.Query().Get("section"))
		assert.Equal(t, fmt.Sprintf("%d", expectedLimit), r.URL.Query().Get("limit"))
		w.WriteHeader(expectedStatus)
		_, err := fmt.Fprint(w, expectedResponseBody)
		require.NoError(t, err)
	}))
	defer testReadmeServer.Close()

	defer func(u string) { readmeSearchUrl = u }(readmeSearchUrl)
	readmeSearchUrl = testReadmeServer.URL

	resp := SearchGuides(context.Background(), expectedApiKey, expectedQuery, expectedLimit)
	assert.Equal(t, expectedResponseBody, resp.Body)
	assert.Equal(t, expectedStatus, resp.Status)
}

func TestSearchGuides_PropagatesReadmeError(t *testing.T) {
	testReadmeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"message":"invalid api key"}`)
	}))
	defer testReadmeServer.Close()

	defer func(u string) { readmeSearchUrl = u }(readmeSearchUrl)
	readmeSearchUrl = testReadmeServer.URL

	resp := SearchGuides(context.Background(), "bad-key", "uploading", 5)
	// 401s from readme propagate up — caller decides how to handle.
	assert.Equal(t, http.StatusUnauthorized, resp.Status)
	assert.Contains(t, resp.Body, "invalid api key")
}
