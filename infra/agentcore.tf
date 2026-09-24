# AgentCore resources created by hand in the console, imported so that removing this file
# produces a visible destroy plan (ADR 0002). Nothing here is meant to change: the plan must
# show 0 to change.
#
# Destroy order. Each harness owns the runtime (with its DEFAULT endpoint and workload
# identity) and the managed memory that AWS created for it, and DeleteHarness cascades to
# the memory. AWS refuses to delete a harness-managed memory directly. So every harness
# depends_on what it owns, and each runtime on its endpoint and identity: the owner is
# destroyed first and the owned resources after it, where a 404 counts as deleted. The
# depends_on lines run the opposite way to ownership on purpose. They reach the destroy
# only through the dependencies Terraform records in state at apply time.
#
# The harness DEFAULT endpoints have no Terraform resource and go with their harness.

# default_tags cannot be excluded per resource, and would retag everything imported here. A
# plan that only imports must change nothing, so these resources use a provider without them.
# Keep this block until the destroy is applied: Terraform needs it to delete them.
provider "aws" {
  alias  = "untagged"
  region = "us-east-1"
}

data "aws_caller_identity" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id

  # Keyed by harness name ("sellling" is spelled that way in AWS).
  harnesses = {
    customer_agent_dev = {
      runtime_id = "harness_customer_agent_dev-l9uxMFBuBz"
      memory_id  = "customer_agent_dev-jeQfwC3z4o"
    }
    sellling_agent_dev = {
      runtime_id = "harness_sellling_agent_dev-I9sH6Z4o6T"
      memory_id  = "sellling_agent_dev-vi53KoHdI5"
    }
  }

  # The console's default service role and execution policy for each harness. harness_i4yq9
  # was deleted before this import and left its role behind, attached to nothing.
  harness_roles = {
    customer_agent_dev = { role = "AmazonBedrockAgentCoreHarnessDefaultServiceRole-0yup6", policy = "AmazonBedrockAgentCoreHarnessExecutionPolicy_hwmyt" }
    sellling_agent_dev = { role = "AmazonBedrockAgentCoreHarnessDefaultServiceRole-p2r9o", policy = "AmazonBedrockAgentCoreHarnessExecutionPolicy_b96nv" }
    harness_i4yq9      = { role = "AmazonBedrockAgentCoreHarnessDefaultServiceRole-76y50", policy = "AmazonBedrockAgentCoreHarnessExecutionPolicy_aw2p8" }
  }

  policy_arn_prefix = "arn:aws:iam::${local.account_id}:policy/service-role/"
}

# --- Harnesses ---------------------------------------------------------------------------

import {
  to = aws_bedrockagentcore_harness.customer_agent_dev
  id = "customer_agent_dev-mLgKpnYt0f"
}

import {
  to = aws_bedrockagentcore_harness.sellling_agent_dev
  id = "sellling_agent_dev-D4WqQDGkwY"
}

# system_prompt is required, so it holds a placeholder. The real prompts and environment
# variables stay in AWS and never enter git; ignore_changes keeps the plan at 0 to change.
resource "aws_bedrockagentcore_harness" "customer_agent_dev" {
  provider = aws.untagged

  harness_name       = "customer_agent_dev"
  execution_role_arn = aws_iam_role.harness["customer_agent_dev"].arn
  allowed_tools      = ["*"]
  max_iterations     = 12
  max_tokens         = 4096
  timeout_seconds    = 300
  truncation = [{
    strategy = "sliding_window"
    config = [{
      sliding_window = [{ messages_count = 150 }]
      summarization  = null
    }]
  }]

  environment {
    agentcore_runtime_environment {
      lifecycle_configuration = [{
        idle_runtime_session_timeout = 900
        max_lifetime                 = 28800
      }]
      network_configuration {
        network_mode = "PUBLIC"
      }
    }
  }

  memory {
    managed_memory_configuration {
      event_expiry_duration = 30
      strategies            = ["SEMANTIC", "SUMMARIZATION"]
    }
  }

  model {
    bedrock_model_config {
      api_format = "converse_stream"
      max_tokens = 4096
      model_id   = "us.openai.gpt-5.6-luna"
    }
  }

  system_prompt {
    text = "PLACEHOLDER"
  }

  depends_on = [
    aws_bedrockagentcore_agent_runtime.harness,
    aws_bedrockagentcore_memory.harness,
    aws_iam_role_policy_attachment.harness,
  ]

  lifecycle {
    ignore_changes = [system_prompt, environment_variables]
  }
}

