import { useEffect, useId, useRef, useState } from 'react';
import { endReason } from './format';

// Styled replacement for <select>: button + listbox with keyboard support.
export function Select({ value, onChange, options, label, className = '' }) {
    const [open, setOpen] = useState(false);
    const [active, setActive] = useState(-1);
    const root = useRef(null);
    const id = useId();
    const selectedIndex = options.findIndex(option => String(option.value) === String(value));
    const selected = options[selectedIndex];

    useEffect(() => {
        if (!open) return undefined;
        const closeOutside = event => { if (!root.current?.contains(event.target)) setOpen(false); };
        document.addEventListener('mousedown', closeOutside);
        return () => document.removeEventListener('mousedown', closeOutside);
    }, [open]);

    const show = () => { setActive(Math.max(0, selectedIndex)); setOpen(true); };
    const choose = option => { onChange(option.value); setOpen(false); };
    const onKeyDown = event => {
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            if (!open) return show();
            const step = event.key === 'ArrowDown' ? 1 : -1;
            setActive(index => (index + step + options.length) % options.length);
        } else if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            if (open && active >= 0) choose(options[active]); else show();
        } else if (event.key === 'Escape' || event.key === 'Tab') {
            setOpen(false);
        }
    };

    return (
        <div ref={root} className={`relative ${className}`}>
            <button type="button" aria-haspopup="listbox" aria-expanded={open} aria-label={label} aria-controls={id}
                aria-activedescendant={open && active >= 0 ? `${id}-${active}` : undefined}
                onClick={() => (open ? setOpen(false) : show())} onKeyDown={onKeyDown}
                className={`w-full flex items-center justify-between gap-2 bg-white border rounded-xl px-3 py-2 text-sm text-slate-700 transition focus:outline-none focus:ring-2 focus:ring-purple-300 ${open ? 'border-purple-400' : 'border-slate-200 hover:border-purple-300'}`}>
                <span className="truncate">{selected ? selected.label : 'Select...'}</span>
                <span className={`material-symbols-outlined text-lg text-slate-400 transition-transform ${open ? 'rotate-180' : ''}`} aria-hidden="true">expand_more</span>
            </button>
            {open && (
                <ul id={id} role="listbox" aria-label={label}
                    className="absolute z-50 mt-1.5 w-full min-w-max bg-white border border-slate-100 rounded-xl shadow-xl shadow-slate-200/70 py-1 max-h-64 overflow-auto">
                    {options.map((option, index) => {
                        const isSelected = index === selectedIndex;
                        return (
                            <li key={option.value} id={`${id}-${index}`} role="option" aria-selected={isSelected}
                                onMouseDown={event => event.preventDefault()} onClick={() => choose(option)} onMouseEnter={() => setActive(index)}
                                className={`flex items-center justify-between gap-4 px-3 py-2 text-sm cursor-pointer ${index === active ? 'bg-purple-50' : ''} ${isSelected ? 'text-[#7c3aed] font-semibold' : 'text-slate-700'}`}>
                                {option.label}
                                {isSelected && <span className="material-symbols-outlined text-base" aria-hidden="true">check</span>}
                            </li>
                        );
                    })}
                </ul>
            )}
        </div>
    );
}

// Styled file picker that shows the chosen file name.
export function FileInput({ accept, file, onChange, label = 'Choose file', required = false }) {
    return (
        <label className="flex items-center gap-3 border border-dashed border-slate-300 bg-white rounded-xl px-3 py-2 cursor-pointer transition hover:border-purple-400 hover:bg-purple-50/40 focus-within:ring-2 focus-within:ring-purple-300">
            <span className="material-symbols-outlined text-[#7c3aed]" aria-hidden="true">upload_file</span>
            <span className={`text-sm truncate ${file ? 'text-slate-800 font-medium' : 'text-slate-500'}`}>{file ? file.name : label}</span>
            <input type="file" accept={accept} required={required} onChange={event => onChange(event.target.files[0] || null)} className="sr-only" />
        </label>
    );
}

const TONES = {
    amber: 'bg-amber-50 text-amber-700 border-amber-200',
    emerald: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    rose: 'bg-rose-50 text-rose-700 border-rose-200',
    violet: 'bg-violet-50 text-violet-700 border-violet-200',
    blue: 'bg-blue-50 text-blue-700 border-blue-200',
    slate: 'bg-slate-100 text-slate-600 border-slate-200',
};

// Readable badge for why a session ended.
export function EndReasonBadge({ reason }) {
    const { label, tone } = endReason(reason);
    return (
        <span className={`inline-flex items-center whitespace-nowrap px-2.5 py-0.5 rounded-full border text-[11px] font-semibold ${TONES[tone]}`}>
            {label}
        </span>
    );
}
