import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';

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

function StaffStationManagement() {
    const { user } = useAuth();
    const [selectedStation, setSelectedStation] = useState(null);
    const [stations, setStations] = useState([]);
    const [loading, setLoading] = useState(true);
    const [toast, setToast] = useState(null);
    const [alert, setAlert] = useState(null);

    const fetchStations = async () => {
        try {
            const baseUrl = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
                ? 'http://localhost:8000'
                : `http://${window.location.hostname}:8000`;

            const response = await fetch(`${baseUrl}/computers`, {
                headers: { "Authorization": `Bearer ${user.token}` }
            });
            if (response.ok) {
                const data = await response.json();
                setStations(data);
            }
        } catch (error) {
            console.error("Error fetching computers:", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (user) {
            fetchStations();
            // Optional: Auto-refresh every 5 seconds
            const interval = setInterval(fetchStations, 5000);
            return () => clearInterval(interval);
        }
    }, [user]);

    const handleRemoteAction = async (id, action, name) => {
        setAlert({
            title: `Confirm ${action}?`,
            message: `Are you sure you want to ${action} station ${name}?`,
            type: 'warning',
            onConfirm: async () => {
                setAlert(null);
                try {
                    const baseUrl = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
                        ? 'http://localhost:8000'
                        : `http://${window.location.hostname}:8000`;

                    const response = await fetch(`${baseUrl}/api/admin/computers/${id}/command`, {
                        method: "POST",
                        headers: { "Content-Type": "application/json", "Authorization": `Bearer ${user.token}` },
                        body: JSON.stringify({ command: action.toLowerCase() })
                    });

                    if (response.ok) {
                        setToast({ type: 'success', message: `Command ${action} sent successfully.` });
                    } else {
                        const err = await response.json();
                        setToast({ type: 'error', message: `Failed: ${err.detail || 'Unknown error'}` });
                    }
                } catch (error) {
                    console.error("Error sending command:", error);
                    setToast({ type: 'error', message: "Network error sending command" });
                }
            }
        });
    };

    return (
        <div className="p-8 space-y-8 h-full overflow-y-auto custom-scrollbar relative">
            <style>{`
                @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
                @keyframes zoomIn { from { transform: scale(0.95); opacity: 0; } to { transform: scale(1); opacity: 1; } }
            `}</style>
            
            {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
            
            {/* Custom Alert Modal */}
            {alert && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm" style={{ animation: 'zoomIn 0.2s ease-out forwards' }}>
                    <div className="bg-white rounded-2xl p-6 w-full max-w-sm shadow-2xl border ring-4 ring-red-50 text-center">
                        <div className={`w-14 h-14 rounded-full flex items-center justify-center mx-auto mb-4 ${alert.type === 'danger' ? 'bg-red-100 text-red-500' : 'bg-orange-100 text-orange-500'}`}>
                            <span className="material-symbols-outlined text-3xl">warning</span>
                        </div>
                        <h3 className="text-xl font-bold text-slate-800 mb-2">{alert.title}</h3>
                        <p className="text-slate-500 text-sm mb-6">{alert.message}</p>
                        <div className="grid grid-cols-2 gap-3">
                            <button onClick={() => setAlert(null)} className="py-2.5 rounded-xl font-bold text-slate-500 hover:bg-slate-100">Cancel</button>
                            <button onClick={alert.onConfirm} className={`py-2.5 rounded-xl font-bold text-white shadow-lg ${alert.type === 'danger' ? 'bg-red-500 hover:bg-red-600' : 'bg-orange-500 hover:bg-orange-600'}`}>Confirm</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Computer Stations Grid Area */}
            <div className="glass-card rounded-2xl p-6 shadow-sm text-left">
                <div className="flex items-center justify-between mb-8">
                    <div>
                        <h4 className="font-bold text-slate-800">รายการเครื่องคอมพิวเตอร์ (Computer Stations)</h4>
                        <p className="text-xs text-slate-500">Select a station to manage session control</p>
                    </div>
                </div>

                {/* Grid Loop */}
                {loading ? (
                    <div className="flex justify-center p-12"><div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-purple-500 animate-spin"></div></div>
                ) : (
                    <div className="grid grid-cols-2 lg:grid-cols-4 xl:grid-cols-6 gap-4">
                        {stations.map((comp) => {
                            let displayStatus = 'offline';
                            let statusColor = 'bg-slate-400';
                            let iconColor = 'text-slate-400 bg-slate-100';
                            let cardStyle = 'grayscale opacity-75';

                            if (comp.is_online) {
                                cardStyle = ''; // Active
                                if (comp.status === 'in-use' || comp.status === 'occupied') {
                                    displayStatus = 'in-use';
                                    statusColor = 'bg-blue-500';
                                    iconColor = 'text-blue-600 bg-blue-100';
                                } else if (comp.status === 'maintenance') {
                                    displayStatus = 'maintenance';
                                    statusColor = 'bg-orange-500';
                                    iconColor = 'text-orange-500 bg-orange-100';
                                } else {
                                    displayStatus = 'available';
                                    statusColor = 'bg-emerald-500';
                                    iconColor = 'text-emerald-500 bg-emerald-100';
                                }
                            }

                            return (
                                <div
                                    key={comp.id}
                                    onClick={() => setSelectedStation(comp)}
                                    className={`group relative bg-white border border-slate-200 rounded-xl p-4 flex flex-col items-center gap-3 transition-all hover:shadow-md cursor-pointer hover:-translate-y-1 ${cardStyle} ${displayStatus === 'in-use' ? 'ring-1 ring-blue-100 bg-blue-50/10' : ''}`}
                                >
                                    <div className="absolute top-2 right-2">
                                        <div className={`w-2 h-2 rounded-full ${statusColor}`}></div>
                                    </div>

                                    <div className={`w-12 h-12 rounded-xl flex items-center justify-center ${iconColor}`}>
                                        <span className="material-symbols-outlined text-2xl">desktop_windows</span>
                                    </div>

                                    <div className="text-center">
                                        <p className="font-bold text-slate-700 text-sm whitespace-nowrap overflow-hidden text-ellipsis w-20" title={comp.name}>{comp.name}</p>
                                        <p className="text-[10px] uppercase font-bold text-slate-400 mt-0.5">{displayStatus}</p>
                                        {comp.hwid && <p className="text-[8px] text-slate-300 font-mono mt-1 truncate w-16">{comp.hwid.substring(0, 6)}...</p>}
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                )}
            </div>

            {/* ================= MODAL Overlay ================= */}
            {selectedStation && (
                <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-sm z-50 flex items-center justify-center transition-opacity duration-300">
                    {/* Modal Card */}
                    <div className="bg-white rounded-3xl p-8 w-full max-w-sm shadow-2xl animate-zoom-in text-left relative">

                        <div className="flex items-center justify-between mb-8">
                            <div className="flex items-center gap-4">
                                <div className="w-12 h-12 bg-slate-50 rounded-2xl flex items-center justify-center text-slate-600 border border-slate-100">
                                    <span className="material-symbols-outlined text-2xl">desktop_windows</span>
                                </div>
                                <div>
                                    <h3 className="font-black text-slate-800 text-lg tracking-tight overflow-hidden text-ellipsis whitespace-nowrap w-40" title={selectedStation.name}>{selectedStation.name}</h3>
                                    <p className="text-xs text-slate-500 mt-0.5">
                                        {selectedStation.is_online 
                                          ? (selectedStation.status === 'in-use' || selectedStation.status === 'occupied' ? 'Machine is in use' : 'Machine is idle')
                                          : 'Offline'}
                                    </p>
                                </div>
                            </div>
                            <button
                                onClick={() => setSelectedStation(null)}
                                className="text-slate-400 hover:text-slate-600 transition-colors bg-slate-50 p-2 rounded-full hover:bg-slate-100"
                            >
                                <span className="material-symbols-outlined text-xl">close</span>
                            </button>
                        </div>

                        {selectedStation.is_online ? (
                            <div className="space-y-4">
                                <button
                                    onClick={() => {
                                        handleRemoteAction(selectedStation.id, 'Logout', selectedStation.name);
                                        setSelectedStation(null);
                                    }}
                                    className="w-full bg-amber-500 hover:bg-amber-600 text-white font-bold py-4 rounded-2xl text-sm flex items-center justify-center gap-2 transition-all shadow-lg shadow-amber-200"
                                >
                                    <span className="material-symbols-outlined">logout</span> Force Logout (ตัดการเชื่อมต่อ)
                                </button>
                                <button
                                    onClick={() => {
                                        handleRemoteAction(selectedStation.id, 'Restart', selectedStation.name);
                                        setSelectedStation(null);
                                    }}
                                    className="w-full bg-slate-100 hover:bg-slate-200 text-slate-700 font-bold py-4 rounded-2xl text-sm flex items-center justify-center gap-2 transition-all"
                                >
                                    <span className="material-symbols-outlined">restart_alt</span> Reset Machine (เริ่มใหม่)
                                </button>
                            </div>
                        ) : (
                            <div className="p-4 bg-slate-50 rounded-2xl text-center text-slate-500 font-medium text-sm border border-slate-100">
                                This machine is currently offline. Remote actions are disabled.
                            </div>
                        )}

                        <p className="text-[10px] text-slate-400 text-center mt-6 font-bold uppercase tracking-widest">
                            Innovation Hub Management System
                        </p>
                    </div>
                </div>
            )}
        </div>
    );
}

export default StaffStationManagement;

