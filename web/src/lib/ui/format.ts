export function formatDate(value: string): string {
	return new Date(value).toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit' });
}

export function formatTime(value: string): string {
	return new Date(value).toLocaleTimeString('it-IT', {
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	});
}

export function formatChatTimestamp(value: string): string {
	const d = new Date(value);
	const now = new Date();
	const sameDay =
		d.getFullYear() === now.getFullYear() &&
		d.getMonth() === now.getMonth() &&
		d.getDate() === now.getDate();
	const time = d.toLocaleTimeString('it-IT', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
	if (sameDay) return time;
	const date = d.toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit' });
	return `${date} ${time}`;
}

export function formatRtt(ms: number): string {
	if (ms < 0) return '';
	if (ms < 1000) return `${ms}ms`;
	const sec = Math.round(ms / 1000);
	if (sec < 60) return `${sec}s`;
	return `${Math.floor(sec / 60)}m ${sec % 60}s`;
}
