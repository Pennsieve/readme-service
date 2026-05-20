// Lambda log group — explicit so retention is consistent across services.
resource "aws_cloudwatch_log_group" "readme_service_lambda_log_group" {
  name              = "/aws/lambda/${aws_lambda_function.service_lambda.function_name}"
  retention_in_days = 30
  tags              = local.common_tags
}

// API Gateway access log group. Receives the per-request access log lines
// formatted by aws_apigatewayv2_stage.readme_service_gateway_stage.
resource "aws_cloudwatch_log_group" "readme_service_gateway_log_group" {
  name              = "${var.environment_name}/${var.service_name}/readme-api-gateway"
  retention_in_days = 30
}
