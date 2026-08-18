import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function alzetteProvider(pi: ExtensionAPI) {
  const baseUrl = process.env.ALZETTE_PI_PROXY_URL;
  const sessionKey = process.env.ALZETTE_PI_PROXY_KEY;
  if (!baseUrl || !sessionKey) {
    throw new Error("Start Pi with `alzette-agent pi`; direct loading is not supported.");
  }

  let aliases: string[] = [];
  try {
    const parsed = JSON.parse(process.env.ALZETTE_PI_MODELS ?? "[]");
    if (Array.isArray(parsed)) aliases = parsed.filter((value): value is string => typeof value === "string");
  } catch {
    throw new Error("Alzette model access is invalid.");
  }
  if (aliases.length === 0) throw new Error("No Alzette models are assigned to this employee.");

  pi.registerProvider("alzette-employee", {
	name: "Alzette employee access",
    baseUrl,
    apiKey: "$ALZETTE_PI_PROXY_KEY",
    authHeader: true,
    api: "openai-completions",
    models: aliases.map((id) => ({
      id,
      name: id,
      reasoning: false,
      input: ["text"],
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
      contextWindow: 128000,
      maxTokens: 32768,
      compat: {
        supportsDeveloperRole: false,
        supportsUsageInStreaming: false,
		supportsStore: false,
        maxTokensField: "max_tokens",
      },
    })),
  });
}
