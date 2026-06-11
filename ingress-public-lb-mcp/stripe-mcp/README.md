# Stripe MCP Server

MCP server that exposes Stripe tools for listing products, looking up customers, and processing payments. Runs behind the regional load balancer where the `ping-authz-shim` enforces authorization on every request before it reaches this server.

```
MCP Client → Regional Load Balancer → ping-authz-shim (allow/deny) → stripe-mcp
```

## OAuth Discovery

OAuth discovery endpoints (`/.well-known/`) are passed through unauthenticated so MCP clients can bootstrap the authorization flow. These are only needed by **attended agents** (e.g. Claude Desktop via `mcp-remote`) that must discover the authorization server to initiate a user login. **Delegated agents** (e.g. `ping-store-agent`) already know the AIC token endpoint and perform RFC 8693 token exchange directly — they never hit these endpoints.

## Tools

| Tool | Description | Parameters |
|---|---|---|
| `list_stripe_products` | List all active products from the Stripe catalog, including prices | — |
| `get_stripe_product` | Get details for a specific Stripe product | `product_id` (required) |
| `get_stripe_customer` | Look up the authenticated user's Stripe customer record and saved payment method | — (uses auth context) |
| `create_stripe_payment_intent` | Purchase a product using the authenticated user's saved payment method | `product_id` (required), `quantity` (optional) |

## Configuration

| Variable | Description |
|---|---|
| `STRIPE_SECRET_KEY` | Stripe API secret key |
| `PINGONE_AIC_ISSUER` | PingOne AIC OAuth 2.0 issuer URL (used for userinfo and discovery metadata) |
| `MCP_REQUIRED_SCOPES` | Space-separated OAuth scopes required by this MCP server (e.g. `email stripe_mcp:invoke`) |
| `MCP_SERVER_PORT` | Port to listen on (set to `8080` for Cloud Run) |

## Deployment

Deployed to Cloud Run via the Cloud Build pipeline at [`../deploy/gcp/cloudbuild.stripe-mcp.yaml`](../deploy/gcp/cloudbuild.stripe-mcp.yaml).

First, store the Stripe secret key in GCP Secret Manager:

```bash
echo -n "sk_live_..." | gcloud secrets create stripe-secret-key --data-file=-
```

**Trigger from repo root:**

```bash
gcloud builds submit \
  --config ingress-public-lb-mcp/deploy/gcp/cloudbuild.stripe-mcp.yaml \
  --substitutions \
    _PINGONE_AIC_ISSUER=https://your-aic-issuer,\
    _MCP_REQUIRED_SCOPES="stripe_mcp:invoke email"
```

The pipeline builds the Docker image, pushes it to Artifact Registry, and deploys to Cloud Run with `--ingress internal-and-cloud-load-balancing`. The Stripe secret is injected from Secret Manager at deploy time.

Copy `.env.sample` to `.env` and fill in values for local development.

## Files

| File | Responsibility |
|---|---|
| `main.go` | Entrypoint — wires config, MCP server, and HTTP listener |
| `http_routes.go` | Request router, bearer token auth, caller identity injection |
| `oauth_discovery.go` | OAuth/protected-resource discovery metadata (attended agents only) |
| `mcp_tools.go` | MCP tool definitions (list/get products, customer lookup, payments) |
| `stripe_client.go` | Stripe API client functions |
| `util.go` | Shared helpers — env loading, JSON conversion, caller identity resolution |
