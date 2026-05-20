package handler

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetDocument(t *testing.T) {
	expectedStatus := http.StatusOK
	expectedResponseBody := `{"key": "value"}`

	expectedApiKey := "test-readme-api-key"
	expectedSlug := "test-pennsieve-slug"

	testReadmeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, strings.TrimPrefix(r.URL.Path, "/"), expectedSlug)
		actualApiKey, actualPass, basicAuthSet := r.BasicAuth()
		assert.True(t, basicAuthSet)
		assert.Equal(t, expectedApiKey, actualApiKey)
		assert.Empty(t, actualPass)
		w.WriteHeader(expectedStatus)
		_, err := fmt.Fprint(w, expectedResponseBody)
		require.NoError(t, err)
	}))
	defer testReadmeServer.Close()

	defer func(url string) { readmeDocsUrl = url }(readmeDocsUrl)

	readmeDocsUrl = testReadmeServer.URL

	resp := GetDocument(context.Background(),
		expectedApiKey,
		expectedSlug)
	assert.Equal(t, expectedResponseBody, resp.Body)
	assert.Equal(t, expectedStatus, resp.Status)
}

func TestDocumentPathRegex(t *testing.T) {
	// Both forms must match: the prefix-stripped form (`/docs/...`) that
	// arrives from this service's dedicated API Gateway (api_mapping_key =
	// "readme"), and the legacy unstripped form (`/readme/docs/...`) that
	// arrives via pennsieve-go-api's gateway while that repo still defines
	// the route. Once pennsieve-go-api drops its route definition, the
	// unstripped form goes away.
	for path, match := range map[string]bool{
		// new prefix-stripped form
		"/docs/doc-page":                 true,
		"/docs/":                         true,
		"/docs":                          false,
		"/docs/doc-slug/some-more-stuff": false,
		// legacy unstripped form
		"/readme/docs/doc-page":                 true,
		"/readme/docs/":                         true,
		"/readme/docs":                          false,
		"/readme/docs/doc-slug/some-more-stuff": false,
		// unrelated
		"/something/else": false,
		"/search":         false,
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, match, documentPathRegex.MatchString(path))
		})
	}
}

func TestSearchPathRegex(t *testing.T) {
	for path, match := range map[string]bool{
		"/search":         true, // new prefix-stripped form
		"/readme/search":  true, // legacy unstripped form
		"/search/":        false,
		"/search/extra":   false,
		"/docs/something": false,
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, match, searchPathRegex.MatchString(path))
		})
	}
}
