export function formatTime(value: string): string {
	return new Date(value).toLocaleTimeString('it-IT', {
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	});
}

export function formatRtt(ms: number): string {
	if (ms < 0) return '';
	if (ms < 1000) return `${ms}ms`;
	const sec = Math.round(ms / 1000);
	if (sec < 60) return `${sec}s`;
	return `${Math.floor(sec / 60)}m ${sec % 60}s`;
}
