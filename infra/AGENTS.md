# Infrastructure instructions

Read [the shared workflow](../docs/agents/workflow.md), [SETUP](../docs/SETUP.md), and the infrastructure ADR before Terraform or AWS work. Terraform is the repo's infrastructure source; inspect current provider constraints and account configuration rather than copying dated values into instructions.

## Prepare and authorize

Inspect configuration and propose the smallest scoped change. Run local formatting and validation where dependencies are available. Dependency installation, authentication, cost, public contracts, and destructive changes follow the shared approval rules.

Authentication commands, including `aws login`, require explicit user authorization. After authorization, verify the active profile, account, region, and identity before live work. Inspect permissions separately from configuration and actual execution results.

Show the concrete Terraform plan and its relevant additions, changes, deletions, and cost implications before requesting apply approval. A prior implementation or plan request does not authorize apply. Deployment and other AWS mutations need explicit authorization. Merge does not authorize deployment or apply.

Keep credentials in local environment/configuration and sanitize logs and plans before sharing them. Preserve state protections and the committed provider lockfile. Use `amazon-bedrock` only for explicit Bedrock or AgentCore work; it is not a general agent-development requirement.

## Verify and report

Report which local checks passed, whether a live plan ran, the verified account, and any blocked checks. Configuration validity is not evidence of deployment success. Only claim deployed behavior after observing the approved execution and its relevant result.
