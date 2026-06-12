"""Configuration loaded from environment variables."""
import os


def _require(key: str) -> str:
    val = os.getenv(key, "")
    if not val:
        raise EnvironmentError(f"Required environment variable {key!r} is not set")
    return val


class Settings:
    google_cloud_project: str = ""
    google_cloud_location: str = "us-central1"
    agent_port: int = 3000
    gemini_model: str = "gemini-2.0-flash"
    # Direct MCP server URLs — the Agent Gateway (ping-authz-agent-gateway)
    # is registered in GCP Agent Registry and its authz policy is active.
    # When google-adk[agent-identity] goes GA, replace these two env vars
    # with AGENT_GATEWAY_URL and swap agent.py back to AgentRegistry.get_mcp_toolset().
    pingone_aic_mcp_url: str = ""
    entra_mcp_url: str = ""

    def load(self) -> None:
        self.google_cloud_project = _require("GOOGLE_CLOUD_PROJECT")
        self.google_cloud_location = os.getenv("GOOGLE_CLOUD_LOCATION", "us-central1")
        self.agent_port = int(os.getenv("AGENT_PORT", "3000"))
        self.gemini_model = os.getenv("GEMINI_MODEL", "gemini-2.0-flash")
        self.pingone_aic_mcp_url = _require("PINGONE_AIC_MCP_URL")
        self.entra_mcp_url = _require("ENTRA_MCP_URL")


settings = Settings()
