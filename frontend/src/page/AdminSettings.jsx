import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

const Toast = ({ message, type, onClose }) => {
    useEffect(() => { const timer = setTimeout(onClose, 3000); return () => clearTimeout(timer); }, [onClose]);
    const bgClass = type === 'success' ? 'bg-emerald-50 border-emerald-200 text-emerald-800' : 'bg-rose-50 border-rose-200 text-rose-800';
    const icon = type === 'success' ? 'check_circle' : 'error';
    return (
        <div className={`fixed bottom-6 right-6 z-[200] flex items-center gap-3 px-4 py-3 rounded-xl border shadow-lg ${bgClass}`} style={{ animation: 'slideIn 0.3s ease-out forwards' }}>
            <span className="material-symbols-outlined">{icon}</span><span className="text-sm font-bold">{message}</span>
        </div>
    );
};

function AdminSettings() {
    const { user } = useAuth();
    const [isLoading, setIsLoading] = useState(false);
    const [toast, setToast] = useState(null);
    const [settings, setSettings] = useState({
        sessionDuration: 120,
        maintenanceMode: false
    });

    useEffect(() => {
        const fetchSettings = async () => {
            if (!user) return;
            try {
                const response = await apiFetch("/api/admin/settings");
                if (response.ok) {
                    const data = await response.json();
                    setSettings({
                        sessionDuration: parseInt(data.session_duration) || 120,
                        maintenanceMode: data.maintenance_mode === "true"
                    });
                }
            } catch (err) {
                console.error("Failed to fetch settings:", err);
            }
        };
        fetchSettings();
    }, [user]);

    const handleSave = async () => {
        setIsLoading(true);
        try {
            const payload = {
                session_duration: String(settings.sessionDuration),
                maintenance_mode: String(settings.maintenanceMode)
            };

            const response = await apiFetch("/api/admin/settings", {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify(payload)
            });

            if (response.ok) {
                setToast({ type: 'success', message: "Settings saved to MySQL database successfully!" });
            } else {
                setToast({ type: 'error', message: "Failed to update settings" });
            }
        } catch {
            setToast({ type: 'error', message: "Network error saving settings" });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="h-full overflow-hidden flex flex-col relative">
            <style>{`
                @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
            `}</style>
            {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}

            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">System Settings</h1>
                <button
                    onClick={handleSave}
                    disabled={isLoading}
                    className="bg-[#7c3aed] hover:bg-[#6d28d9] disabled:opacity-50 text-white px-4 py-2 rounded-lg text-sm font-bold flex items-center gap-2 transition-all shadow-md shadow-purple-200"
                >
                    {isLoading ? (
                        <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                    ) : (
                        <span className="material-symbols-outlined text-lg">save</span>
                    )}
                    <span>{isLoading ? 'Saving...' : 'Save Changes'}</span>
                </button>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8">
                <div className="max-w-4xl mx-auto space-y-8">

                    {/* Booking Rules */}
                    <div className="glass-card p-6 rounded-2xl shadow-sm">
                        <div className="flex items-center gap-3 mb-6 pb-4 border-b border-slate-100">
                            <div className="w-10 h-10 rounded-xl bg-purple-50 flex items-center justify-center text-[#7c3aed]">
                                <span className="material-symbols-outlined">tune</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-slate-800">Booking Rules</h3>
                                <p className="text-xs text-slate-500">Set the maximum session duration</p>
                            </div>
                        </div>

                        <div className="space-y-4">
                            <div>
                                <label htmlFor="session-duration" className="block text-xs font-bold text-slate-500 uppercase mb-1">Max Session Duration (Minutes)</label>
                                <div className="flex items-center gap-4">
                                    <input
                                        id="session-duration"
                                        type="range" min="30" max="240" step="30"
                                        value={settings.sessionDuration}
                                        onChange={(e) => setSettings({ ...settings, sessionDuration: e.target.value })}
                                        className="flex-1 accent-[#7c3aed]"
                                    />
                                    <span className="font-bold text-slate-700 bg-slate-100 px-3 py-1 rounded-lg text-sm">{settings.sessionDuration} min</span>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Access Control */}
                    <div className="glass-card p-6 rounded-2xl shadow-sm">
                        <div className="flex items-center gap-3 mb-6 pb-4 border-b border-slate-100">
                            <div className="w-10 h-10 rounded-xl bg-blue-50 flex items-center justify-center text-blue-500">
                                <span className="material-symbols-outlined">security</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-slate-800">Access Control</h3>
                                <p className="text-xs text-slate-500">Manage login and security policies</p>
                            </div>
                        </div>

                        <div className="space-y-4">
                            <div className="flex items-center justify-between p-4 bg-slate-50 rounded-xl border border-slate-100">
                                <div>
                                    <p className="font-bold text-slate-700 text-sm">Maintenance Mode</p>
                                    <p className="text-xs text-slate-500">Disable all stations for booking (Admin only access)</p>
                                </div>
                                <button
                                    type="button"
                                    role="switch"
                                    aria-checked={settings.maintenanceMode}
                                    aria-label="Maintenance mode"
                                    onClick={() => setSettings({ ...settings, maintenanceMode: !settings.maintenanceMode })}
                                    className={`w-12 h-6 border-0 rounded-full relative cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-500 ${settings.maintenanceMode ? 'bg-orange-500' : 'bg-slate-300'}`}
                                >
                                    <div className={`absolute top-1 w-4 h-4 bg-white rounded-full shadow-sm transition-all ${settings.maintenanceMode ? 'right-1' : 'left-1'}`}></div>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default AdminSettings;