resource "aws_bedrockagentcore_harness" "sellling_agent_dev" {
  provider = aws.untagged

  harness_name       = "sellling_agent_dev"
  execution_role_arn = aws_iam_role.harness["sellling_agent_dev"].arn
  allowed_tools      = ["*"]
  max_iterations     = 75
  timeout_seconds    = 3600
  truncation = [{
    strategy = "sliding_window"
    config = [{
      sliding_window = [{ messages_count = 150 }]
      summarization  = null
    }]
  }]

  environment {
    agentcore_runtime_environment {
      lifecycle_configuration = [{
        idle_runtime_session_timeout = 900
        max_lifetime                 = 28800
      }]
      network_configuration {
        network_mode = "PUBLIC"
      }
    }
  }

  memory {
    managed_memory_configuration {
      event_expiry_duration = 30
      strategies            = ["SEMANTIC", "SUMMARIZATION"]
    }
  }

  model {
    bedrock_model_config {
      api_format = "converse_stream"
      model_id   = "global.anthropic.claude-sonnet-4-6"
    }
  }

  system_prompt {
    text = "PLACEHOLDER"
  }

  tool {
    name = "aws_browser_v1"
    type = "agentcore_browser"
    config {
      agentcore_browser {
        browser_arn = "arn:aws:bedrock-agentcore:us-east-1:aws:browser/aws.browser.v1"
      }
    }
  }

  depends_on = [
    aws_bedrockagentcore_agent_runtime.harness,
    aws_bedrockagentcore_memory.harness,
    aws_iam_role_policy_attachment.harness,
  ]

  lifecycle {
    ignore_changes = [system_prompt, environment_variables]
  }
}

# --- What each harness owns --------------------------------------------------------------

import {
  for_each = local.harnesses
  to       = aws_bedrockagentcore_agent_runtime.harness[each.key]
  id       = each.value.runtime_id
}

import {
  for_each = local.harnesses
  to       = aws_bedrockagentcore_agent_runtime_endpoint.harness[each.key]
  id       = "${each.value.runtime_id},DEFAULT"
}

import {
  for_each = local.harnesses
  to       = aws_bedrockagentcore_workload_identity.harness[each.key]
  id       = each.value.runtime_id
}

import {
  for_each = local.harnesses
  to       = aws_bedrockagentcore_memory.harness[each.key]
  id       = each.value.memory_id
}

resource "aws_bedrockagentcore_agent_runtime" "harness" {
  for_each = local.harnesses
  provider = aws.untagged

  agent_runtime_name = "harness_${each.key}"
  role_arn           = aws_iam_role.harness[each.key].arn

  agent_runtime_artifact {
    container_configuration {
      container_uri = "public.ecr.aws/i0n3d3i5/harness-us-east-1:latest"
    }
  }

  network_configuration {
    network_mode = "PUBLIC"
  }

  depends_on = [
    aws_bedrockagentcore_agent_runtime_endpoint.harness,
    aws_bedrockagentcore_workload_identity.harness,
    aws_iam_role_policy_attachment.harness,
  ]

  lifecycle {
    ignore_changes = [environment_variables]
  }
}

resource "aws_bedrockagentcore_agent_runtime_endpoint" "harness" {
  for_each = local.harnesses
  provider = aws.untagged

  agent_runtime_id = each.value.runtime_id
  name             = "DEFAULT"
}

resource "aws_bedrockagentcore_workload_identity" "harness" {
  for_each = local.harnesses
  provider = aws.untagged

  name = each.value.runtime_id
}

resource "aws_bedrockagentcore_memory" "harness" {
  for_each = local.harnesses
  provider = aws.untagged

  name                  = each.key
  event_expiry_duration = 30
}

# --- IAM ---------------------------------------------------------------------------------

import {
  for_each = local.harness_roles
  to       = aws_iam_role.harness[each.key]
  id       = each.value.role
}

import {
  for_each = local.harness_roles
  to       = aws_iam_policy.harness[each.key]
  id       = "${local.policy_arn_prefix}${each.value.policy}"
}

