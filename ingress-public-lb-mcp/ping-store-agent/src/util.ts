/** Response from an RFC 8693 token exchange grant. */
export interface TokenExchangeResult {
  access_token: string;
  token_type: string;
  expires_in: number;
}

/** Incoming request body for the POST /chat endpoint. */
export interface ChatRequest {
  message: string;
}

/** Error with an associated HTTP status code. */
export class HttpError extends Error {
  constructor(public readonly statusCode: number, message: string) {
    super(message);
  }
}

/** Returns the value of a required environment variable or throws if missing. */
export const requiredEnv = (name: string): string => {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
};

/** Extracts the Bearer token from a request's Authorization header or throws 401. */
export const extractBearerToken = (req: { headers: { authorization?: string } }): string => {
  const authHeader = req.headers.authorization;
  if (!authHeader?.startsWith("Bearer "))
    throw new HttpError(401, "Missing Bearer token");
  return authHeader.slice(7);
};

/** Builds a Basic auth header value from client credentials. */
export const basicAuthHeader = (clientId: string, clientSecret: string): string => {
  return `Basic ${Buffer.from(`${clientId}:${clientSecret}`).toString("base64")}`;
};

/** Extracts plain text from a Strands agent result. */
export const extractAgentResponseText = (result: any): string => {
  if (typeof result === "string") return result;
  if (result?.lastMessage?.content) {
    const blocks = result.lastMessage.content as any[];
    return blocks.filter((b: any) => b.text).map((b: any) => b.text).join("\n");
  }
  return JSON.stringify(result);
};
