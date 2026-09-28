import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { toCSV } from '../csv';

function AdminUsageHistory() {
    const { user } = useAuth();
    const [sessions, setSessions] = useState([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [retryCount, setRetryCount] = useState(0);

    useEffect(() => {
        let active = true;
        const fetchHistory = async () => {
            setLoading(true);
            try {
                const response = await apiFetch("/admin/usage-history?period=all", { user });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                if (!Array.isArray(data)) throw new Error("Unexpected usage history response");
                if (active) {
                    setSessions(data);
                    setLoadError("");
                }
            } catch (error) {
                console.error("Error fetching usage history:", error);
                if (active) setLoadError("Unable to load usage history. Check the server connection and try again.");
            } finally {
                if (active) setLoading(false);
            }
        };

        if (user) fetchHistory();
        return () => { active = false; };
    }, [user, retryCount]);

    const exportCSV = () => {
        if (sessions.length === 0) return;
        const rows = sessions.map(session => [
            session.id,
            session.user_name || session.user_id,
            session.department || '-',
            session.computer_name || session.computer_id,
            session.start_time,
            session.end_time || (session.session_ends_at ? 'Active' : '-'),
            session.duration_minutes || 0,
            session.termination_reason || (session.end_time ? 'completed' : 'active')
        ]);
        const blob = new Blob([toCSV(["Session ID", "User", "Department", "Station", "Start Time", "End Time", "Duration Minutes", "Result"], rows)], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.setAttribute("href", url);
        link.setAttribute("download", `usage_history_${new Date().toISOString().slice(0,10)}.csv`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    const formatDate = (dateString) => {
        if (!dateString) return '—';
        const date = new Date(dateString);
        if (Number.isNaN(date.getTime())) return '—';
        return date.toLocaleString('en-US', {
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
                        disabled={sessions.length === 0}
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
                    ) : loadError ? (
                        <div className="p-8 text-center text-rose-700" role="alert">
                            <p>{loadError}</p>
                            <button onClick={() => setRetryCount(count => count + 1)} className="mt-3 font-bold underline">Try again</button>
                        </div>
                    ) : (
                        <table className="w-full text-left border-collapse">
                            <thead>
                                <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                    <th className="p-4 pl-6">Session ID</th>
                                    <th className="p-4">User</th>
                                    <th className="p-4">Department</th>
                                    <th className="p-4">Station</th>
                                    <th className="p-4">Start Time</th>
                                    <th className="p-4">End Time</th>
                                    <th className="p-4">Minutes</th>
                                    <th className="p-4 pr-6">Result</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 bg-white/50">
                                {sessions.length === 0 ? (
                                    <tr><td colSpan="8" className="p-8 text-center text-slate-500">No usage records found.</td></tr>
                                ) : sessions.map((session) => (
                                    <tr key={session.id} className="hover:bg-purple-50/50 transition-colors">
                                        <td className="p-4 pl-6 font-mono text-xs text-slate-400">#{session.id}</td>
                                        <td className="p-4 font-bold text-slate-700 text-sm">
                                            {session.user_name || `User #${session.user_id}`}
                                        </td>
                                        <td className="p-4 text-sm text-slate-600">{session.department || '—'}</td>
                                        <td className="p-4 text-sm text-slate-600">
                                            <div className="flex items-center gap-2">
                                                <span className="material-symbols-outlined text-sm text-slate-400">desktop_windows</span>
                                                {session.computer_name || `COM-${session.computer_id}`}
                                            </div>
                                        </td>
                                        <td className="p-4 text-sm text-slate-600">{formatDate(session.start_time)}</td>
                                        <td className="p-4 text-sm text-slate-600">{session.end_time ? formatDate(session.end_time) : session.session_ends_at ? 'Active' : '—'}</td>
                                        <td className="p-4 text-sm text-slate-600">{Number(session.duration_minutes) || 0}</td>
                                        <td className="p-4 pr-6">
                                            <span className="px-2 py-1 rounded bg-slate-100 text-slate-600 text-[10px] font-bold uppercase tracking-wider">
                                                {session.termination_reason || (session.end_time ? 'completed' : 'active')}
                                            </span>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    )}
                    {!loading && !loadError && <p className="border-t border-slate-100 px-5 py-3 text-xs text-slate-400">Showing up to 100 latest usage records.</p>}
                </div>
            </div>
        </div>
    );
}

export default AdminUsageHistory;
