resource "aws_ssm_parameter" "readme_api_key" {
  name      = "/${var.environment_name}/${var.service_name}/readme-api-key"
  overwrite = false
  type      = "SecureString"
  value     = "dummy"

  lifecycle {
    ignore_changes = [value]
  }
}
