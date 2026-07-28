# Application secrets stored in AWS Secrets Manager and synced into the cluster by
# the External Secrets Operator (ESO). The app's Postgres DSN (which embeds the
# RDS endpoint and password) and a generated JWT signing secret live here rather
# than in a plaintext Kubernetes Secret.

resource "random_password" "jwt" {
  length  = 48
  special = false
}

locals {
  postgres_dsn = "postgres://${var.db_username}:${var.db_password}@${module.rds.endpoint}:${module.rds.port}/${var.db_name}?sslmode=require"
}

resource "aws_secretsmanager_secret" "app" {
  name = "${var.project}/app"

  # Short-lived lab: delete immediately instead of a 7–30 day recovery window,
  # so the name can be reused across teardown/redeploy cycles.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "app" {
  secret_id = aws_secretsmanager_secret.app.id
  secret_string = jsonencode({
    POSTGRES_DSN = local.postgres_dsn
    JWT_SECRET   = random_password.jwt.result
  })
}
