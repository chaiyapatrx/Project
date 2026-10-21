import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';

const DialogContext = createContext(null);

// In-app replacement for window.confirm/window.alert.
// await dialog({ title, message, confirmLabel, cancelLabel, tone }) resolves true or false;
// pass cancelLabel: null for a single OK button.
export function DialogProvider({ children }) {
    const [dialog, setDialog] = useState(null);
    const confirmButton = useRef(null);
    const open = useCallback(options => new Promise(resolve => setDialog({ ...options, resolve })), []);
    const close = result => {
        dialog.resolve(result);
        setDialog(null);
    };

    useEffect(() => {
        if (!dialog) return undefined;
        confirmButton.current?.focus();
        const onKeyDown = event => {
            if (event.key === 'Escape') {
                dialog.resolve(false);
                setDialog(null);
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [dialog]);

    const danger = dialog?.tone === 'danger';
    return (
        <DialogContext.Provider value={open}>
            {children}
            {dialog && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center bg-slate-900/40 backdrop-blur-sm p-6"
                    onMouseDown={event => event.target === event.currentTarget && close(false)}>
                    <div role="alertdialog" aria-modal="true" aria-labelledby="app-dialog-title" aria-describedby="app-dialog-message"
                        className="bg-white rounded-2xl shadow-2xl w-full max-w-sm p-6">
                        <div className="flex items-start gap-3">
                            <span className={`material-symbols-outlined shrink-0 w-10 h-10 rounded-full flex items-center justify-center ${danger ? 'bg-rose-50 text-rose-600' : 'bg-purple-50 text-[#7c3aed]'}`} aria-hidden="true">
                                {danger ? 'warning' : 'info'}
                            </span>
                            <div className="min-w-0">
                                <h2 id="app-dialog-title" className="text-base font-bold text-slate-800">{dialog.title}</h2>
                                <p id="app-dialog-message" className="mt-1 text-sm text-slate-600 whitespace-pre-line">{dialog.message}</p>
                            </div>
                        </div>
                        <div className="mt-6 flex justify-end gap-2">
                            {dialog.cancelLabel !== null && (
                                <button type="button" onClick={() => close(false)} className="px-4 py-2 rounded-xl text-sm font-semibold text-slate-600 hover:bg-slate-100">
                                    {dialog.cancelLabel || 'Cancel'}
                                </button>
                            )}
                            <button type="button" ref={confirmButton} onClick={() => close(true)}
                                className={`px-4 py-2 rounded-xl text-sm font-bold text-white shadow-sm ${danger ? 'bg-rose-600 hover:bg-rose-700' : 'bg-[#7c3aed] hover:bg-[#6d28d9]'}`}>
                                {dialog.confirmLabel || 'OK'}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </DialogContext.Provider>
    );
}

// eslint-disable-next-line react-refresh/only-export-components
export const useDialog = () => useContext(DialogContext);
