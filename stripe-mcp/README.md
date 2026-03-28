# stripe-mcp

MCP server that exposes Stripe tools for listing products, looking up customers, and processing payments. Runs behind the Agent Gateway where the `ping-authz-shim` enforces authorization on every request before it reaches this server. OAuth discovery requests (`/.well-known/`) are passed through unauthenticated so MCP clients can bootstrap the authorization flow.

```
MCP Client → Agent Gateway → ping-authz-shim (allow/deny) → stripe-mcp
```

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
| `PING_AIC_ISSUER` | PingOne AIC OAuth 2.0 issuer URL (used for userinfo and discovery metadata) |
| `OAUTH_SCOPES` | Space-separated OAuth scopes advertised in discovery metadata (e.g. `openid profile email stripe_mcp:invoke`) |

## Files

| File | Responsibility |
|---|---|
| `main.go` | HTTP server bootstrap and config |
| `routes.go` | Request router (OAuth discovery + MCP handler) |
| `oauth.go` | OAuth discovery metadata and userinfo resolution |
| `tools.go` | MCP tool definitions (list/get products, customer lookup, payments) |
| `stripe_client.go` | Stripe API client functions |
