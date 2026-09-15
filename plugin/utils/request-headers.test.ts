import { describe, expect, test } from "bun:test";
import { getTunnelRequestHeaders } from "./request-headers";

describe("getTunnelRequestHeaders", () => {
	test("adds the browser warning bypass for current ngrok free domains", () => {
		expect(
			getTunnelRequestHeaders("https://example.ngrok-free.dev")
		).toEqual({
			"ngrok-skip-browser-warning": "1",
		});
		expect(
			getTunnelRequestHeaders("https://example.ngrok-free.app")
		).toEqual({ "ngrok-skip-browser-warning": "1" });
	});

	test("supports legacy ngrok domains", () => {
		expect(getTunnelRequestHeaders("https://example.ngrok.io")).toEqual({
			"ngrok-skip-browser-warning": "1",
		});
	});

	test("does not add tunnel headers to ordinary self-hosted servers", () => {
		expect(getTunnelRequestHeaders("https://webhooks.example.com")).toEqual({});
	});

	test("leaves invalid URLs to the request error path", () => {
		expect(getTunnelRequestHeaders("not a URL")).toEqual({});
	});
});
