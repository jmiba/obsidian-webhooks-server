export interface PendingEvent {
	id: string;
	path: string;
}

export interface ClearQueueResult {
	acknowledged: number;
	failedId?: string;
}

/** Acknowledge the reviewed snapshot in order; stop if any request fails. */
export async function clearPendingSnapshot(
	events: PendingEvent[],
	acknowledge: (id: string) => Promise<boolean>
): Promise<ClearQueueResult> {
	let acknowledged = 0;
	for (const event of events) {
		try {
			if (!await acknowledge(event.id)) {
				return { acknowledged, failedId: event.id };
			}
		} catch {
			return { acknowledged, failedId: event.id };
		}
		acknowledged++;
	}
	return { acknowledged };
}
