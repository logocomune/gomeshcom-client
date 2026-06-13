const DM_TOAST_DURATION_MS = 30_000;

type DmToast = {
	id: number;
	from: string;
};

class ToastStore {
	toasts = $state<DmToast[]>([]);
	private nextId = 0;

	addDm(from: string) {
		const id = this.nextId++;
		this.toasts = [...this.toasts, { id, from }];
		setTimeout(() => this.dismiss(id), DM_TOAST_DURATION_MS);
	}

	dismiss(id: number) {
		this.toasts = this.toasts.filter((t) => t.id !== id);
	}
}

export const toastStore = new ToastStore();
