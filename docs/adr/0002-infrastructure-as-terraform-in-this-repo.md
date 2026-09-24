# AWS infrastructure is Terraform in this repo, with state in S3 and plans reviewed in the PR

Until now, AWS resources were created by hand, and nothing recorded what existed or why. The AgentCore
harnesses outlived the code that called them. From now on, AWS infrastructure is Terraform under `infra/`
in this repository, not in a separate repo. The team is four founders with one app and one AWS account,
so a change and the infrastructure it needs land in the same PR. State lives in a versioned, encrypted
S3 bucket and uses S3-native locking (`use_lockfile`), so four people can apply without a lock table or
an account outside AWS. There is no CI, so the PR body carries the `terraform plan` output, and the
author runs `terraform apply` from `dev` after the merge, with their own `aws login` session.

## Considered options

- **A separate infrastructure repo.** Rejected: it pays off when several apps share infrastructure or a
  different team owns it, and neither is true here.
- **HCP Terraform.** Rejected: another account, plus AWS credentials stored outside AWS, to get a plan
  UI that the PR body already provides.

## Consequences

- The state bucket is created once with the CLI, documented in `docs/SETUP.md`, and never managed by
  Terraform, so Terraform never manages its own state.
- Resources created before this decision are imported before they are changed or destroyed. The import
  plan changes nothing; removing the config then produces a visible destroy plan.
- `dev` is the source of truth for applied infrastructure. A plan from a feature branch is a preview,
  not an apply.
