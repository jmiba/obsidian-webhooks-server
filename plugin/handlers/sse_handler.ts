/**
 * sse_handler.ts - SSE (Server-Sent Events) handler for real-time webhook events
 *
 * Manages the EventSource connection to the webhook server for receiving
 * real-time event notifications. Handles connection lifecycle, automatic
 * reconnection on errors, and state change notifications.
 *
 * Key features:
 * - Real-time event delivery via SSE
 * - Automatic reconnection with configurable delays
 * - Connection state tracking and callbacks
 * - Graceful error handling
 * - Clean disconnection
 */

import type { WebhookEvent } from "../types";
import { getTunnelRequestHeaders } from "../utils/request-headers";

/**
 * Callback type for handling received events
 */
type EventCallback = (event: WebhookEvent) => Promise<void>;

/**
 * Callback type for connection state changes
 */
type StateChangeCallback = (state: string, message: string) => void;

/**
 * Configuration options for SSEHandler
 */
export interface SSEHandlerOptions {
	/** Whether to automatically reconnect on connection errors */
	autoReconnect?: boolean;

	/** Delay before attempting reconnection in milliseconds */
	reconnectDelayMs?: number;

	/** Maximum number of reconnection attempts (0 = unlimited) */
	maxReconnectAttempts?: number;
}

/**
 * Default configuration
 */
const DEFAULT_OPTIONS: Required<SSEHandlerOptions> = {
	autoReconnect: true,
	reconnectDelayMs: 3000,
	maxReconnectAttempts: 0, // unlimited
};

/**
 * SSEHandler manages Server-Sent Events connection to webhook server
 */
export class SSEHandler {
	private serverUrl: string;
	private clientKey: string;
	private onEvent: EventCallback;
	private onStateChange: StateChangeCallback;
	private options: Required<SSEHandlerOptions>;

	private eventSource: EventSource | null = null;
	private fetchAbortController: AbortController | null = null;
	private fetchConnected: boolean = false;
	private reconnectAttempts: number = 0;
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private isManualDisconnect: boolean = false;

	/**
	 * Create a new SSEHandler
	 *
	 * @param serverUrl - Base URL of the webhook server
	 * @param clientKey - Client authentication key
	 * @param onEvent - Callback to invoke for each received event
	 * @param onStateChange - Callback to invoke when connection state changes
	 * @param options - Optional configuration
	 */
	constructor(
		serverUrl: string,
		clientKey: string,
		onEvent: EventCallback,
		onStateChange: StateChangeCallback = () => {},
		options?: SSEHandlerOptions
	) {
		this.serverUrl = serverUrl;
		this.clientKey = clientKey;
		this.onEvent = onEvent;
		this.onStateChange = onStateChange;
		this.options = { ...DEFAULT_OPTIONS, ...options };
	}

	/**
	 * Establish SSE connection to the webhook server
	 *
	 * If already connected, the existing connection will be closed first.
	 * Creates an EventSource and sets up event handlers.
	 *
	 */
	connect(): void {
		// Close an existing transport before opening a replacement.
		this.closeCurrentConnection();

		// Mark as intentional connection
		this.isManualDisconnect = false;

		// Clear any pending reconnection attempts
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}

		// Notify connecting state
		this.onStateChange("connecting", "Establishing SSE connection...");

