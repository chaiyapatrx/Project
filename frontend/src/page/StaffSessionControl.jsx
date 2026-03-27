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

function StaffSessionControl() {
    const { user } = useAuth();
    const [bookings, setBookings] = useState([]);
    const [loading, setLoading] = useState(true);
    const [searchQuery, setSearchQuery] = useState('');
    const [toast, setToast] = useState(null);
    const [alert, setAlert] = useState(null);

    const fetchBookings = async () => {
        try {
            const baseUrl = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
                ? 'http://localhost:8000'
                : `http://${window.location.hostname}:8000`;

            const response = await fetch(`${baseUrl}/admin/bookings`, {
                headers: { "Authorization": `Bearer ${user.token}` }
            });
            if (response.ok) {
                const data = await response.json();
                // Filter only confirmed or active bookings
                const activeBookings = data.filter(b => b.status === "confirmed" || b.status === "active");
                setBookings(activeBookings);
            }
        } catch (error) {
            console.error("Error fetching bookings:", error);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (user) {
            fetchBookings();
            const interval = setInterval(fetchBookings, 10000);
            return () => clearInterval(interval);
        }
    }, [user]);

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

    const handleTerminate = async (bookingId) => {
        setAlert({
            title: "Terminate Session?",
            message: "Are you sure you want to forcibly terminate this user's active session?",
            type: "danger",
            onConfirm: async () => {
                setAlert(null);
                setToast({ type: 'success', message: `Terminate functionality executed for booking ID: ${bookingId}` });
            }
        });
    };
    
    const handleGlobalAction = async (action) => {
        setAlert({
            title: `Confirm ${action} ALL?`,
            message: `Are you sure you want to ${action} all stations? This is an emergency action.`,
            type: "danger",
            onConfirm: async () => {
                setAlert(null);
                setToast({ type: 'success', message: `Global ${action} command sent.` });
            }
        });
    };

    // Filter by search query (username/student ID or station name)
    const filteredBookings = bookings.filter(b => {
        if (!searchQuery) return true;
        const search = searchQuery.toLowerCase();
        const usernameMatch = b.user?.username?.toLowerCase().includes(search);
        const stationMatch = b.computer?.name?.toLowerCase().includes(search);
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
                                const username = session.user?.username || `User ${session.user_id}`;
                                const stationName = session.computer?.name || `Station ${session.computer_id}`;
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
                                            <div className="flex justify-end gap-1 mt-1">
                                                <button className="text-[10px] font-bold text-red-500 hover:text-red-700 hover:underline" onClick={() => handleTerminate(session.id)}>Terminate</button>
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

                        <div className="grid grid-cols-2 gap-4">
                            <button onClick={() => handleGlobalAction('Lock')} className="p-4 rounded-xl bg-red-50 text-red-600 border border-red-100 font-bold text-sm hover:bg-red-100 transition-colors flex flex-col items-center gap-2">
                                <span className="material-symbols-outlined text-2xl">lock</span>
                                Lock All Stations
                            </button>
                            <button onClick={() => handleGlobalAction('Shutdown')} className="p-4 rounded-xl bg-amber-50 text-amber-600 border border-amber-100 font-bold text-sm hover:bg-amber-100 transition-colors flex flex-col items-center gap-2">
                                <span className="material-symbols-outlined text-2xl">power_settings_new</span>
                                Shutdown Lab
                            </button>
                        </div>
                    </div>

                    <div className="glass-card rounded-2xl p-6 shadow-sm">
                        <h3 className="font-bold text-slate-800 mb-4">Message Broadcast</h3>
                        <textarea
                            className="w-full h-32 bg-slate-50 border border-slate-200 rounded-xl p-4 text-sm outline-none focus:ring-2 focus:ring-purple-200 resize-none mb-4"
                            placeholder="Type a message to send to all active screens..."
                        ></textarea>
                        <button className="w-full py-3 bg-slate-800 text-white rounded-xl text-sm font-bold hover:bg-slate-900 transition-all flex items-center justify-center gap-2">
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
