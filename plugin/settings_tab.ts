/**
 * settings_tab.ts - Plugin Settings UI (v3.0 - Simplified)
 *
 * Implements Progressive Disclosure principle:
 * - Only 2 essential fields visible by default
 * - Advanced settings collapsed
 * - Smart defaults that "just work"
 */

import { App, Modal, Notice, PluginSettingTab, Setting, requestUrl } from "obsidian";
import type ObsidianWebhooksPlugin from "./main";
import type { WebhookSettings } from "./types";
import { getTunnelRequestHeaders } from "./utils/request-headers";
import { PendingQueue } from "./handlers/pending_queue";
import type { PendingEvent } from "./utils/clear-pending";

class PendingQueueModal extends Modal {
	private queue: PendingQueue;
	private events: PendingEvent[] = [];
	private busy = false;

	constructor(app: App, serverUrl: string, clientKey: string) {
		super(app);
		this.queue = new PendingQueue(serverUrl, clientKey);
	}

	onOpen(): void {
		void this.load();
	}

	private async load(): Promise<void> {
		if (this.busy) return;
		this.busy = true;
		this.contentEl.empty();
		this.contentEl.createEl("h2", { text: "Pending webhook events" });
		this.contentEl.createEl("p", { text: "Loading pending events..." });
		try {
			this.events = await this.queue.list();
			this.render();
		} catch (error) {
			this.contentEl.empty();
			this.contentEl.createEl("h2", { text: "Pending webhook events" });
			this.contentEl.createEl("p", {
				text: error instanceof Error ? error.message : "Could not load pending events",
			});
			new Setting(this.contentEl).addButton((button) => button
				.setButtonText("Retry")
				.onClick(() => void this.load()));
		} finally {
			this.busy = false;
		}
	}

	private render(message?: string): void {
		this.contentEl.empty();
		this.contentEl.createEl("h2", { text: "Pending webhook events" });
		if (message) this.contentEl.createEl("p", { text: message });
		this.contentEl.createEl("p", {
			text: `${this.events.length} pending event${this.events.length === 1 ? "" : "s"}.`,
		});
		if (this.events.length > 0) {
			const list = this.contentEl.createEl("ul");
			for (const event of this.events.slice(0, 20)) {
				list.createEl("li", { text: event.path });
			}
			if (this.events.length > 20) {
				this.contentEl.createEl("p", { text: `Showing the first 20 of ${this.events.length} paths.` });
			}
			this.contentEl.createEl("p", {
				text: "Clearing acknowledges these events without writing them to the vault. They will no longer be delivered. New events are unaffected.",
			});
		}
		new Setting(this.contentEl)
			.addButton((button) => button.setButtonText("Refresh").onClick(() => void this.load()))
			.addButton((button) => {
				button.setButtonText(`Clear ${this.events.length} pending`)
					.setWarning()
					.setDisabled(this.events.length === 0)
					.onClick(() => void this.clear(button));
			});
	}

	private async clear(button: { setDisabled(disabled: boolean): unknown; setButtonText(text: string): unknown }): Promise<void> {
		if (this.busy || this.events.length === 0) return;
		this.busy = true;
		button.setDisabled(true);
		button.setButtonText("Clearing...");
		const count = this.events.length;
		const result = await this.queue.clear(this.events);
		this.events = this.events.slice(result.acknowledged);
		try {
			this.events = await this.queue.list();
			this.render(result.failedId
				? `Acknowledged ${result.acknowledged} of ${count}; stopped after a failed request. ${this.events.length} remain pending.`
				: `Acknowledged ${result.acknowledged} event${result.acknowledged === 1 ? "" : "s"}. ${this.events.length} remain pending.`);
		} catch (error) {
			this.render(`Acknowledged ${result.acknowledged} of ${count}. Could not refresh the remaining queue: ${error instanceof Error ? error.message : String(error)}`);
		} finally {
			this.busy = false;
		}
	}
}

