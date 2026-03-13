terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# ─────────────────────────────────────────────
# User Pool
# ─────────────────────────────────────────────
resource "aws_cognito_user_pool" "main" {
  name = "${var.prefix}-users"

  # Use email as the username
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  username_configuration {
    case_sensitive = false
  }

  # Password policy
  password_policy {
    minimum_length                   = 8
    require_uppercase                = true
    require_lowercase                = true
    require_numbers                  = true
    require_symbols                  = false
    temporary_password_validity_days = 7
  }

  # MFA: optional TOTP
  mfa_configuration = "OPTIONAL"

  software_token_mfa_configuration {
    enabled = true
  }

  # Email verification
  verification_message_template {
    default_email_option = "CONFIRM_WITH_CODE"
    email_subject        = "Your Cash 5 verification code"
    email_message        = "Your verification code is {####}"
  }

  # Account recovery via email
  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  # Schema attributes
  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = true
    mutable             = true

    string_attribute_constraints {
      min_length = 5
      max_length = 254
    }
  }

  # Email configuration (Cognito default sender)
  email_configuration {
    email_sending_account = "COGNITO_DEFAULT"
  }

  # Admin create user config
  admin_create_user_config {
    allow_admin_create_user_only = false
  }

  tags = var.tags
}

# ─────────────────────────────────────────────
# App Client  (no secret — mobile / SPA)
# ─────────────────────────────────────────────
resource "aws_cognito_user_pool_client" "app" {
  name         = "${var.prefix}-app-client"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret = false

  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
    "ALLOW_USER_SRP_AUTH",
  ]

  # Token validity
  access_token_validity  = 1  # hours
  id_token_validity      = 1  # hours
  refresh_token_validity = 30 # days

  token_validity_units {
    access_token  = "hours"
    id_token      = "hours"
    refresh_token = "days"
  }

  # OAuth / hosted UI settings
  supported_identity_providers = ["COGNITO"]
  callback_urls                = var.app_callback_urls
  logout_urls                  = var.app_callback_urls

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true

  prevent_user_existence_errors = "ENABLED"
}

# ─────────────────────────────────────────────
# User Groups
# ─────────────────────────────────────────────
resource "aws_cognito_user_group" "users" {
  name         = "users"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Standard application users"
  precedence   = 10
}

resource "aws_cognito_user_group" "admins" {
  name         = "admins"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Administrator users"
  precedence   = 1
}

# ─────────────────────────────────────────────
# User Pool Domain  (for hosted UI / JWKS endpoint)
# ─────────────────────────────────────────────
resource "aws_cognito_user_pool_domain" "main" {
  domain       = var.prefix
  user_pool_id = aws_cognito_user_pool.main.id
}
