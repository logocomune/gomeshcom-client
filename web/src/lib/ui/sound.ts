let ctx: AudioContext | null = null;

function getAudioContext(): AudioContext | null {
	if (typeof window === 'undefined') return null;
	if (!ctx) ctx = new AudioContext();
	return ctx;
}

function playTone(
	frequency: number,
	startTime: number,
	duration: number,
	gain: number,
	ac: AudioContext
) {
	const osc = ac.createOscillator();
	const env = ac.createGain();

	osc.type = 'sine';
	osc.frequency.setValueAtTime(frequency, startTime);

	env.gain.setValueAtTime(0, startTime);
	env.gain.linearRampToValueAtTime(gain, startTime + 0.01);
	env.gain.exponentialRampToValueAtTime(0.001, startTime + duration);

	osc.connect(env);
	env.connect(ac.destination);

	osc.start(startTime);
	osc.stop(startTime + duration);
}

// Two-note ascending ping — distinct from generic packet sounds
export function playDmAlert(): void {
	const ac = getAudioContext();
	if (!ac) return;

	// Resume context if suspended (browser autoplay policy)
	const resume = ac.state === 'suspended' ? ac.resume() : Promise.resolve();
	void resume.then(() => {
		const now = ac.currentTime;
		playTone(880, now, 0.18, 0.25, ac);
		playTone(1320, now + 0.12, 0.22, 0.2, ac);
	});
}