export class WebhookSettingTab extends PluginSettingTab {
	plugin: ObsidianWebhooksPlugin;
	private advancedExpanded = false;
	private statusBannerEl: HTMLElement | null = null;
	private statusTextEl: HTMLElement | null = null;

	constructor(app: App, plugin: ObsidianWebhooksPlugin) {
		super(app, plugin);
		this.plugin = plugin;
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();

		// Status indicator at the top (most important)
		this.renderStatusIndicator(containerEl);

		// Client Key (only essential setting)
		this.renderClientKey(containerEl);

		// Test Connection button
		this.renderTestConnection(containerEl);

		// Manual queue recovery
		new Setting(containerEl)
			.setName("Pending events")
			.setDesc("Review and clear events that keep failing")
			.addButton((button) => button
				.setButtonText("Review queue")
				.onClick(() => {
					new PendingQueueModal(this.app, this.plugin.settings.serverUrl, this.plugin.settings.clientKey).open();
				}));

		// Advanced Settings (collapsed by default)
		this.renderAdvancedSettings(containerEl);
	}

	private renderStatusIndicator(containerEl: HTMLElement): void {
		this.statusBannerEl = containerEl.createDiv({
			cls: "webhook-status-banner",
		});
		this.statusTextEl = this.statusBannerEl.createEl("span", {
			cls: "webhook-status-text",
		});
		this.updateConnectionStatus();
	}

	updateConnectionStatus(): void {
		if (!this.statusBannerEl || !this.statusTextEl) {
			return;
		}

		const status = this.plugin.connectionStatus;
		let className = "webhook-status-error";
		let message = status.message;

		if (status.state === "connected") {
			className = "webhook-status-connected";
			message = "Connected";
		} else if (status.state === "connecting" || status.message.includes("Reconnecting")) {
			className = "webhook-status-warning";
		}

		this.statusBannerEl.className = `webhook-status-banner ${className}`;
		this.statusTextEl.textContent = message;
	}

	private renderClientKey(containerEl: HTMLElement): void {
		new Setting(containerEl)
			.setName("Client key")
			.setDesc("Get your key from the dashboard")
			.addText((text) =>
				text
					.setPlaceholder("Paste your client key")
					.setValue(this.plugin.settings.clientKey)
					.onChange(async (value) => {
						this.plugin.settings.clientKey = value.trim();
						await this.plugin.saveSettings();
					})
			);
	}

	private renderTestConnection(containerEl: HTMLElement): void {
		const setting = new Setting(containerEl)
			.setName("Test connection")
			.setDesc("Verify your client key and server connection")
			.addButton((button) => {
				button
					.setButtonText("Test")
					.onClick(async () => {
						button.setDisabled(true);
						button.setButtonText("Testing...");
						await this.testConnection(setting.settingEl);
						button.setDisabled(false);
						button.setButtonText("Test");
					});
			});
	}

	private async testConnection(settingEl: HTMLElement): Promise<void> {
		// Remove previous result if exists (search in parent since result is a sibling)
		const parent = settingEl.parentElement;
		const existingResult = parent?.querySelector(".webhook-test-result");
		if (existingResult) {
			existingResult.remove();
		}

		const clientKey = this.plugin.settings.clientKey;
		if (!clientKey) {
			const resultDiv = this.createResultDiv(settingEl, "test-error");
			resultDiv.textContent = "Enter your client key first";
			return;
		}

		const resultDiv = this.createResultDiv(settingEl);

		try {
			const startTime = Date.now();
			const response = await requestUrl({
				url: `${this.plugin.settings.serverUrl}/test/${clientKey}`,
				method: "POST",
				headers: getTunnelRequestHeaders(this.plugin.settings.serverUrl),
				throw: false,
			});

			if (response.status < 200 || response.status >= 300) {
				const errorMsg = response.status === 401 || response.status === 404
					? "Invalid client key"
					: `Server error: ${response.status}`;
				resultDiv.textContent = `Test failed: ${errorMsg}`;
				resultDiv.classList.add("test-error");
				return;
			}

			const latency = Date.now() - startTime;
			resultDiv.textContent = `Test passed! Server responded in ${latency}ms`;
			resultDiv.classList.add("test-success");

			// Rebuild handlers with the current settings and establish the live connection.
			await this.plugin.reconnectWithCurrentSettings();

			// Auto-hide result after 10 seconds
			setTimeout(() => {
				resultDiv.remove();
			}, 10000);
		} catch (error) {
			const errorMsg = error instanceof Error ? error.message : "Network error";
			resultDiv.textContent = `Test failed: ${errorMsg}`;
			resultDiv.classList.add("test-error");
		}
	}

