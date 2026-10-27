#!/usr/bin/env bash
# Builds a release, uploads it and switches the instance from infra/ to it (docs/SETUP.md).
# Every run ships a new release. --seed also loads the committed listings into RDS and, when the
# /hausy/demo_password parameter exists, the demo agency (cmd/demo).
set -euo pipefail
cd "$(dirname "$0")/.."

case "${1:-}" in
"") seed=false ;;
--seed) seed=true ;;
*)
  echo "usage: scripts/deploy.sh [--seed]" >&2
  exit 2
  ;;
esac

out() { terraform -chdir=infra output -raw "$1"; }
site=$(out site_url)
instance=$(out instance_id)
bucket=$(out artifacts_bucket)
db_endpoint=$(out db_endpoint)
db_secret=$(out db_secret_arn)
release=$(date -u +%Y%m%dT%H%M%SZ)-$(git describe --always --dirty)

build=$(mktemp -d)
trap 'rm -rf "$build"' EXIT
mkdir "$build/bin" "$build/data"
for cmd in hausy listings demo; do
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$build/bin/$cmd" "./cmd/$cmd"
done
# The build inlines NEXT_PUBLIC_SITE_URL. The standalone output is plain JS and wasm, with
# no native modules, so a macOS build runs on the arm64 Linux instance. vinext leaves out
# React, its peer dependency, which only resolved locally through frontend/node_modules.
(cd frontend && NEXT_PUBLIC_SITE_URL="$site" npm run build)
cp -R frontend/dist/standalone "$build/web"
cp -R frontend/node_modules/{react,react-dom,scheduler} "$build/web/node_modules/"
cp data/*.jsonl "$build/data"
COPYFILE_DISABLE=1 tar --no-xattrs -czf "$build/release.tgz" -C "$build" bin web data
aws s3 cp --only-show-errors "$build/release.tgz" "s3://$bucket/releases/$release.tgz"

# Runs on the instance as root. Secrets stay on the instance: nothing prints them.
aws s3 cp --only-show-errors - "s3://$bucket/releases/$release.sh" <<'EOF'
set -euo pipefail
release=$1 bucket=$2 db_endpoint=$3 db_secret=$4 seed=$5
dir=/opt/hausy/releases/$release
mkdir -p "$dir"
aws s3 cp "s3://$bucket/releases/$release.tgz" - | tar -xz --no-same-owner -C "$dir"

db_uri=$(aws secretsmanager get-secret-value --secret-id "$db_secret" --query SecretString --output text |
  jq -r --arg host "$db_endpoint" '"postgresql://\(.username | @uri):\(.password | @uri)@\($host)/hausy?sslmode=require"')
ai_key=$(aws ssm get-parameter --name /hausy/ai_gateway_api_key --with-decryption --query Parameter.Value --output text)
if [ "$ai_key" = unset ]; then
  echo "Set the /hausy/ai_gateway_api_key parameter first (docs/SETUP.md)." >&2
  exit 1
fi
umask 077
cat >/etc/hausy/api.env <<ENV
HAUSY_API_ADDR=127.0.0.1:8080
DATABASE_URI=$db_uri
HAUSY_LLM=bedrock
BEDROCK_MODEL_ID=us.anthropic.claude-sonnet-4-6
AWS_REGION=us-east-1
HAUSY_PLANNER=jev
AI_GATEWAY_API_KEY=$ai_key
HAUSY_LOG_LEVEL=info
HAUSY_REALTOR_SIGNUP=closed
ENV
cat >/etc/hausy/web.env <<ENV
HOST=127.0.0.1
PORT=3000
HAUSY_BACKEND_URL=http://127.0.0.1:8080
VINEXT_TRUST_PROXY=1
ENV
ln -sfn "$dir" /opt/hausy/current

healthy() {
  for _ in $(seq 60); do
    curl -fs -o /dev/null "$1" && return
    sleep 2
  done
  echo "$1 never answered. Recent logs:" >&2
  journalctl -u hausy-api -u hausy-web -n 40 --no-pager >&2
  return 1
}
# The API applies migrations at startup, so it restarts before the seed and the web server.
systemctl restart hausy-api
healthy http://127.0.0.1:8080/api/eligibility/facts
if [ "$seed" = true ]; then
  (cd "$dir" && DATABASE_URI=$db_uri bin/listings load)
  if demo_password=$(aws ssm get-parameter --name /hausy/demo_password --with-decryption --query Parameter.Value --output text 2>/dev/null); then
    DATABASE_URI=$db_uri HAUSY_DEMO_PASSWORD=$demo_password "$dir/bin/demo"
  else
    echo "No /hausy/demo_password parameter: skipping the demo agency."
  fi
fi
systemctl restart hausy-web
healthy http://127.0.0.1:3000/api/eligibility/facts
# Keep the three newest releases.
ls -1dt /opt/hausy/releases/* | tail -n +4 | xargs -r rm -rf
echo "deployed $release"
EOF

command="aws s3 cp s3://$bucket/releases/$release.sh /tmp/hausy-release.sh && bash /tmp/hausy-release.sh $release $bucket $db_endpoint $db_secret $seed"
id=$(aws ssm send-command --instance-ids "$instance" --document-name AWS-RunShellScript \
  --comment "deploy $release" --parameters "{\"commands\":[\"$command\"]}" \
  --query Command.CommandId --output text)
invocation() { aws ssm get-command-invocation --command-id "$id" --instance-id "$instance" --query "$1" --output text; }
for _ in $(seq 120); do
  sleep 5
  status=$(invocation Status 2>/dev/null || echo Pending)
  case $status in Pending | InProgress | Delayed) ;; *) break ;; esac
done
invocation StandardOutputContent
invocation StandardErrorContent >&2
if [ "$status" != Success ]; then
  echo "Deploy $release: $status" >&2
  exit 1
fi

# From outside: Caddy, its certificate, the web server, the API and the database.
curl -fsS --retry 30 --retry-delay 5 --retry-all-errors -o /dev/null "$site/api/eligibility/facts"
echo "$site is serving $release"
