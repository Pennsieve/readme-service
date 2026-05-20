package handler

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"io"
	"net/http"
	"regexp"
)

type ReadmeResponse struct {
	Body   string
	Status int
}

func (r *ReadmeResponse) AsAPIGatewayV2HTTPResponse() *events.APIGatewayV2HTTPResponse {
	return response(r.Body, r.Status)
}

func NewReadmeErrorResponse(status int, format string, args ...any) *ReadmeResponse {
	msg := fmt.Sprintf(format, args...)
	logger.Error("error getting document", "error", msg)
	return &ReadmeResponse{
		Body:   fmt.Sprintf(`{"message": %q}`, msg),
		Status: status,
	}
}

var readmeDocsUrl = "https://dash.readme.com/api/v1/docs"

// documentPathRegex tolerates two forms during the gateway migration:
//
//   /docs/{slug}            — the form arriving via the dedicated readme-service
//                             gateway (api_mapping_key strips the /readme prefix
//                             before forwarding; see terraform/gateway.tf)
//   /readme/docs/{slug}     — the legacy form arriving via pennsieve-go-api's
//                             monolithic gateway, which doesn't strip prefixes
//
// Both deploy paths can run concurrently. Once pennsieve-go-api removes its
// readme-service integration, the second form goes away and this regex can
// drop the optional `^/readme` group.
var documentPathRegex = regexp.MustCompile(`^(?:/readme)?/docs/[^/]*$`)

// searchPathRegex mirrors documentPathRegex's dual-form handling.
var searchPathRegex = regexp.MustCompile(`^(?:/readme)?/search$`)

func GetDocument(ctx context.Context, apiKey string, slug string) *ReadmeResponse {
	url := fmt.Sprintf("%s/%s", readmeDocsUrl, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error creating request to %s: %v", url, err)
	}
	req.Header.Set("accept", applicationJson)
	req.SetBasicAuth(apiKey, "")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error while calling %s %s: %v", http.MethodGet, url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("error closing response body", "url", url, "error", err)
		}
	}()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewReadmeErrorResponse(http.StatusInternalServerError, "error reading body of response from %s %s: %v", http.MethodGet, url, err)
	}
	return &ReadmeResponse{
		Body:   string(respBytes),
		Status: resp.StatusCode,
	}
}
