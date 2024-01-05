package handler

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"log/slog"
	"net/http"
	"os"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

func ReadmeServiceHandler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (*events.APIGatewayV2HTTPResponse, error) {
	logger = logger.With("requestID", request.RequestContext.RequestID)
	apiKey, errorResp := readmeApiKey()
	if errorResp != nil {
		return errorResp, nil
	}
	path := request.RequestContext.HTTP.Path
	switch path {
	case "/readme/docs":
		return handleRequest(ctx, request, apiKey)
	default:
		return NewReadmeErrorResponse(http.StatusNotFound, "resource not found: %s", path).AsAPIGatewayV2HTTPResponse(), nil
	}
}

func handleRequest(ctx context.Context, request events.APIGatewayV2HTTPRequest, apiKey string) (*events.APIGatewayV2HTTPResponse, error) {
	method := request.RequestContext.HTTP.Method
	if method != http.MethodGet {
		return NewReadmeErrorResponse(http.StatusMethodNotAllowed,
			"unsupported method for path %s: %s",
			request.RequestContext.HTTP.Path,
			method).AsAPIGatewayV2HTTPResponse(), nil
	}
	slug, ok := request.PathParameters["slug"]
	if !ok || len(slug) == 0 {
		return NewReadmeErrorResponse(http.StatusBadRequest, "no slug requested").AsAPIGatewayV2HTTPResponse(), nil
	}
	logger = logger.With("slug", slug)
	readmeResponse := GetDocument(ctx, apiKey, slug)
	return &events.APIGatewayV2HTTPResponse{Body: readmeResponse.Body, StatusCode: readmeResponse.Status}, nil

}

func readmeApiKey() (string, *events.APIGatewayV2HTTPResponse) {
	envKey := "README_API_KEY"
	if apiKey, ok := os.LookupEnv(envKey); !ok {
		return "", NewReadmeErrorResponse(http.StatusInternalServerError, "environment variable %s is not set", envKey).AsAPIGatewayV2HTTPResponse()
	} else if len(apiKey) == 0 {
		return "", NewReadmeErrorResponse(http.StatusInternalServerError, "environment variable %s is empty", envKey).AsAPIGatewayV2HTTPResponse()
	} else {
		return apiKey, nil
	}
}
