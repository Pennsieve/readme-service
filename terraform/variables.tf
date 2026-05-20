variable "aws_account" {}

variable "aws_region" {}

variable "environment_name" {}

variable "service_name" {}

variable "vpc_name" {}

variable "domain_name" {}

variable "image_tag" {}

variable "lambda_bucket" {
  default = "pennsieve-cc-lambda-functions-use1"
}

// api2.pennsieve.{net,io} — the platform-wide HTTP API custom domain that
// this service's dedicated gateway mounts under (via api_mapping_key =
// "readme" in gateway.tf). Pass this in from the per-env entry in the
// infrastructure repo, same as packages-service does.
variable "api_domain_name" {}

locals {
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
  // CORS allowlist for the readme-service API Gateway. Limited to the
  // first-party Pennsieve web surfaces in each env.
  cors_allowed_origins = (var.environment_name == "prod"
    ? ["https://discover.pennsieve.io", "https://app.pennsieve.io", "https://docs.pennsieve.io"]
    : ["http://localhost:3000", "https://discover.pennsieve.net", "https://app.pennsieve.net", "https://docs.pennsieve.io"]
  )
}
