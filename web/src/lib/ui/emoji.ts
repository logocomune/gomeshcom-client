export function insertAtSelection(
	message: string,
	selectionStart: number,
	selectionEnd: number,
	value: string
): { message: string; cursor: number } {
	const start = Math.max(0, Math.min(selectionStart, message.length));
	const end = Math.max(start, Math.min(selectionEnd, message.length));
	const nextMessage = `${message.slice(0, start)}${value}${message.slice(end)}`;

	return { message: nextMessage, cursor: start + value.length };
}

export function runeCount(value: string): number {
	return Array.from(value).length;
}

export function limitRunes(value: string, maximum: number): string {
	return Array.from(value).slice(0, maximum).join('');
}
