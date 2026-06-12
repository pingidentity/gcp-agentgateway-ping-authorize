# entra-mcp

Go MCP server wrapping the **Microsoft Graph API** for Microsoft Entra
(Azure AD) user provisioning. Deployed behind the internal Agent Gateway;
the ext_proc shim enforces PingAuthorize policy before each request reaches
this server.

Exposes the same MCP tool interface as `pingone-aic-mcp` for a uniform
provisioning experience across both identity systems.

## MCP Tools

| Tool | Graph API | Parameters |
|---|---|---|
| `provision_user` | `POST /v1.0/users` | `username`*, `email`*, `password`*, `first_name`, `last_name` |
| `deprovision_user` | `DELETE /v1.0/users/{id}` | `email`* |
| `update_user_status` | `PATCH /v1.0/users/{id}` | `email`*, `enabled`* |
| `list_users` | `GET /v1.0/users?$filter=…` | `filter` (OData) |

\* required

## Authentication

Uses Azure AD **client credentials** against
`https://login.microsoftonline.com/{tenantId}/oauth2/v2.0/token` with scope
`https://graph.microsoft.com/.default`. Tokens are cached and refreshed
automatically.

The caller's bearer token is validated by the ext_proc shim at the Agent
Gateway — this server trusts the gateway to enforce auth.

## OAuth Discovery

Serves `/.well-known/oauth-protected-resource` and
`/.well-known/oauth-authorization-server` pointing to the Azure AD tenant
endpoints.

## Environment Variables

```
MCP_SERVER_PORT=8080
AZURE_TENANT_ID=
AZURE_CLIENT_ID=
AZURE_CLIENT_SECRET=
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
docker build -t entra-mcp .
docker run -p 8080:8080 --env-file .env entra-mcp
```

## Azure App Registration Setup

The app registration needs:
- **API permissions**: `User.ReadWrite.All` (Application, Microsoft Graph)
- **Grant type**: Client credentials (no user sign-in)
- Grant admin consent for the directory

Create under **Azure Portal → Entra ID → App registrations → New registration**.
