resource "aws_lambda_function" "service_lambda" {
  # Stamp the deployed build in the description so it's visible at the top of the
  # Lambda Console page. AWS discards the uploaded zip's filename post-deploy, so
  # image_tag is otherwise unrecoverable from the deployed function.
  description   = "a Lambda Function which handles requests for a serverless readme.io api proxy (image_tag: ${var.image_tag})"
  function_name = "${var.environment_name}-${var.service_name}-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]
  role          = aws_iam_role.readme_service_lambda_role.arn
  timeout       = 300
  memory_size   = 128
  s3_bucket     = var.lambda_bucket
  s3_key        = "${var.service_name}/${var.service_name}-${var.image_tag}.zip"

  # Resource tag: discoverable in the Console (Tags tab) and fleet-wide via the
  # Resource Groups Tagging API (get-resources --tag-filters Key=ImageTag).
  tags = merge(local.common_tags, {
    ImageTag = var.image_tag
  })

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.upload_v2_security_group_id]
  }

  environment {
    variables = {
      # Deployed build: readable by the running code (os.Getenv("IMAGE_TAG")) and via
      # `aws lambda get-function-configuration ... Environment.Variables.IMAGE_TAG`.
      IMAGE_TAG        = var.image_tag
      ENV              = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
      REGION           = var.aws_region
      README_API_KEY   = aws_ssm_parameter.readme_api_key.value
    }
  }
}
