// Up to two leading characters for an avatar. Thai vowels and tone marks stay
// attached to their consonant instead of being split off.
export function initials(name, fallback = '?') {
    const words = String(name || '').trim().split(/\s+/).filter(Boolean);
    if (!words.length) return fallback;
    const first = word => {
        const segments = typeof Intl.Segmenter === 'function'
            ? [...new Intl.Segmenter('th', { granularity: 'grapheme' }).segment(word)].map(s => s.segment)
            : [...word];
        return segments[0] || '';
    };
    const letters = words.length > 1 ? first(words[0]) + first(words[1]) : first(words[0]);
    return letters.toUpperCase();
}

// Why a station session ended, as stored in usage_logs.termination_reason.
export const END_REASONS = {
    timeout: { label: 'Time ended', tone: 'emerald' },
    normal: { label: 'Completed', tone: 'emerald' },
    completed: { label: 'Completed', tone: 'emerald' },
    cancelled: { label: 'Cancelled', tone: 'rose' },
    forced_by_staff: { label: 'Ended by staff', tone: 'violet' },
    force_logout: { label: 'Ended by staff', tone: 'violet' },
    system_shutdown: { label: 'Shut down', tone: 'slate' },
    active: { label: 'In progress', tone: 'blue' },
};

export function endReason(reason) {
    return END_REASONS[reason] || { label: reason ? reason.replace(/_/g, ' ') : 'Ended', tone: 'slate' };
}
