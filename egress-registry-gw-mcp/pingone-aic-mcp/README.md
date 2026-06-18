# gw-pingone-aic-mcp

Go MCP server wrapping the **PingOne AIC (ForgeRock Identity Cloud)** managed
user REST API. Deployed as Cloud Run service `gw-pingone-aic-mcp` (internal
ingress); the Agent Gateway's ext_proc shim enforces PingAuthorize policy
before each request reaches this server.

## MCP Tools

| Tool | AIC API | Parameters |
|---|---|---|
| `provision_user` | `POST /openidm/managed/{realm}_user` | `username`*, `email`*, `password`*, `first_name`, `last_name` |
| `deprovision_user` | `DELETE /openidm/managed/{realm}_user/{id}` | `email`* |
| `update_user_status` | `PATCH /openidm/managed/{realm}_user/{id}` | `email`*, `enabled`* |
| `list_users` | `GET /openidm/managed/{realm}_user?_queryFilter=…` | `filter` |

\* required

## Authentication

Uses its own **admin OAuth client** (client credentials, `fr:idm:*`) to call
the AIC management API. Tokens are cached and auto-refreshed.

The caller's bearer token is validated by the ext_proc shim at the Agent
Gateway — this server trusts the gateway to enforce auth.

## OAuth Discovery

Serves `/.well-known/oauth-protected-resource` (RFC 9728) and
`/.well-known/oauth-authorization-server` (RFC 8414).

## Environment Variables

```
MCP_SERVER_PORT=8080
AIC_BASE_URL=                # https://openam-your-tenant.forgeblocks.com
AIC_ADMIN_CLIENT_ID=
AIC_ADMIN_CLIENT_SECRET=
AIC_REALM=alpha
MCP_REQUIRED_SCOPES=pingone:provisioning
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
  --config egress-registry-gw-mcp/pingone-aic-mcp/cloudbuild.yaml .
```

## AIC Admin Client Setup

Create an OAuth 2.0 client under **Realm → Applications → Clients** with:
- Grant type: `client_credentials`
- Scopes: `fr:idm:*`
