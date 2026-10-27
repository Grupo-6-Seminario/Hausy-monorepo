# The whole app on one instance: Caddy terminates TLS and proxies to the web server, which calls
# the API on localhost. Postgres is RDS. scripts/deploy.sh ships releases (docs/SETUP.md).

data "aws_caller_identity" "current" {}

# us-east-1e offers neither t4g.small nor db.t4g.micro, so both are pinned to one zone.
data "aws_subnet" "app" {
  availability_zone = "us-east-1a"
  default_for_az    = true
}

data "aws_ssm_parameter" "al2023" {
  name = "/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-arm64"
}

data "aws_bedrock_inference_profile" "sonnet" {
  inference_profile_id = "us.anthropic.claude-sonnet-4-6"
}

# The site host is the Elastic IP with dashes under sslip.io, so Caddy can get a certificate
# without a domain. The address is allocated apart from the instance so user data can name it.
resource "aws_eip" "app" {
  domain = "vpc"
  tags   = { Name = "hausy" }
}

locals {
  site_host = "${replace(aws_eip.app.public_ip, ".", "-")}.sslip.io"
}

resource "aws_security_group" "app" {
  name        = "hausy-app"
  description = "HTTP and HTTPS to Caddy. No SSH: use SSM."

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "db" {
  name        = "hausy-db"
  description = "Postgres from the app instance only."

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]
  }
}

resource "aws_db_instance" "db" {
  identifier             = "hausy"
  engine                 = "postgres"
  engine_version         = "14.24"
  instance_class         = "db.t4g.micro"
  allocated_storage      = 20
  storage_type           = "gp3"
  storage_encrypted      = true
  availability_zone      = data.aws_subnet.app.availability_zone
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  db_name                = "hausy"
  username               = "hausy"
  # The password lives in a Secrets Manager secret that RDS owns; it never enters state.
  manage_master_user_password = true
  backup_retention_period     = 1
  skip_final_snapshot         = true
  deletion_protection         = false
  # Standard support for 14 ends 2027-02-28; paid Extended Support would start on its own.
  engine_lifecycle_support = "open-source-rds-extended-support-disabled"
}

# RDS rotates its secret every 7 days by default, but the instance reads the password only at
# deploy time, so a rotation would lock the API out of new connections.
resource "aws_secretsmanager_secret_rotation" "db" {
  secret_id        = aws_db_instance.db.master_user_secret[0].secret_arn
  rotation_enabled = false
}

resource "aws_s3_bucket" "artifacts" {
  bucket_prefix = "hausy-artifacts-"
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "artifacts" {
  bucket                  = aws_s3_bucket.artifacts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Set the real key out of band (docs/SETUP.md). Write-only, so neither value is stored in state.
resource "aws_ssm_parameter" "ai_gateway_api_key" {
  name             = "/hausy/ai_gateway_api_key"
  type             = "SecureString"
  value_wo         = "unset"
  value_wo_version = 1
}

resource "aws_iam_role" "app" {
  name = "hausy-app"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Principal = { Service = "ec2.amazonaws.com" }, Action = "sts:AssumeRole" }]
  })
}

resource "aws_iam_role_policy_attachment" "ssm" {
  role       = aws_iam_role.app.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

data "aws_iam_policy_document" "app" {
  # The cross-region profile and every foundation model it routes to.
  statement {
    actions   = ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"]
    resources = concat([data.aws_bedrock_inference_profile.sonnet.inference_profile_arn], data.aws_bedrock_inference_profile.sonnet.models[*].model_arn)
  }

  statement {
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_db_instance.db.master_user_secret[0].secret_arn]
  }

  statement {
    actions   = ["ssm:GetParameter"]
    resources = ["arn:aws:ssm:us-east-1:${data.aws_caller_identity.current.account_id}:parameter/hausy/*"]
  }

  statement {
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.artifacts.arn}/*"]
  }
}

resource "aws_iam_role_policy" "app" {
  role   = aws_iam_role.app.id
  policy = data.aws_iam_policy_document.app.json
}

resource "aws_iam_instance_profile" "app" {
  name = "hausy-app"
  role = aws_iam_role.app.name
}

resource "aws_instance" "app" {
  ami                    = data.aws_ssm_parameter.al2023.insecure_value
  instance_type          = "t4g.small"
  subnet_id              = data.aws_subnet.app.id
  vpc_security_group_ids = [aws_security_group.app.id]
  iam_instance_profile   = aws_iam_instance_profile.app.name
  # User data runs on first boot only, so changing it replaces the instance; redeploy afterwards.
  user_data                   = templatefile("${path.module}/user_data.sh", { site_host = local.site_host })
  user_data_replace_on_change = true

  # Standard credits: a busy CPU throttles instead of billing surplus credits.
  credit_specification {
    cpu_credits = "standard"
  }

  metadata_options {
    http_tokens = "required"
  }

  root_block_device {
    volume_size = 16
    volume_type = "gp3"
    encrypted   = true
  }

  tags = { Name = "hausy" }

  # A newer AMI must not replace a running instance.
  lifecycle {
    ignore_changes = [ami]
  }
}

resource "aws_eip_association" "app" {
  instance_id   = aws_instance.app.id
  allocation_id = aws_eip.app.id
}

output "site_url" {
  value = "https://${local.site_host}"
}

output "instance_id" {
  value = aws_instance.app.id
}

output "artifacts_bucket" {
  value = aws_s3_bucket.artifacts.bucket
}

output "db_endpoint" {
  value = aws_db_instance.db.endpoint
}

output "db_secret_arn" {
  value = aws_db_instance.db.master_user_secret[0].secret_arn
}
