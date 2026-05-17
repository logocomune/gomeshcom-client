export function watchDesktop(onChange: (isDesktop: boolean) => void): () => void {
	if (typeof matchMedia === 'undefined') {
		onChange(true);
		return () => {};
	}
	const mq = matchMedia('(min-width: 768px)');
	const handler = () => onChange(mq.matches);
	handler();
	mq.addEventListener('change', handler);
	return () => mq.removeEventListener('change', handler);
}
