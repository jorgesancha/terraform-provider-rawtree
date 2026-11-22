---
page_title: "RawTree Provider"
description: Manage RawTree resources and external ingestion integrations.
---

# RawTree Provider

Manage named databases in existing RawTree clusters. The provider also configures ingestion from S3, WAF, CloudFront, and Supabase.

## Authentication

Set `management_token` or `RAWTREE_MANAGEMENT_TOKEN` for native resources. A cluster-scoped admin API key can manage databases.
If you omit the management token, native resources use the configured API key.
The database resource takes its organization and cluster as arguments.

Ingestion integrations use `api_key`, `organization`, and `project` from the provider block, environment, or saved CLI settings, in that order.
Use a dedicated ingestion key when a configuration also manages databases. The provider keeps the management token separate from credentials it sends to AWS.

## Example Usage

```hcl
terraform {
  required_providers {
    rawtree = {
      source = "rawtreedb/rawtree"
    }
  }
}

# Set RAWTREE_MANAGEMENT_TOKEN to a credential with database administration access.
provider "rawtree" {}

resource "rawtree_database" "analytics" {
  organization = "acme"
  cluster      = "production"
  name         = "analytics"
}
```

## Schema

### Optional

- `management_token` (String, Sensitive) sets the credential for native resources. Set it through `RAWTREE_MANAGEMENT_TOKEN`.
- `api_key` (String, Sensitive) sets the ingestion key and the fallback credential for native resources. Set it through `RAWTREE_API_KEY`.
- `api_url` (String) sets the API origin for both resource families. The defaults are `https://api.rawtree.com` for native resources and `https://api.us-east-1.aws.rawtree.com` for integrations. Set it through `RAWTREE_URL`.
- `organization` (String) sets the integration organization. Set it through `RAWTREE_ORG`.
- `project` (String) sets the integration project. Set it through `RAWTREE_PROJECT`.
