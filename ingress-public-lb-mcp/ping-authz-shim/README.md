# lb-ping-authz-shim

Go gRPC service implementing Envoy's `ext_proc` protocol. The regional load
balancer calls this service on every inbound `/mcp` request; the shim
extracts policy attributes from the request and consults PingAuthorize for
an allow/deny decision before the request reaches `lb-stripe-mcp`.

Deployed as Cloud Run service `lb-ping-authz-shim` (internal ingress, HTTP/2).

## Request Phases

**Phase 1 — Request headers:** fast-path decisions: passthrough for
`/.well-known/*`, 404 for unknown paths, 401 for missing bearer token.
Authenticated `/mcp` requests proceed to phase 2.

**Phase 2 — Request body:** parses the MCP JSON-RPC body, extracts tool
name and arguments, and calls PingAuthorize with the full attribute set.

## Policy Attributes

```json
{
  "attributes": {
    "access_token": "<delegated bearer token>",
    ":path": "/mcp",
    ":method": "POST",
    "mcp_method": "tools/call",
    "mcp_tool_name": "create_stripe_payment_intent",
    "mcp_product_id": "prod_abc123",
    "mcp_quantity": "1",
    "mcp_total_price": "29.99",
    "mcp_currency": "usd"
  }
}
```

## Environment Variables

```
SHIM_SERVER_PORT=8080
PING_AUTHORIZE_URL=               # PingAuthorize governance engine endpoint
MCP_SERVER_URL=                   # Load balancer URL (used in WWW-Authenticate)
PING_AUTHORIZE_SKIP_TLS_VERIFY=false
MCP_REQUIRED_SCOPES=openid profile email stripe_mcp:invoke
```

## Local Development

```bash
cp .env.sample .env
export $(cat .env | xargs)
go run .
```

## Deploy

```bash
gcloud builds submit \
  --config ingress-public-lb-mcp/ping-authz-shim/cloudbuild.yaml .
```
