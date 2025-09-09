# Development main.tf (this is for dev/test only)
# See the documentation for production deployment instructions
terraform {
  backend "s3" {}
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {}