	private createResultDiv(settingEl: HTMLElement, ...classes: string[]): HTMLElement {
		const div = document.createElement("div");
		div.className = ["webhook-test-result", ...classes].join(" ");
		settingEl.insertAdjacentElement("afterend", div);
		return div;
	}

	private renderAdvancedSettings(containerEl: HTMLElement): void {
		// Collapsible Advanced Settings section
		new Setting(containerEl)
			.setName("Advanced")
			.setHeading()
			.addExtraButton((button) => {
				button
					.setIcon(this.advancedExpanded ? "chevron-down" : "chevron-right")
					.setTooltip(this.advancedExpanded ? "Collapse" : "Expand")
					.onClick(() => {
						this.advancedExpanded = !this.advancedExpanded;
						this.display(); // Re-render
					});
			});

		if (!this.advancedExpanded) {
			return;
		}

		// Server URL (for self-hosted setups)
		new Setting(containerEl)
			.setName("Server URL")
			.setDesc("Change only for self-hosted setup")
			.addText((text) =>
				text
					.setPlaceholder("https://your-server.example.com")
					.setValue(this.plugin.settings.serverUrl)
					.onChange(async (value) => {
						// Normalize: remove trailing slash
						this.plugin.settings.serverUrl = value.trim().replace(/\/$/, "");
						await this.plugin.saveSettings();
					})
			);

		// Auto-reconnect
		new Setting(containerEl)
			.setName("Auto-reconnect")
			.setDesc("Automatically connect when plugin loads")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.autoConnect)
					.onChange(async (value) => {
						this.plugin.settings.autoConnect = value;
						await this.plugin.saveSettings();
					})
			);

		// Write mode
		new Setting(containerEl)
			.setName("Write mode")
			.setDesc("How to write content to files")
			.addDropdown((dropdown) =>
				dropdown
					.addOption("append", "Append to end")
					.addOption("overwrite", "Overwrite file")
					.addOption("frontmatter", "Update YAML frontmatter")
					.setValue(this.plugin.settings.defaultMode)
					.onChange(async (value) => {
						this.plugin.settings.defaultMode =
							value as WebhookSettings["defaultMode"];
						await this.plugin.saveSettings();
					})
			);

		// Debug logging
		new Setting(containerEl)
			.setName("Debug logging")
			.setDesc("Enable console output for troubleshooting")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.enableDebugLogging)
					.onChange(async (value) => {
						this.plugin.settings.enableDebugLogging = value;
						await this.plugin.saveSettings();
					})
			);

		// Quick actions
		new Setting(containerEl)
			.setName("Quick actions")
			.setHeading();

		// Open Dashboard button
		new Setting(containerEl)
			.setName("User dashboard")
			.setDesc("View your keys and webhook logs")
			.addButton((button) => {
				button
					.setButtonText("Open dashboard")
					.onClick(() => {
						const clientKey = this.plugin.settings.clientKey;
						if (!clientKey) {
							new Notice("Please configure your client key first");
							return;
						}
						window.open(`${this.plugin.settings.serverUrl}/dashboard`, "_blank");
					});
			});

	}

}
