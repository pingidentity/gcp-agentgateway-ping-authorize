# pingone-aic-mcp

Go MCP server wrapping the **PingOne AIC (ForgeRock Identity Cloud)** managed
user REST API. Deployed behind the internal Agent Gateway; the ext_proc shim
enforces PingAuthorize policy before each request reaches this server.

## MCP Tools

| Tool | AIC API | Parameters |
|---|---|---|
| `provision_user` | `POST /openidm/managed/{realm}_user` | `username`*, `email`*, `password`*, `first_name`, `last_name` |
| `deprovision_user` | `DELETE /openidm/managed/{realm}_user/{id}` | `email`* |
| `update_user_status` | `PATCH /openidm/managed/{realm}_user/{id}` | `email`*, `enabled`* |
| `list_users` | `GET /openidm/managed/{realm}_user?_queryFilter=…` | `filter` |

\* required

## Authentication

The server uses its own **admin OAuth client** (client credentials, scope
`fr:idm:*`) to authenticate to the AIC management API. Tokens are cached and
refreshed automatically (60-second safety margin).

The **caller's** bearer token (from the agent) is validated by the ext_proc
shim at the Agent Gateway — this server trusts the gateway to enforce auth.

## OAuth Discovery

Serves `/.well-known/oauth-protected-resource` (RFC 9728) and
`/.well-known/oauth-authorization-server` (RFC 8414) for MCP client
bootstrapping.

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
# Edit .env
export $(cat .env | xargs)
go run .
```

## Docker

```bash
docker build -t pingone-aic-mcp .
docker run -p 8080:8080 --env-file .env pingone-aic-mcp
```

## AIC Admin Client Setup

The admin client needs read/write access to managed identities:
- Grant type: `client_credentials`
- Scopes: `fr:idm:*` (or `fr:idm:admin`)
- Create under **Realm → Applications → OAuth 2.0 → Clients** in AIC console
