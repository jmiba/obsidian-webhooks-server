import { describe, expect, test } from "bun:test";
import { clearPendingSnapshot } from "./clear-pending";

const events = [
	{ id: "first", path: "A.md" },
	{ id: "second", path: "B.md" },
	{ id: "third", path: "C.md" },
];

describe("clearPendingSnapshot", () => {
	test("acknowledges only the reviewed events in order", async () => {
		const seen: string[] = [];
		const result = await clearPendingSnapshot(events.slice(0, 2), async (id) => {
			seen.push(id);
			return true;
		});

		expect(seen).toEqual(["first", "second"]);
		expect(result).toEqual({ acknowledged: 2 });
	});

	test("stops at a failed acknowledgement and reports partial progress", async () => {
		const seen: string[] = [];
		const result = await clearPendingSnapshot(events, async (id) => {
			seen.push(id);
			return id !== "second";
		});

		expect(seen).toEqual(["first", "second"]);
		expect(result).toEqual({ acknowledged: 1, failedId: "second" });
	});
});
