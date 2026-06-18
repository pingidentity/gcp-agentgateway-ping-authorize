"""Deploy ping-provisioner-agent to Vertex AI Agent Runtime.

Uses the agentplatform client (not vertexai.agent_engines) so we can pass
identity_type=AGENT_IDENTITY — required for the SPIFFE identity that the
Agent Gateway uses for egress IAM enforcement.

build_agent() is defined here (in __main__) so cloudpickle serialises the
full function bytecode rather than a module reference.

Usage:
    python deploy_agent.py \
        --project tech-partner-ping \
        --region us-central1 \
        --network-attachment projects/tech-partner-ping/regions/us-central1/networkAttachments/agent-gateway-attachment \
        --pingone-mcp-resource projects/tech-partner-ping/locations/us-central1/mcpServers/agentregistry-00000000-0000-0000-2638-c746341351e1 \
        --entra-mcp-resource   projects/tech-partner-ping/locations/us-central1/mcpServers/agentregistry-00000000-0000-0000-0664-717abde689d3
"""
import argparse
import os
import urllib.parse
import urllib.request

import vertexai
from vertexai.agent_engines import AdkApp
from google.adk.agents import LlmAgent
from google.adk.integrations.agent_registry import AgentRegistry
from google.adk.tools.mcp_tool.mcp_toolset import McpToolset
from google.adk.tools.mcp_tool.mcp_session_manager import StreamableHTTPConnectionParams
from google.genai import types as genai_types
import agentplatform
from agentplatform._genai import types as ap_types

AIC_TOKEN_ENDPOINT = (
    "https://openam-tntp-aiagents.forgeblocks.com"
    "/am/oauth2/realms/root/realms/alpha/access_token"
)


