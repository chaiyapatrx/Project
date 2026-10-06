import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { useMonitorWebSocket } from '../useMonitorWebSocket';

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

function StaffSessionControl() {
    const { user } = useAuth();
    const [bookings, setBookings] = useState([]);
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [toast, setToast] = useState(null);
    const [alert, setAlert] = useState(null);

    const fetchBookings = useCallback(async () => {
        try {
            const response = await apiFetch("/admin/bookings", { user });
            if (response.ok) {
                const data = await response.json();
                const activeBookings = data.filter(b => b.status === "active");
                setBookings(activeBookings);
            }
        } catch (error) {
            console.error("Error fetching bookings:", error);
        } finally {
            setLoading(false);
        }
    }, [user]);

    useMonitorWebSocket({
        user,
        onStatusChanged: () => {
            fetchBookings();
        }
    });

    useEffect(() => {
        if (user) {
            fetchBookings();
            const interval = setInterval(fetchBookings, 10000);
            return () => clearInterval(interval);
        }
    }, [user, fetchBookings]);

    const calculateRemainingTime = (endTimeString) => {
        if (!endTimeString) return "Unknown";
        // Handle timezone issues manually if needed, or assume UTC/local matching
        const end = new Date(endTimeString);
        if (!end.getTime()) return "Invalid Date";

        const now = new Date();
        const diffMs = end - now;

        if (diffMs <= 0) return "Expired";

        const diffMins = Math.floor(diffMs / 60000);
        const hours = Math.floor(diffMins / 60);
        const mins = diffMins % 60;

        if (hours > 0) return `${hours}h ${mins}m remaining`;
        return `${mins} mins remaining`;
    };

    const getStatusType = (endTimeString) => {
        if (!endTimeString) return 'Unknown';
        const end = new Date(endTimeString);
        const now = new Date();
        const diffMins = Math.floor((end - now) / 60000);

        if (diffMins <= 0) return 'Expired';
        if (diffMins <= 5) return 'Expire Soon';
        if (diffMins <= 15) return 'Warning';
        return 'Active';
    };

    const handleExtend = async (bookingId, minutes) => {
        try {
            const response = await apiFetch(`/bookings/${bookingId}/extend`, {
                method: "POST",
                user,
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ add_minutes: minutes })
            });

            if (response.ok) {
                setToast({ type: 'success', message: `Added +${minutes} minutes to session #${bookingId}` });
                fetchBookings();
            } else {
                const err = await response.json();
                setToast({ type: 'error', message: err.error || "Failed to extend session" });
            }
        } catch {
            setToast({ type: 'error', message: "Network error extending session" });
        }
    };

    const handleTerminate = async (bookingId) => {
        setAlert({
            title: "Terminate Session?",
            message: "Are you sure you want to forcibly terminate this user's active session?",
            type: "danger",
            onConfirm: async () => {
                setAlert(null);
                try {
                    const response = await apiFetch(`/bookings/${bookingId}`, {
                        method: "DELETE",
                        user
                    });

                    if (response.ok) {
                        setToast({ type: 'success', message: `Session #${bookingId} terminated successfully.` });
                        fetchBookings();
                    } else {
                        setToast({ type: 'error', message: "Failed to terminate session." });
                    }
                } catch (error) {
                    console.error("Terminate session error:", error);
                    setToast({ type: 'error', message: "Network error terminating session." });
                }
            }
        });
    };

    const [broadcastMsg, setBroadcastMsg] = useState('');

    const handleBroadcast = async () => {
        if (!broadcastMsg.trim()) return;
        try {
            const response = await apiFetch(`/api/admin/computers/broadcast`, {
                method: "POST",
                user,
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ command: "NOTIFICATION", data: { message: broadcastMsg } })
            });

            if (response.ok) {
                const data = await response.json();
                const deliveredCount = Number(data.delivered_count) || 0;
                if (deliveredCount > 0) {
                    setToast({ type: 'success', message: `Broadcast message sent to ${deliveredCount} online screens.` });
                    setBroadcastMsg('');
                } else {
                    setToast({ type: 'error', message: "No online screens received the broadcast message." });
                }
            } else {
                const err = await response.json().catch(() => ({}));
                setToast({ type: 'error', message: err.error || err.detail || "Failed to broadcast message." });
            }
        } catch {
            setToast({ type: 'error', message: "Network error sending broadcast." });
        }
    };

    const handleGlobalAction = async (action) => {
        setAlert({
            title: `Confirm ${action} ALL?`,
            message: `Are you sure you want to ${action} all stations? This is an emergency action.`,
            type: "danger",
            onConfirm: async () => {
                setAlert(null);
                try {
                    const response = await apiFetch(`/api/admin/computers/broadcast`, {
                        method: "POST",
                        user,
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ command: action.toUpperCase() })
                    });

                    if (response.ok) {
                        const data = await response.json();
                        const deliveredCount = Number(data.delivered_count) || 0;
                        const endedCount = Number(data.sessions_ended_count) || 0;
                        if (action.toLowerCase() === 'logout') await fetchBookings();
                        setToast(deliveredCount > 0 || endedCount > 0
                            ? { type: 'success', message: action.toLowerCase() === 'logout' ? `Ended ${endedCount} station sessions; ${deliveredCount} agents received the lock.` : `Global ${action} command sent to ${deliveredCount} machines.` }
                            : { type: 'error', message: `No online machines received the ${action} command.` });
                    } else {
                        const err = await response.json().catch(() => ({}));
                        setToast({ type: 'error', message: err.error || err.detail || `Failed to execute global ${action}.` });
                    }
                } catch {
                    setToast({ type: 'error', message: "Network error sending global command." });
                }
            }
        });
    };

    // Filter by search query (username/student ID or station name)
    const filteredBookings = bookings.filter(b => {
        if (!searchQuery) return true;
        const search = searchQuery.toLowerCase();
        const username = b.user?.username || b.user_username || b.user_name || `user${b.user_id}`;
        const station = b.computer?.name || b.computer_name || `com-${b.computer_id}`;
        const usernameMatch = username.toLowerCase().includes(search);
        const stationMatch = station.toLowerCase().includes(search);
        return usernameMatch || stationMatch;
    });

    return (
        <div className="p-8 h-full overflow-y-auto custom-scrollbar relative">
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

            <h2 className="text-xl font-bold text-slate-800 mb-6">Session Control</h2>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                {/* Active Sessions List */}
                <div className="glass-card rounded-2xl p-6 shadow-sm">
                    <div className="flex items-center justify-between mb-6">
                        <h3 className="font-bold text-slate-700">Active Bookings</h3>
                        <div className="flex gap-2">
                            <input
                                type="text"
                                placeholder="Search by User/Station..."
                                value={searchQuery}
                                onChange={e => setSearchQuery(e.target.value)}
                                className="bg-slate-50 border border-slate-200 rounded-lg px-3 py-1.5 text-xs outline-none focus:ring-2 focus:ring-purple-200 w-48"
                            />
                        </div>
                    </div>

                    <div className="space-y-3 max-h-[60vh] overflow-y-auto pr-2 custom-scrollbar">
                        {loading ? (
                            <div className="flex justify-center p-8"><div className="w-6 h-6 rounded-full border-2 border-slate-200 border-t-purple-500 animate-spin"></div></div>
                        ) : filteredBookings.length === 0 ? (
                            <div className="text-center p-8 text-slate-500 text-sm">No active bookings found.</div>
                        ) : (
                            filteredBookings.map((session, i) => {
                                const status = getStatusType(session.end_time);
                                const remaining = calculateRemainingTime(session.end_time);
                                const username = session.user?.username || session.user_username || session.user_name || `User #${session.user_id}`;
                                const stationName = session.computer?.name || session.computer_name || `COM-${session.computer_id}`;
                                const isExpired = status === 'Expired';
                                
                                return (
                                    <div key={session.id || i} className={`flex items-center justify-between p-3 rounded-xl border ${isExpired ? 'border-red-200 bg-red-50/50' : 'border-slate-100 hover:bg-slate-50'} transition-colors`}>
                                        <div className="flex items-center gap-3">
                                            <div className="w-10 h-10 rounded-full bg-slate-100 flex items-center justify-center text-slate-500 font-bold text-xs">
                                                <span className="material-symbols-outlined text-xl">person</span>
                                            </div>
                                            <div>
                                                <p className="text-sm font-bold text-slate-800">{username}</p>
                                                <p className="text-xs text-slate-500">Station: {stationName}</p>
                                            </div>
                                        </div>
                                        <div className="text-right">
                                            <p className={`text-xs font-bold ${isExpired || status === 'Expire Soon' ? 'text-red-500' :
                                                    status === 'Warning' ? 'text-amber-500' : 'text-emerald-500'
                                                }`}>{remaining}</p>
                                            <div className="flex justify-end items-center gap-2 mt-1.5">
                                                <button
                                                    className="text-[10px] font-bold text-purple-600 hover:text-purple-800 hover:bg-purple-50 px-1.5 py-0.5 rounded border border-purple-200 transition-colors"
                                                    onClick={() => handleExtend(session.id, 30)}
                                                    title="Add 30 minutes"
                                                >
                                                    +30m
                                                </button>
                                                <button
                                                    className="text-[10px] font-bold text-purple-600 hover:text-purple-800 hover:bg-purple-50 px-1.5 py-0.5 rounded border border-purple-200 transition-colors"
                                                    onClick={() => handleExtend(session.id, 60)}
                                                    title="Add 60 minutes"
                                                >
                                                    +60m
                                                </button>
                                                <button className="text-[10px] font-bold text-red-500 hover:text-red-700 hover:underline ml-1" onClick={() => handleTerminate(session.id)}>Terminate</button>
                                            </div>
                                        </div>
                                    </div>
                                );
                            })
                        )}
                    </div>
                </div>

                {/* Quick Actions */}
                <div className="space-y-6">
                    <div className="glass-card rounded-2xl p-6 shadow-sm border-l-4 border-l-red-500">
                        <h3 className="font-bold text-slate-800 mb-2 flex items-center gap-2">
                            <span className="material-symbols-outlined text-red-500">warning</span>
                            Emergency Controls
                        </h3>
                        <p className="text-xs text-slate-500 mb-6">Use these actions only in case of emergency or system maintenance.</p>

                        <div className="grid grid-cols-1 gap-4">
                            <button onClick={() => handleGlobalAction('Lock')} className="p-4 rounded-xl bg-red-50 text-red-600 border border-red-100 font-bold text-sm hover:bg-red-100 transition-colors flex flex-col items-center gap-2">
                                <span className="material-symbols-outlined text-2xl">lock</span>
                                Lock All Stations
                            </button>
                        </div>
                    </div>

                    <div className="glass-card rounded-2xl p-6 shadow-sm">
                        <h3 className="font-bold text-slate-800 mb-4">Message Broadcast</h3>
                        <textarea
                            value={broadcastMsg}
                            onChange={(e) => setBroadcastMsg(e.target.value)}
                            className="w-full h-32 bg-slate-50 border border-slate-200 rounded-xl p-4 text-sm outline-none focus:ring-2 focus:ring-purple-200 resize-none mb-4"
                            placeholder="Type a message to send to all active screens..."
                        ></textarea>
                        <button
                            onClick={handleBroadcast}
                            className="w-full py-3 bg-slate-800 text-white rounded-xl text-sm font-bold hover:bg-slate-900 transition-all flex items-center justify-center gap-2"
                        >
                            <span className="material-symbols-outlined text-lg">send</span>
                            Broadcast Message
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default StaffSessionControl;