# Imported too, or DeleteRole fails with DeleteConflict while the policy is still attached.
import {
  for_each = local.harness_roles
  to       = aws_iam_role_policy_attachment.harness[each.key]
  id       = "${each.value.role}/${local.policy_arn_prefix}${each.value.policy}"
}

resource "aws_iam_role" "harness" {
  for_each = local.harness_roles
  provider = aws.untagged

  name = each.value.role
  path = "/service-role/"

  assume_role_policy = jsonencode({
    Statement = [{
      Action = "sts:AssumeRole"
      Condition = {
        ArnLike = {
          "aws:SourceArn" = "arn:aws:bedrock-agentcore:us-east-1:${local.account_id}:*"
        }
        StringEquals = {
          "aws:SourceAccount" = local.account_id
        }
      }
      Effect = "Allow"
      Principal = {
        Service = "bedrock-agentcore.amazonaws.com"
      }
    }]
    Version = "2012-10-17"
  })
}

resource "aws_iam_policy" "harness" {
  for_each = local.harness_roles
  provider = aws.untagged

  name = each.value.policy
  path = "/service-role/"

  policy = jsonencode({
    Statement = [{
      Action   = ["bedrock:InvokeModel", "bedrock:InvokeModelWithResponseStream"]
      Effect   = "Allow"
      Resource = ["arn:aws:bedrock:*::foundation-model/*", "arn:aws:bedrock:us-east-1:${local.account_id}:*"]
      Sid      = "BedrockModelInvocation"
      }, {
      Action   = ["bedrock-mantle:CreateInference"]
      Effect   = "Allow"
      Resource = "arn:aws:bedrock-mantle:us-east-1:${local.account_id}:*"
      Sid      = "BedrockMantleInference"
      }, {
      Action   = ["bedrock-mantle:CallWithBearerToken"]
      Effect   = "Allow"
      Resource = "*"
      Sid      = "BedrockMantleCallWithBearerToken"
      }, {
      Action   = ["ecr-public:GetAuthorizationToken"]
      Effect   = "Allow"
      Resource = "*"
      Sid      = "EcrPublicTokenAccess"
      }, {
      Action   = ["sts:GetServiceBearerToken"]
      Effect   = "Allow"
      Resource = "*"
      Sid      = "StsForEcrPublicPull"
      }, {
      Action   = ["ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer", "ecr:BatchCheckLayerAvailability"]
      Effect   = "Allow"
      Resource = "arn:aws:ecr:us-east-1:*:repository/harness-*"
      Sid      = "EcrManagedImagePull"
      }, {
      Action   = ["ecr:GetAuthorizationToken"]
      Effect   = "Allow"
      Resource = "*"
      Sid      = "EcrManagedImageToken"
      }, {
      Action   = ["xray:PutTraceSegments", "xray:PutTelemetryRecords", "xray:GetSamplingRules", "xray:GetSamplingTargets"]
      Effect   = "Allow"
      Resource = "*"
      Sid      = "XRayTracingAccess"
      }, {
      Action   = ["logs:CreateLogGroup", "logs:DescribeLogStreams"]
      Effect   = "Allow"
      Resource = "arn:aws:logs:us-east-1:${local.account_id}:log-group:/aws/bedrock-agentcore/runtimes/*"
      Sid      = "CloudWatchLogsGroup"
      }, {
      Action   = ["logs:DescribeLogGroups"]
      Effect   = "Allow"
      Resource = "arn:aws:logs:us-east-1:${local.account_id}:log-group:*"
      Sid      = "CloudWatchLogsDescribeGroups"
      }, {
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
      Effect   = "Allow"
      Resource = "arn:aws:logs:us-east-1:${local.account_id}:log-group:/aws/bedrock-agentcore/runtimes/*:log-stream:*"
      Sid      = "CloudWatchLogsStream"
      }, {
      Action   = ["logs:PutResourcePolicy"]
      Effect   = "Allow"
      Resource = ["arn:aws:logs:us-east-1:${local.account_id}:log-group:/aws/bedrock-agentcore/runtimes/harness_${each.key}-*"]
      Sid      = "CloudWatchLogsPutResourcePolicy"
      }, {
      Action = "cloudwatch:PutMetricData"
      Condition = {
        StringEquals = {
          "cloudwatch:namespace" = "bedrock-agentcore"
        }
      }
      Effect   = "Allow"
      Resource = "*"
      Sid      = "CloudWatchMetricsPublish"
      }, {
      Action   = ["bedrock-agentcore:GetWorkloadAccessToken", "bedrock-agentcore:GetWorkloadAccessTokenForJWT"]
      Effect   = "Allow"
      Resource = ["arn:aws:bedrock-agentcore:us-east-1:${local.account_id}:workload-identity-directory/default", "arn:aws:bedrock-agentcore:us-east-1:${local.account_id}:workload-identity-directory/default/workload-identity/harness_${each.key}-*"]
      Sid      = "AgentCoreWorkloadIdentity"
      }, {
      Action   = ["bedrock-agentcore:StartBrowserSession", "bedrock-agentcore:StopBrowserSession", "bedrock-agentcore:GetBrowserSession", "bedrock-agentcore:ListBrowserSessions", "bedrock-agentcore:UpdateBrowserStream", "bedrock-agentcore:ConnectBrowserAutomationStream", "bedrock-agentcore:ConnectBrowserLiveViewStream"]
      Effect   = "Allow"
      Resource = "arn:aws:bedrock-agentcore:us-east-1:aws:browser/*"
      Sid      = "AgentCoreBrowserDefault"
      }, {
      Action   = ["bedrock-agentcore:StartCodeInterpreterSession", "bedrock-agentcore:StopCodeInterpreterSession", "bedrock-agentcore:GetCodeInterpreterSession", "bedrock-agentcore:ListCodeInterpreterSessions", "bedrock-agentcore:InvokeCodeInterpreter"]
      Effect   = "Allow"
      Resource = "arn:aws:bedrock-agentcore:us-east-1:aws:code-interpreter/*"
      Sid      = "AgentCoreCodeInterpreterDefault"
      }, {
      Action = ["elasticfilesystem:ClientMount", "elasticfilesystem:ClientWrite"]
      Condition = {
        ArnLike = {
          "elasticfilesystem:AccessPointArn" = "arn:aws:elasticfilesystem:us-east-1:${local.account_id}:access-point/*"
        }
      }
      Effect   = "Allow"
      Resource = "arn:aws:elasticfilesystem:us-east-1:${local.account_id}:file-system/*"
      Sid      = "EFSClientAccess"
      }, {
      Action   = ["elasticfilesystem:DescribeAccessPoints", "elasticfilesystem:DescribeMountTargets"]
      Effect   = "Allow"
      Resource = ["arn:aws:elasticfilesystem:us-east-1:${local.account_id}:file-system/*", "arn:aws:elasticfilesystem:us-east-1:${local.account_id}:access-point/*"]
      Sid      = "EFSDescribe"
      }, {
      Action = ["s3files:ClientMount", "s3files:ClientWrite", "s3files:ClientRootAccess"]
      Condition = {
        ArnLike = {
          "s3files:AccessPointArn" = "arn:aws:s3files:us-east-1:${local.account_id}:file-system/*/access-point/*"
        }
      }
      Effect   = "Allow"
      Resource = "arn:aws:s3files:us-east-1:${local.account_id}:file-system/*"
      Sid      = "S3FilesClientAccess"
      }, {
      Action   = ["s3files:GetAccessPoint", "s3files:ListMountTargets"]
      Effect   = "Allow"
      Resource = ["arn:aws:s3files:us-east-1:${local.account_id}:file-system/*", "arn:aws:s3files:us-east-1:${local.account_id}:file-system/*/access-point/*"]
      Sid      = "S3FilesDescribe"
      }, {
      Action   = ["bedrock-agentcore:CreateEvent", "bedrock-agentcore:DeleteEvent", "bedrock-agentcore:GetEvent", "bedrock-agentcore:ListEvents", "bedrock-agentcore:RetrieveMemoryRecords"]
      Effect   = "Allow"
      Resource = "arn:aws:bedrock-agentcore:us-east-1:${local.account_id}:memory/${each.key}-*"
      Sid      = "AgentCoreMemory"
    }]
    Version = "2012-10-17"
  })
}

resource "aws_iam_role_policy_attachment" "harness" {
  for_each = local.harness_roles
  provider = aws.untagged

  role       = aws_iam_role.harness[each.key].name
  policy_arn = aws_iam_policy.harness[each.key].arn
}
