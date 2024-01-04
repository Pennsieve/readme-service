resource "aws_lambda_function" "service_lambda" {
  description       = "a Lambda Function which handles requests for a serverless readme.io api proxy"
  function_name     = "${var.environment_name}-${var.service_name}-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler           = "readme_service"
  runtime           = "go1.x"
  role              = aws_iam_role.readme_service_lambda_role.arn
  timeout           = 300
  memory_size       = 128
  s3_bucket         = var.lambda_bucket
  s3_key            = "${var.service_name}/${var.service_name}-${var.image_tag}.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.upload_v2_security_group_id]
  }

  environment {
    variables = {
      ENV = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
      REGION = var.aws_region
    }
  }
}
