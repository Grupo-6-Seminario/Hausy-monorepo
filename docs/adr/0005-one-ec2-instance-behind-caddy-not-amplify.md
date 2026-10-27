---
status: accepted
---

# Run the app on one EC2 instance behind Caddy, with RDS, instead of Amplify

The first AWS deployment had to be public and working the same day, for classmates and teachers to
search on their own. The frontend is not Next.js but vinext, which builds a Node server
(`output: 'standalone'`), and a search turn streams NDJSON for up to 65 seconds while the model
works. One EC2 instance runs Caddy, the web server and the Go API; Caddy gets a Let's Encrypt
certificate for the Elastic IP's `sslip.io` name, so no domain is needed. The API listens only on
localhost, and Postgres is RDS in the default VPC, reachable only from the instance. Releases ship
through S3 and SSM with `scripts/deploy.sh`; there is no SSH.

## Considered options

- **Amplify Hosting for the frontend.** Rejected for now: Amplify lists Next.js streaming as
  unsupported and has reports of responses cut at 30 seconds, it needs the vinext output repackaged
  into its deployment specification, and the repository belongs to an organization whose admin would
  have to grant access. Amplify was chosen only for simplicity, which one instance gives as well.
- **App Runner for the API.** Rejected: closed to new customers since 2026-04-30.
- **Lambda behind a function URL.** Rejected: RDS would have to be public, since Lambda outside a VPC
  has no fixed address, and each concurrent request would hold its own database connections.

## Consequences

- One instance is a single point of failure, which a course demo accepts.
- The site URL is the Elastic IP under `sslip.io`. A domain would replace it without other changes.
- About 32 USD a month gross, paid from the Free plan credits; the budget alert is at 40 USD. The
  stack is destroyed with `terraform destroy` when the course ends.
