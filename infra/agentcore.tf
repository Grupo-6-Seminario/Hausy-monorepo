# The AgentCore resources imported here were removed, so the plan destroys them. The harness
# goes first and what it owned follows, in the order recorded in state (see git history for
# the import).
#
# Terraform still needs the provider they were imported with to delete them. Delete this file
# once that destroy is applied.
provider "aws" {
  alias  = "untagged"
  region = "us-east-1"
}
