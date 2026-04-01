/**
 * Token Validation
 *
 * Validates the user's PingOne AIC access token before allowing agent access.
 * Checks issuer, audience, required scope (stripe_mcp:invoke), and the may_act
 * claim that authorizes this agent to perform token exchange on behalf of the user.
 */

const EXPECTED_ISSUER = process.env.AIC_ISSUER || "https://openam-tntp-aiagents.forgeblocks.com:443/am/oauth2/alpha";
const EXPECTED_AUDIENCE = process.env.AGENT_CLIENT_ID || "ping-store-agent";
const REQUIRED_SCOPE = "stripe_mcp:invoke";

function decodeJwtPayload(token: string): Record<string, unknown> {
  const base64Url = token.split(".")[1];
  if (!base64Url) throw new Error("Invalid token format");
  const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
  return JSON.parse(Buffer.from(base64, "base64").toString());
}

export function validateSubjectToken(token: string): void {
  const claims = decodeJwtPayload(token);

  if (claims.iss !== EXPECTED_ISSUER) {
    throw new Error(`Invalid issuer: expected ${EXPECTED_ISSUER}, got ${claims.iss}`);
  }

  if (claims.aud !== EXPECTED_AUDIENCE) {
    throw new Error(`Invalid audience: expected ${EXPECTED_AUDIENCE}, got ${claims.aud}`);
  }

  const scopes = Array.isArray(claims.scope) ? claims.scope : String(claims.scope || "").split(" ");
  if (!scopes.includes(REQUIRED_SCOPE)) {
    throw new Error(`Missing required scope: ${REQUIRED_SCOPE}`);
  }

  const mayAct = claims.may_act as Record<string, string> | undefined;
  if (!mayAct || mayAct.client_id !== EXPECTED_AUDIENCE) {
    throw new Error(`Missing or invalid may_act claim: agent ${EXPECTED_AUDIENCE} is not authorized to act`);
  }
}
