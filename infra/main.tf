terraform {
  required_version = ">= 1.15"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.66"
    }
  }

  # Created once with the AWS CLI (docs/SETUP.md), never managed here.
  backend "s3" {
    bucket       = "hausy-tfstate-csofja"
    key          = "infra/terraform.tfstate"
    region       = "us-east-1"
    encrypt      = true
    use_lockfile = true
  }
}

# No profile here: each founder sets AWS_PROFILE.
provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      ManagedBy = "terraform"
      Project   = "hausy"
    }
  }
}
