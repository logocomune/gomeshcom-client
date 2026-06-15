const TOAST_DURATION_MS = 30_000;

type DmToast = {
	id: number;
	kind: 'dm';
	from: string;
};

type MentionToast = {
	id: number;
	kind: 'mention';
	from: string;
	channel: string;
};

type AppToast = DmToast | MentionToast;

class ToastStore {
	toasts = $state<AppToast[]>([]);
	private nextId = 0;

	addDm(from: string) {
		const id = this.nextId++;
		this.toasts = [...this.toasts, { id, kind: 'dm', from }];
		setTimeout(() => this.dismiss(id), TOAST_DURATION_MS);
	}

	addMention(from: string, channel: string) {
		const id = this.nextId++;
		this.toasts = [...this.toasts, { id, kind: 'mention', from, channel }];
		setTimeout(() => this.dismiss(id), TOAST_DURATION_MS);
	}

	dismiss(id: number) {
		this.toasts = this.toasts.filter((t) => t.id !== id);
	}
}

export const toastStore = new ToastStore();
