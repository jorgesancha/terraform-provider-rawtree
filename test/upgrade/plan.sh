#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/config"
go -C "$root" build -o "$work/bin/terraform-provider-rawtree" .
cp "$root/test/upgrade/main.tf" "$work/config/main.tf"
cp "$root/test/upgrade/waf-v0.1.0.json" "$work/config/terraform.tfstate"
cat > "$work/terraform.rc" <<CONFIG
provider_installation {
  dev_overrides { "rawtreedb/rawtree" = "$work/bin" }
  direct {}
}
CONFIG
# The fixture uses synthetic credentials. Skip AWS refresh and require an empty plan.
TF_CLI_CONFIG_FILE="$work/terraform.rc" terraform -chdir="$work/config" plan -input=false -refresh=false -detailed-exitcode
