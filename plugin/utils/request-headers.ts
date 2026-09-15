/**
 * Headers required by tunnel providers in front of the webhook server.
 *
 * ngrok's free HTTP endpoints inject a browser interstitial unless the client
 * explicitly identifies the request as programmatic. Limit the header to
 * known ngrok hostnames so ordinary self-hosted servers do not receive an
 * unexpected custom header.
 */
export function getTunnelRequestHeaders(serverUrl: string): Record<string, string> {
	try {
		const hostname = new URL(serverUrl).hostname.toLowerCase();
		if (
			hostname.endsWith(".ngrok-free.dev") ||
			hostname.endsWith(".ngrok-free.app") ||
			hostname.endsWith(".ngrok.app") ||
			hostname.endsWith(".ngrok.io")
		) {
			return { "ngrok-skip-browser-warning": "1" };
		}
	} catch {
		// Invalid URLs are handled by the caller's request error path.
	}

	return {};
}
