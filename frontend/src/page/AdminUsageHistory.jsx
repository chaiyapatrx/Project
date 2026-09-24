import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { toCSV } from '../csv';

function AdminUsageHistory() {
    const { user } = useAuth();
    const [bookings, setBookings] = useState([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchBookings = async () => {
            try {
                const response = await apiFetch("/admin/bookings");
                if (response.ok) {
                    const data = await response.json();
                    setBookings(data);
                }
            } catch (error) {
                console.error("Error fetching bookings:", error);
            } finally {
                setLoading(false);
            }
        };

        if (user) {
            fetchBookings();
        }
    }, [user]);

    const exportCSV = () => {
        if (!bookings || bookings.length === 0) return;
        const rows = bookings.map(b => [b.id, b.user_name || b.user_id, b.computer_name || b.computer_id, b.start_time, b.end_time || '-', b.status]);
        const blob = new Blob([toCSV(["Session ID", "User", "Computer", "Start Time", "End Time", "Status"], rows)], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.setAttribute("href", url);
        link.setAttribute("download", `usage_history_${new Date().toISOString().slice(0,10)}.csv`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    const formatDate = (dateString) => {
        return new Date(dateString).toLocaleString('en-US', {
            dateStyle: 'medium',
            timeStyle: 'short'
        });
    };

    return (
        <div className="h-full overflow-hidden flex flex-col">
            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">Usage History</h1>
                <div className="flex gap-2">
                    <button
                        onClick={exportCSV}
                        className="bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 px-4 py-2 rounded-lg text-sm font-bold flex items-center gap-2 transition-all"
                    >
                        <span className="material-symbols-outlined text-lg">download</span>
                        <span>Export CSV</span>
                    </button>
                </div>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8">
                <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                    {loading ? (
                        <div className="flex justify-center p-12"><div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-purple-500 animate-spin"></div></div>
                    ) : (
                        <table className="w-full text-left border-collapse">
                            <thead>
                                <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                    <th className="p-4 pl-6">Session ID</th>
                                    <th className="p-4">User ID</th>
                                    <th className="p-4">Computer ID</th>
                                    <th className="p-4">Start Time</th>
                                    <th className="p-4">End Time</th>
                                    <th className="p-4 pr-6">Status</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 bg-white/50">
                                {(!bookings || bookings.length === 0) ? (
                                    <tr><td colSpan="6" className="p-8 text-center text-slate-500">No history found.</td></tr>
                                ) : bookings.map((booking, i) => (
                                    <tr key={i} className="hover:bg-purple-50/50 transition-colors">
                                        <td className="p-4 pl-6 font-mono text-xs text-slate-400">#{booking.id}</td>
                                        <td className="p-4 font-bold text-slate-700 text-sm">
                                            {booking.user_name || (booking.user ? (booking.user.full_name || booking.user.username) : `User #${booking.user_id}`)}
                                        </td>
                                        <td className="p-4 text-sm text-slate-600">
                                            <div className="flex items-center gap-2">
                                                <span className="material-symbols-outlined text-sm text-slate-400">desktop_windows</span>
                                                {booking.computer_name || (booking.computer ? booking.computer.name : `COM-${booking.computer_id}`)}
                                            </div>
                                        </td>
                                        <td className="p-4 text-sm text-slate-600">{formatDate(booking.start_time)}</td>
                                        <td className="p-4 text-sm text-slate-600">{formatDate(booking.end_time)}</td>
                                        <td className="p-4 pr-6">
                                            <span className={`px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider ${booking.status === 'confirmed' ? 'bg-emerald-100 text-emerald-600' : 'bg-slate-100 text-slate-500'
                                                }`}>
                                                {booking.status}
                                            </span>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    )}
                </div>
            </div>
        </div>
    );
}

export default AdminUsageHistory;
