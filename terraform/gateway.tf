// Dedicated API Gateway for readme-service. Mirrors the packages-service
// pattern — each service owns its own gateway resource and OpenAPI spec
// rather than threading routes through the monolithic pennsieve-go-api
// gateway. Mount point: api2.pennsieve.{net,io}/readme/*.

resource "aws_apigatewayv2_api" "readme_service_api" {
  name          = "Readme Service Serverless API"
  protocol_type = "HTTP"
  description   = "API for the readme.io documentation proxy"

  cors_configuration {
    allow_origins     = local.cors_allowed_origins
    allow_methods     = ["OPTIONS", "GET"]
    allow_headers     = ["*"]
    allow_credentials = true
    expose_headers    = ["*"]
    max_age           = 300
  }

  body = templatefile("${path.module}/readme-service.yml", {
    authorize_lambda_invoke_uri = data.terraform_remote_state.api_gateway.outputs.authorizer_lambda_invoke_uri
    gateway_authorizer_role     = data.terraform_remote_state.api_gateway.outputs.authorizer_invocation_role
    readme_service_lambda_arn   = aws_lambda_function.service_lambda.arn
  })
}

// Mount at api2.pennsieve.{net,io}/readme. The api_mapping_key is the path
// prefix the platform-wide custom domain (var.api_domain_name) strips
// before routing requests into this gateway. So a request to
// https://api2.pennsieve.io/readme/docs/uploading-data is routed here with
// the in-gateway path /docs/uploading-data.
resource "aws_apigatewayv2_api_mapping" "readme_service_api_map" {
  api_id          = aws_apigatewayv2_api.readme_service_api.id
  domain_name     = var.api_domain_name
  stage           = aws_apigatewayv2_stage.readme_service_gateway_stage.id
  api_mapping_key = "readme"
}

resource "aws_apigatewayv2_stage" "readme_service_gateway_stage" {
  api_id      = aws_apigatewayv2_api.readme_service_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.readme_service_gateway_log_group.arn

    format = jsonencode({
      requestId               = "$context.requestId"
      sourceIp                = "$context.identity.sourceIp"
      requestTime             = "$context.requestTime"
      protocol                = "$context.protocol"
      httpMethod              = "$context.httpMethod"
      resourcePath            = "$context.resourcePath"
      routeKey                = "$context.routeKey"
      status                  = "$context.status"
      responseLength          = "$context.responseLength"
      integrationErrorMessage = "$context.integrationErrorMessage"
    })
  }
}

resource "aws_apigatewayv2_integration" "readme_service_integration" {
  api_id             = aws_apigatewayv2_api.readme_service_api.id
  integration_type   = "AWS_PROXY"
  connection_type    = "INTERNET"
  integration_method = "POST"
  integration_uri    = aws_lambda_function.service_lambda.invoke_arn
}

resource "aws_lambda_permission" "readme_service_lambda_permission" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.service_lambda.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.readme_service_api.execution_arn}/*/*"
}