def _fetch_aic_token(client_id: str, client_secret: str, scope: str) -> str:
    import json
    body = urllib.parse.urlencode({
        "grant_type": "client_credentials",
        "client_id": client_id,
        "client_secret": client_secret,
        "scope": scope,
    }).encode()
    req = urllib.request.Request(
        AIC_TOKEN_ENDPOINT,
        data=body,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        data = json.loads(resp.read())
    token = data.get("access_token")
    if not token:
        raise RuntimeError(f"No access_token in AIC response: {data}")
    return token

SYSTEM_PROMPT = """\
You are an identity provisioning agent. You MUST use function calls to invoke tools.
NEVER write Python code, print() statements, or any executable code. ONLY use the
structured function call format to call tools.

You have access to tools for two identity systems:
- PingOne AIC (ForgeRock Identity Cloud): tools with the pingone_ prefix
- Microsoft Entra (Azure AD): tools with the entra_ prefix

When you receive a request:
1. Identify which system(s) to act on.
2. Invoke the appropriate tool directly using a function call.
3. Report the result, including any IDs or errors.

If a tool call is rejected or returns an error, report it and stop.
"""


def _exchange_token(ui_token: str) -> str:
    """RFC 8693 delegation: raw AIC UI token → delegated token.

    Called inside the agent at serve time. Reads EXCHANGE_CLIENT_SECRET from
    the environment (injected from Secret Manager at deploy time).
    sub=user, act.sub=gcp_ping_provision_agent, scope=fr:idm:* fr:idm:admin.
    """
    import json as _json
    import base64 as _b64

    client_id = os.environ.get("EXCHANGE_CLIENT_ID", "gcp_ping_provision_agent")
    client_secret = os.environ.get("EXCHANGE_CLIENT_SECRET", "")
    audience = os.environ.get("DELEGATED_TOKEN_AUDIENCE", client_id)
    scope = os.environ.get("DELEGATED_TOKEN_SCOPES", "fr:idm:* fr:idm:admin pingone:provisioning")

    if not client_secret:
        print("WARN: EXCHANGE_CLIENT_SECRET not set, using raw UI token", flush=True)
        return ui_token

    # Step 1: actor token (client_credentials)
    actor_body = urllib.parse.urlencode({
        "grant_type": "client_credentials",
        "client_id": client_id,
        "client_secret": client_secret,
    }).encode()
    actor_req = urllib.request.Request(
        AIC_TOKEN_ENDPOINT, data=actor_body,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )
    with urllib.request.urlopen(actor_req, timeout=10) as r:
        actor_token = _json.loads(r.read())["access_token"]

    # Step 2: RFC 8693 token exchange
    creds = _b64.b64encode(f"{client_id}:{client_secret}".encode()).decode()
    exchange_body = urllib.parse.urlencode({
        "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
        "subject_token": ui_token,
        "subject_token_type": "urn:ietf:params:oauth:token-type:access_token",
        "actor_token": actor_token,
        "actor_token_type": "urn:ietf:params:oauth:token-type:access_token",
        "requested_token_type": "urn:ietf:params:oauth:token-type:access_token",
        "audience": audience,
        "scope": scope,
    }).encode()
    exchange_req = urllib.request.Request(
        AIC_TOKEN_ENDPOINT, data=exchange_body,
        headers={
            "Content-Type": "application/x-www-form-urlencoded",
            "Authorization": f"Basic {creds}",
        },
        method="POST",
    )
    with urllib.request.urlopen(exchange_req, timeout=10) as r:
        delegated = _json.loads(r.read()).get("access_token", "")

    if delegated:
        try:
            p = delegated.split(".")[1]; p += "=" * (4 - len(p) % 4)
            c = _json.loads(_b64.urlsafe_b64decode(p))
            print(f"DELEGATED_TOKEN sub={c.get('sub')} act={c.get('act')} scope={c.get('scope')} suffix=...{delegated[-8:]}", flush=True)
        except Exception:
            pass

    return delegated or ui_token


def build_agent() -> LlmAgent:
    """Build the LlmAgent — called at serve time inside Agent Runtime."""
    from google.adk.agents.readonly_context import ReadonlyContext

    project = os.environ["GOOGLE_CLOUD_PROJECT"]
    location = os.environ.get("GOOGLE_CLOUD_LOCATION", "us-central1")
    model = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")
    pingone_resource = os.environ["PINGONE_MCP_RESOURCE"]
    entra_resource = os.environ["ENTRA_MCP_RESOURCE"]

    registry = AgentRegistry(project_id=project, location=location)

    def _get_mcp_url(resource: str) -> str:
        server = registry.get_mcp_server(resource)
        for iface in server.get("interfaces", []):
            if iface.get("url"):
                return iface["url"]
        raise ValueError(f"No URL found for MCP server: {resource}")

    pingone_url = _get_mcp_url(pingone_resource)
    entra_url = _get_mcp_url(entra_resource)

    def header_provider(ctx: ReadonlyContext) -> dict:
        # Session state carries the raw AIC UI token from the browser.
        # Exchange it for a delegated RFC 8693 token on first use.
        raw = ctx.state.get("bearer_token", "")
        if not raw:
            return {}
        # Check if it's already a delegated token (has act claim).
        try:
            import base64 as _b64, json as _j
            p = raw.split(".")[1]; p += "=" * (4 - len(p) % 4)
            if _j.loads(_b64.urlsafe_b64decode(p)).get("act"):
                return {"Authorization": f"Bearer {raw}"}
        except Exception:
            pass
        delegated = _exchange_token(raw)
        return {"Authorization": f"Bearer {delegated}"}

    pingone_tools = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=pingone_url),
        header_provider=header_provider,
        tool_name_prefix="pingone_",
    )

    entra_tools = McpToolset(
        connection_params=StreamableHTTPConnectionParams(url=entra_url),
        header_provider=header_provider,
        tool_name_prefix="entra_",
    )

    return LlmAgent(
        name="ping_provisioner",
        model=model,
        instruction=SYSTEM_PROMPT,
        tools=[pingone_tools, entra_tools],
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--project", required=True)
    parser.add_argument("--region", default="us-central1")
    parser.add_argument("--network-attachment", required=True,
                        help="Full URI of the PSC network attachment")
    parser.add_argument("--agent-gateway",
                        help="Agent Gateway resource name (projects/.../agentGateways/NAME); "
                             "enables AGENT_TO_ANYWHERE routing")
    parser.add_argument("--pingone-mcp-resource", required=True)
    parser.add_argument("--entra-mcp-resource", required=True)
    parser.add_argument("--gemini-model", default="gemini-2.5-flash")
    parser.add_argument("--min-instances", type=int, default=1)
    parser.add_argument("--exchange-client-id", default="gcp_ping_provision_agent")
    parser.add_argument("--exchange-client-secret", default=os.environ.get("EXCHANGE_CLIENT_SECRET", ""))
    parser.add_argument("--delegated-token-audience", default=os.environ.get("DELEGATED_TOKEN_AUDIENCE", ""))
    parser.add_argument("--delegated-token-scopes", default="fr:idm:* fr:idm:admin pingone:provisioning")
    args = parser.parse_args()

    env_vars = {
        "GEMINI_MODEL": args.gemini_model,
        "PINGONE_MCP_RESOURCE": args.pingone_mcp_resource,
        "ENTRA_MCP_RESOURCE": args.entra_mcp_resource,
        "EXCHANGE_CLIENT_ID": args.exchange_client_id,
        "EXCHANGE_CLIENT_SECRET": args.exchange_client_secret,
        "DELEGATED_TOKEN_AUDIENCE": args.delegated_token_audience,
        "DELEGATED_TOKEN_SCOPES": args.delegated_token_scopes,
    }

    os.environ.setdefault("GOOGLE_CLOUD_PROJECT", args.project)
    os.environ.setdefault("GOOGLE_CLOUD_LOCATION", args.region)
    for k, v in env_vars.items():
        os.environ.setdefault(k, v)

    staging_bucket = f"gs://{args.project}-agent-staging"
    vertexai.init(project=args.project, location=args.region, staging_bucket=staging_bucket)

    app = AdkApp(agent=build_agent())

    requirements = [
        "google-cloud-aiplatform[agent_engines,adk]>=1.112.0",
        "google-adk[agent-identity,a2a]>=1.0.0",
        "mcp>=1.0.0",
        "google-auth>=2.30.0",
        "google-api-core>=2.19.0",
    ]

    agent_gateway_name = args.agent_gateway or (
        f"projects/{args.project}/locations/{args.region}/agentGateways/ping-authz-agent-gateway"
    )

    print(f"Deploying ping-provisioner to Agent Runtime in {args.project}/{args.region} ...")
    print(f"  PSC network attachment : {args.network_attachment}")
    print(f"  Agent Gateway          : {agent_gateway_name}")
    print(f"  identity_type          : AGENT_IDENTITY")

    client = agentplatform.Client(project=args.project, location=args.region)

    from agentplatform._genai.types.common import (
        ReasoningEngineSpecDeploymentSpecAgentGatewayConfig,
        ReasoningEngineSpecDeploymentSpecAgentGatewayConfigAgentToAnywhereConfig,
    )
    agent_gateway_config = ReasoningEngineSpecDeploymentSpecAgentGatewayConfig(
        agent_to_anywhere_config=(
            ReasoningEngineSpecDeploymentSpecAgentGatewayConfigAgentToAnywhereConfig(
                agent_gateway=agent_gateway_name,
            )
        )
    )

    remote_agent = client.agent_engines.create(
        agent=app,
        config=ap_types.AgentEngineConfig(
            display_name="ping-provisioner-agent",
            description=(
                "Identity provisioning agent — routes MCP calls through "
                "Agent Gateway with PingAuthorize CONTENT_AUTHZ enforcement."
            ),
            staging_bucket=staging_bucket,
            requirements=requirements,
            env_vars=env_vars,
            min_instances=args.min_instances,
            identity_type=ap_types.IdentityType.AGENT_IDENTITY,
            agent_gateway_config=agent_gateway_config,
        ),
    )

    resource_name = remote_agent.api_resource.name
    spec = remote_agent.api_resource.model_dump().get("spec", {})
    effective_identity = spec.get("effective_identity", "")

    print(f"\nDeployed successfully.")
    print(f"  Resource name  : {resource_name}")
    if effective_identity:
        print(f"  Agent identity : {effective_identity}")
    else:
        print(f"  (effective_identity will appear once the engine is active)")

    print(f"\nNext — grant IAM egressor policy:")
    print(f"  python deploy/gcp/grant_egressor_iam.py \\")
    print(f"    --project {args.project} \\")
    print(f"    --region {args.region} \\")
    print(f"    --agent-resource {resource_name}")


if __name__ == "__main__":
    main()
