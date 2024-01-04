package handler

import (
	"github.com/aws/aws-lambda-go/events"
	"log"
)

func init() {
	log.Println("init() ")
}

func ReadmeServiceHandler(request events.APIGatewayV2HTTPRequest) (*events.APIGatewayV2HTTPResponse, error) {
	var err error
	var apiResponse *events.APIGatewayV2HTTPResponse

	log.Println("ReadmeServiceHandler() ")
	apiResponse, err = handleRequest()

	return apiResponse, err
}

func handleRequest() (*events.APIGatewayV2HTTPResponse, error) {
	log.Println("handleRequest() ")
	apiResponse := events.APIGatewayV2HTTPResponse{Body: "{'response':'hello'}", StatusCode: 200}

	return &apiResponse, nil
}
