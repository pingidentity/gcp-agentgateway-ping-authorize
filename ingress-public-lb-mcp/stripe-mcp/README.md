# lb-stripe-mcp

Go MCP server exposing Stripe tools for product browsing and payment
processing. Deployed as Cloud Run service `lb-stripe-mcp` (internal ingress);
only reachable via the regional load balancer after `lb-ping-authz-shim` has
permitted the request.

## MCP Tools

| Tool | Description | Parameters |
|---|---|---|
| `list_stripe_products` | List all active products with prices | — |
| `get_stripe_product` | Get details for a specific product | `product_id`* |
| `get_stripe_customer` | Look up the authenticated user's Stripe customer and saved payment method | — |
| `create_stripe_payment_intent` | Charge the user's saved card | `product_id`*, `quantity`, `total_price`*, `currency`* |

\* required

## Authentication

Caller identity is resolved by calling AIC's `/userinfo` endpoint with the
bearer token to get the user's email, then looking up the matching Stripe
customer. The load balancer and `lb-ping-authz-shim` validate the token
before the request reaches this server.

## OAuth Discovery

Serves `/.well-known/oauth-protected-resource` (RFC 9728) and
`/.well-known/oauth-authorization-server` (RFC 8414). These endpoints are
passed through unauthenticated for MCP clients that need to discover the
authorization server (e.g. Claude Desktop via `mcp-remote`).

## Environment Variables

```
MCP_SERVER_PORT=8080
STRIPE_SECRET_KEY=               # Stripe API secret key (injected from Secret Manager)
PINGONE_AIC_ISSUER=              # AIC issuer URL (for /userinfo and discovery metadata)
MCP_REQUIRED_SCOPES=stripe_mcp:invoke email
```

## Local Development

```bash
cp .env.sample .env
export $(cat .env | xargs)
go run .
```

## Deploy

Store the Stripe key in Secret Manager first:
```bash
printf "sk_live_..." | gcloud secrets create stripe-secret-key --data-file=-
```

```bash
gcloud builds submit \
  --config ingress-public-lb-mcp/stripe-mcp/cloudbuild.yaml .
```
