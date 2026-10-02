import { requestUrl } from "obsidian";
import { ACKHandler } from "./ack_handler";
import { getTunnelRequestHeaders } from "../utils/request-headers";
import { clearPendingSnapshot, type ClearQueueResult, type PendingEvent } from "../utils/clear-pending";

/** Manual queue management only; fetching here never writes event data to the vault. */
export class PendingQueue {
	private ackHandler: ACKHandler;

	constructor(private serverUrl: string, private clientKey: string) {
		this.ackHandler = new ACKHandler(serverUrl, clientKey, { maxRetries: 0 });
	}

	async list(): Promise<PendingEvent[]> {
		if (!this.clientKey.trim()) {
			throw new Error("Configure a client key first");
		}

		const response = await requestUrl({
			url: `${this.serverUrl}/events/${this.clientKey}?poll=true`,
			method: "GET",
			headers: getTunnelRequestHeaders(this.serverUrl),
			throw: false,
		});
		if (response.status < 200 || response.status >= 300) {
			throw new Error(`Could not load pending events (HTTP ${response.status})`);
		}

		const body: unknown = response.json;
		if (!Array.isArray(body) || !body.every((event: unknown) =>
			typeof event === "object" && event !== null &&
			typeof (event as PendingEvent).id === "string" &&
			typeof (event as PendingEvent).path === "string"
		)) {
			throw new Error("Server returned an invalid pending event list");
		}

		// Keep only identifiers and paths; the polling response also contains payload data.
		return body.map((event: PendingEvent) => ({ id: event.id, path: event.path }));
	}

	/** Acknowledge only the reviewed snapshot. Stop at the first failure. */
	async clear(events: PendingEvent[]): Promise<ClearQueueResult> {
		return clearPendingSnapshot(events, (id) => this.ackHandler.acknowledgeEvent(id));
	}
}