		try {
			// Build SSE endpoint URL
			const url = `${this.serverUrl}/events/${this.clientKey}`;
			const tunnelHeaders = getTunnelRequestHeaders(this.serverUrl);

			// Browser EventSource cannot attach request headers. Free ngrok endpoints
			// require a bypass header, so use a fetch-backed SSE stream there.
			if (Object.keys(tunnelHeaders).length > 0) {
				void this.connectWithFetch(url, tunnelHeaders);
				return;
			}

			// Create EventSource
			this.eventSource = new EventSource(url);

			// Set up event handlers
			this.setupEventHandlers();
		} catch (error) {
			const errorMsg =
				error instanceof Error ? error.message : String(error);
			this.onStateChange("error", `Connection failed: ${errorMsg}`);

			// Attempt reconnection if enabled
			if (this.options.autoReconnect) {
				this.scheduleReconnect();
			}
		}
	}

	/**
	 * Close the SSE connection
	 *
	 * Cleanly closes the EventSource and clears reconnection timers.
	 * Safe to call multiple times or when not connected.
	 *
	 * @returns Promise<void>
	 */
	disconnect(): void {
		// Mark as manual disconnect to prevent auto-reconnect
		this.isManualDisconnect = true;

		// Clear reconnection timer
		if (this.reconnectTimer) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}

		this.closeCurrentConnection();

		// Reset reconnect attempts
		this.reconnectAttempts = 0;

		// Notify disconnected state
		this.onStateChange("disconnected", "Connection closed");
	}

	/**
	 * Check if currently connected to SSE
	 *
	 * @returns true if connected, false otherwise
	 */
	isConnected(): boolean {
		return this.fetchConnected || (
			this.eventSource !== null &&
			this.eventSource.readyState === EventSource.OPEN
		);
	}

	private closeCurrentConnection(): void {
		if (this.eventSource) {
			this.eventSource.close();
			this.eventSource = null;
		}

		if (this.fetchAbortController) {
			const controller = this.fetchAbortController;
			this.fetchAbortController = null;
			controller.abort();
		}
		this.fetchConnected = false;
	}

	/**
	 * Open an SSE stream with fetch so tunnel-specific request headers can be
	 * included. EventSource does not expose a headers option.
	 */
	private async connectWithFetch(
		url: string,
		tunnelHeaders: Record<string, string>
	): Promise<void> {
		const controller = new AbortController();
		this.fetchAbortController = controller;

		try {
			const response = await globalThis.fetch(url, {
				method: "GET",
				headers: {
					Accept: "text/event-stream",
					...tunnelHeaders,
				},
				cache: "no-store",
				credentials: "omit",
				signal: controller.signal,
			});

			if (!response.ok) {
				throw new Error(`Server returned ${response.status}`);
			}
			if (!response.body) {
				throw new Error("Server returned no event stream");
			}
			if (controller !== this.fetchAbortController || this.isManualDisconnect) {
				return;
			}

			this.fetchConnected = true;
			this.reconnectAttempts = 0;
			this.onStateChange("connected", "SSE connection established");

			await this.consumeFetchStream(response.body, controller);
			if (!controller.signal.aborted) {
				throw new Error("Event stream closed");
			}
		} catch (error) {
			if (
				controller.signal.aborted ||
				this.isManualDisconnect ||
				controller !== this.fetchAbortController
			) {
				return;
			}

			this.fetchConnected = false;
			this.fetchAbortController = null;
			const message = error instanceof Error ? error.message : String(error);
			this.onStateChange("error", `SSE connection failed: ${message}`);

			if (this.options.autoReconnect) {
				this.scheduleReconnect();
			}
		}
	}

	private async consumeFetchStream(
		stream: ReadableStream<Uint8Array>,
		controller: AbortController
	): Promise<void> {
		const reader = stream.getReader();
		const decoder = new TextDecoder();
		let buffer = "";

		try {
			while (!controller.signal.aborted) {
				const { done, value } = await reader.read();
				if (done) {
					break;
				}

				buffer += decoder.decode(value, { stream: true });
				let boundary = buffer.match(/\r?\n\r?\n/);
				while (boundary?.index !== undefined) {
					const eventBlock = buffer.slice(0, boundary.index);
					buffer = buffer.slice(boundary.index + boundary[0].length);
					await this.handleFetchEventBlock(eventBlock);
					boundary = buffer.match(/\r?\n\r?\n/);
				}
			}
		} finally {
			reader.releaseLock();
		}
	}

	private async handleFetchEventBlock(eventBlock: string): Promise<void> {
		const data = eventBlock
			.split(/\r?\n/)
			.filter((line) => line.startsWith("data:"))
			.map((line) => line.slice(5).replace(/^ /, ""))
			.join("\n");

		// Heartbeats and comments do not contain a data field.
		if (!data) {
			return;
		}

		try {
			const webhookEvent = JSON.parse(data) as WebhookEvent;
			await this.onEvent(webhookEvent);
		} catch (error) {
			console.error(
				"Error parsing SSE event:",
				error instanceof Error ? error.message : error
			);
		}
	}

	/**
	 * Set up event handlers for EventSource
	 *
	 * Handles onopen, onmessage, and onerror events from the EventSource.
	 */
	private setupEventHandlers(): void {
		if (!this.eventSource) {
			return;
		}

		// Connection opened
		this.eventSource.onopen = () => {
			// Reset reconnect attempts on successful connection
			this.reconnectAttempts = 0;

			// Notify connected state
			this.onStateChange("connected", "SSE connection established");
		};

		// Message received
		this.eventSource.onmessage = async (event: MessageEvent) => {
			try {
				// Parse event data as JSON
				const webhookEvent = JSON.parse(event.data as string) as WebhookEvent;

				// Invoke event callback
				await this.onEvent(webhookEvent);
			} catch (error) {
				// Log parsing errors but don't disconnect
				console.error(
					"Error parsing SSE event:",
					error instanceof Error ? error.message : error
				);
			}
		};

		// Error occurred
		this.eventSource.onerror = (_error: Event) => {
			// Notify error state
			this.onStateChange("error", "SSE connection error occurred");

			// Close the current connection
			if (this.eventSource) {
				this.eventSource.close();
				this.eventSource = null;
			}

			// Attempt reconnection if not manually disconnected and auto-reconnect is enabled
			if (!this.isManualDisconnect && this.options.autoReconnect) {
				this.scheduleReconnect();
			}
		};
	}

	/**
	 * Schedule a reconnection attempt
	 *
	 * Uses exponential backoff if multiple attempts are made.
	 * Respects maxReconnectAttempts if configured.
	 */
	private scheduleReconnect(): void {
		// Check if we've exceeded max attempts
		if (
			this.options.maxReconnectAttempts > 0 &&
			this.reconnectAttempts >= this.options.maxReconnectAttempts
		) {
			this.onStateChange(
				"error",
				`Max reconnection attempts (${this.options.maxReconnectAttempts}) reached`
			);
			return;
		}

		// Increment attempt counter
		this.reconnectAttempts++;

		// Calculate delay with exponential backoff (cap at 30 seconds)
		const delay = Math.min(
			this.options.reconnectDelayMs * Math.pow(2, this.reconnectAttempts - 1),
			30000
		);

		// Notify reconnecting state
		this.onStateChange(
			"connecting",
			`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})...`
		);

		// Schedule reconnection
		this.reconnectTimer = setTimeout(() => {
			this.reconnectTimer = null;
			void this.connect();
		}, delay);
	}
}
