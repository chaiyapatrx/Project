import React, { useState, useEffect } from 'react';

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
    const [isLoading, setIsLoading] = useState(false);
    const [toast, setToast] = useState(null);
    const [settings, setSettings] = useState({
        systemName: "AdminCore System",
        timezone: "UTC+7 (Bangkok, Hanoi, Jakarta)",
        sessionDuration: 120,
        guestLogin: false,
        maintenanceMode: false
    });

    const handleSave = () => {
        setIsLoading(true);
        // Simulate API Call
        setTimeout(() => {
            setIsLoading(false);
            setToast({ type: 'success', message: "Settings saved successfully!" });
        }, 1000);
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

                    {/* General Settings */}
                    <div className="glass-card p-6 rounded-2xl shadow-sm">
                        <div className="flex items-center gap-3 mb-6 pb-4 border-b border-slate-100">
                            <div className="w-10 h-10 rounded-xl bg-purple-50 flex items-center justify-center text-[#7c3aed]">
                                <span className="material-symbols-outlined">tune</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-slate-800">General Configuration</h3>
                                <p className="text-xs text-slate-500">Basic system parameters and preferences</p>
                            </div>
                        </div>

                        <div className="space-y-4">
                            <div className="grid grid-cols-2 gap-4">
                                <div>
                                    <label className="block text-xs font-bold text-slate-500 uppercase mb-1">System Name</label>
                                    <input
                                        type="text"
                                        value={settings.systemName}
                                        onChange={(e) => setSettings({ ...settings, systemName: e.target.value })}
                                        className="w-full bg-slate-50 border border-slate-200 rounded-lg px-4 py-2 text-sm font-bold text-slate-700 outline-none focus:ring-2 focus:ring-purple-200 transaction-all"
                                    />
                                </div>
                                <div>
                                    <label className="block text-xs font-bold text-slate-500 uppercase mb-1">Timezone</label>
                                    <select
                                        value={settings.timezone}
                                        onChange={(e) => setSettings({ ...settings, timezone: e.target.value })}
                                        className="w-full bg-slate-50 border border-slate-200 rounded-lg px-4 py-2 text-sm font-bold text-slate-700 outline-none focus:ring-2 focus:ring-purple-200 transaction-all"
                                    >
                                        <option>UTC+7 (Bangkok, Hanoi, Jakarta)</option>
                                        <option>UTC+0 (London)</option>
                                        <option>UTC-5 (New York)</option>
                                    </select>
                                </div>
                            </div>

                            <div>
                                <label className="block text-xs font-bold text-slate-500 uppercase mb-1">Max Session Duration (Minutes)</label>
                                <div className="flex items-center gap-4">
                                    <input
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
                                    <p className="font-bold text-slate-700 text-sm">Guest Login Allowed</p>
                                    <p className="text-xs text-slate-500">Allow users to login without university credentials</p>
                                </div>
                                <div
                                    onClick={() => setSettings({ ...settings, guestLogin: !settings.guestLogin })}
                                    className={`w-12 h-6 rounded-full relative cursor-pointer transition-colors ${settings.guestLogin ? 'bg-[#7c3aed]' : 'bg-slate-300'}`}
                                >
                                    <div className={`absolute top-1 w-4 h-4 bg-white rounded-full shadow-sm transition-all ${settings.guestLogin ? 'right-1' : 'left-1'}`}></div>
                                </div>
                            </div>

                            <div className="flex items-center justify-between p-4 bg-slate-50 rounded-xl border border-slate-100">
                                <div>
                                    <p className="font-bold text-slate-700 text-sm">Maintenance Mode</p>
                                    <p className="text-xs text-slate-500">Disable all stations for booking (Admin only access)</p>
                                </div>
                                <div
                                    onClick={() => setSettings({ ...settings, maintenanceMode: !settings.maintenanceMode })}
                                    className={`w-12 h-6 rounded-full relative cursor-pointer transition-colors ${settings.maintenanceMode ? 'bg-orange-500' : 'bg-slate-300'}`}
                                >
                                    <div className={`absolute top-1 w-4 h-4 bg-white rounded-full shadow-sm transition-all ${settings.maintenanceMode ? 'right-1' : 'left-1'}`}></div>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Notifications */}
                    <div className="glass-card p-6 rounded-2xl shadow-sm">
                        <div className="flex items-center gap-3 mb-6 pb-4 border-b border-slate-100">
                            <div className="w-10 h-10 rounded-xl bg-amber-50 flex items-center justify-center text-amber-500">
                                <span className="material-symbols-outlined">notifications</span>
                            </div>
                            <div>
                                <h3 className="font-bold text-slate-800">Notifications</h3>
                                <p className="text-xs text-slate-500">Email and system alerts</p>
                            </div>
                        </div>

                        <div className="space-y-3">
                            {['System Errors & Warnings', 'Daily Usage Reports', 'New User Registration Alert'].map((item, i) => (
                                <label key={i} className="flex items-center gap-3 cursor-pointer">
                                    <input type="checkbox" defaultChecked className="w-4 h-4 accent-[#7c3aed] rounded" />
                                    <span className="text-sm font-medium text-slate-600">{item}</span>
                                </label>
                            ))}
                        </div>
                    </div>

                </div>
            </div>
        </div>
    );
}

export default AdminSettings;
