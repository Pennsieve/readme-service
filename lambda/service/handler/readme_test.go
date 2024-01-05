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
