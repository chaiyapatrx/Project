import React, { useState, useEffect, useMemo } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { toCSV } from '../csv';

function AdminUsageHistory() {
    const { user } = useAuth();
    const [sessions, setSessions] = useState([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState("");
    const [retryCount, setRetryCount] = useState(0);

    // Filters and Pagination
    const [period, setPeriod] = useState('all'); // all, day, week, month
    const [searchQuery, setSearchQuery] = useState('');
    const [currentPage, setCurrentPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);

    useEffect(() => {
        let active = true;
        const fetchHistory = async () => {
            setLoading(true);
            try {
                const response = await apiFetch(`/admin/usage-history?period=${encodeURIComponent(period)}`, { user });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const data = await response.json();
                if (!Array.isArray(data)) throw new Error("Unexpected usage history response");
                if (active) {
                    setSessions(data);
                    setLoadError("");
                    setCurrentPage(1);
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
    }, [user, period, retryCount]);

    // Client-side search & filtering
    const filteredSessions = useMemo(() => {
        if (!searchQuery.trim()) return sessions;
        const q = searchQuery.toLowerCase().trim();
        return sessions.filter(s =>
            (s.user_name && s.user_name.toLowerCase().includes(q)) ||
            (s.computer_name && s.computer_name.toLowerCase().includes(q)) ||
            (s.department && s.department.toLowerCase().includes(q)) ||
            (s.termination_reason && s.termination_reason.toLowerCase().includes(q)) ||
            String(s.id).includes(q)
        );
    }, [sessions, searchQuery]);

    // Pagination slice
    const totalPages = Math.max(1, Math.ceil(filteredSessions.length / pageSize));
    const paginatedSessions = useMemo(() => {
        const start = (currentPage - 1) * pageSize;
        return filteredSessions.slice(start, start + pageSize);
    }, [filteredSessions, currentPage, pageSize]);

    const exportCSV = () => {
        const target = filteredSessions.length > 0 ? filteredSessions : sessions;
        if (target.length === 0) return;
        const rows = target.map(session => [
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
        link.setAttribute("download", `usage_history_${new Date().toISOString().slice(0, 10)}.csv`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
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

    const renderResultBadge = (session) => {
        const reason = session.termination_reason || (session.end_time ? 'completed' : 'active');
        let color = 'bg-slate-100 text-slate-700';
        if (reason === 'normal' || reason === 'completed') color = 'bg-emerald-50 text-emerald-700 border border-emerald-200';
        else if (reason === 'active') color = 'bg-blue-50 text-blue-700 border border-blue-200 animate-pulse';
        else if (reason === 'timeout') color = 'bg-amber-50 text-amber-700 border border-amber-200';
        else if (reason === 'cancelled' || reason === 'force_logout') color = 'bg-rose-50 text-rose-700 border border-rose-200';

        return (
            <span className={`px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${color}`}>
                {reason}
            </span>
        );
    };

    return (
        <div className="h-full overflow-hidden flex flex-col">
            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <div>
                    <h1 className="text-lg font-bold text-slate-800">Station Usage History</h1>
                    <p className="text-xs text-slate-500">Audit logs and station occupation records</p>
                </div>
                <div className="flex items-center gap-3">
                    <button
                        onClick={exportCSV}
                        disabled={filteredSessions.length === 0}
                        className="bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 px-4 py-2 rounded-lg text-sm font-bold flex items-center gap-2 transition-all disabled:opacity-50"
                    >
                        <span className="material-symbols-outlined text-lg">download</span>
                        <span>Export CSV</span>
                    </button>
                </div>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8 space-y-4">
                {/* Filter and Search Bar */}
                <div className="flex flex-col sm:flex-row gap-3 items-center justify-between">
                    <div className="relative w-full sm:w-80">
                        <span className="material-symbols-outlined absolute left-3 top-2.5 text-slate-400 text-lg">search</span>
                        <input
                            type="text"
                            placeholder="Filter by user, station, department..."
                            value={searchQuery}
                            onChange={(e) => {
                                setSearchQuery(e.target.value);
                                setCurrentPage(1);
                            }}
                            className="w-full pl-9 pr-4 py-2 bg-white border border-slate-200 rounded-xl text-xs focus:outline-none focus:ring-2 focus:ring-violet-500/20 focus:border-violet-500"
                        />
                    </div>

                    <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
                        <div className="flex items-center bg-slate-100 p-1 rounded-xl text-xs font-bold">
                            {[
                                { id: 'all', label: 'All' },
                                { id: 'day', label: 'Today' },
                                { id: 'week', label: 'This Week' },
                                { id: 'month', label: 'This Month' }
                            ].map(p => (
                                <button
                                    key={p.id}
                                    onClick={() => setPeriod(p.id)}
                                    className={`px-3 py-1.5 rounded-lg transition-all ${
                                        period === p.id ? 'bg-white text-violet-700 shadow-sm' : 'text-slate-600 hover:text-slate-900'
                                    }`}
                                >
                                    {p.label}
                                </button>
                            ))}
                        </div>

                        <select
                            value={pageSize}
                            onChange={(e) => {
                                setPageSize(Number(e.target.value));
                                setCurrentPage(1);
                            }}
                            className="bg-white border border-slate-200 text-slate-700 text-xs font-bold rounded-xl px-2.5 py-2 focus:outline-none focus:ring-2 focus:ring-violet-500/20"
                        >
                            <option value={10}>10 / page</option>
                            <option value={20}>20 / page</option>
                            <option value={50}>50 / page</option>
                        </select>
                    </div>
                </div>

                {/* Table Container */}
                <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                    {loading ? (
                        <div className="flex justify-center p-12">
                            <div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-violet-600 animate-spin"></div>
                        </div>
                    ) : loadError ? (
                        <div className="p-8 text-center text-rose-700" role="alert">
                            <p>{loadError}</p>
                            <button onClick={() => setRetryCount(count => count + 1)} className="mt-3 font-bold underline">Try again</button>
                        </div>
                    ) : (
                        <table className="w-full text-left border-collapse">
                            <thead>
                                <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                    <th className="p-4 pl-6">ID</th>
                                    <th className="p-4">User</th>
                                    <th className="p-4">Department</th>
                                    <th className="p-4">Station</th>
                                    <th className="p-4">Start Time</th>
                                    <th className="p-4">End Time</th>
                                    <th className="p-4 text-center">Duration</th>
                                    <th className="p-4 pr-6 text-right">Status</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 bg-white/50 text-sm">
                                {paginatedSessions.length === 0 ? (
                                    <tr>
                                        <td colSpan="8" className="p-8 text-center text-slate-500">
                                            {searchQuery ? 'No matching records found.' : 'No usage records available.'}
                                        </td>
                                    </tr>
                                ) : (
                                    paginatedSessions.map((session) => (
                                        <tr key={session.id} className="hover:bg-purple-50/30 transition-colors">
                                            <td className="p-4 pl-6 font-mono text-xs text-slate-400">#{session.id}</td>
                                            <td className="p-4 font-bold text-slate-800">
                                                {session.user_name || `User #${session.user_id}`}
                                            </td>
                                            <td className="p-4 text-slate-600">{session.department || '—'}</td>
                                            <td className="p-4 text-slate-600">
                                                <div className="flex items-center gap-2">
                                                    <span className="material-symbols-outlined text-sm text-slate-400">desktop_windows</span>
                                                    <span className="font-medium text-slate-800">{session.computer_name || `COM-${session.computer_id}`}</span>
                                                </div>
                                            </td>
                                            <td className="p-4 text-xs text-slate-600">{formatDate(session.start_time)}</td>
                                            <td className="p-4 text-xs text-slate-600">
                                                {session.end_time ? formatDate(session.end_time) : session.session_ends_at ? (
                                                    <span className="text-emerald-600 font-bold">Active</span>
                                                ) : '—'}
                                            </td>
                                            <td className="p-4 text-center text-xs font-mono text-slate-700">
                                                {Number(session.duration_minutes) || 0}m
                                            </td>
                                            <td className="p-4 pr-6 text-right">
                                                {renderResultBadge(session)}
                                            </td>
                                        </tr>
                                    ))
                                )}
                            </tbody>
                        </table>
                    )}

                    {/* Pagination Bar */}
                    {!loading && !loadError && (
                        <div className="border-t border-slate-100 px-6 py-3 flex items-center justify-between text-xs text-slate-500 bg-slate-50/40">
                            <div>
                                Showing <span className="font-bold text-slate-700">{filteredSessions.length > 0 ? (currentPage - 1) * pageSize + 1 : 0}</span> to{' '}
                                <span className="font-bold text-slate-700">{Math.min(currentPage * pageSize, filteredSessions.length)}</span> of{' '}
                                <span className="font-bold text-slate-700">{filteredSessions.length}</span> records
                            </div>
                            {totalPages > 1 && (
                                <div className="flex items-center gap-1.5">
                                    <button
                                        onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                                        disabled={currentPage === 1}
                                        className="px-2.5 py-1 rounded-lg border border-slate-200 bg-white font-bold disabled:opacity-40 hover:bg-slate-50"
                                    >
                                        Prev
                                    </button>
                                    <span className="px-2 font-bold text-slate-700">
                                        Page {currentPage} of {totalPages}
                                    </span>
                                    <button
                                        onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                                        disabled={currentPage === totalPages}
                                        className="px-2.5 py-1 rounded-lg border border-slate-200 bg-white font-bold disabled:opacity-40 hover:bg-slate-50"
                                    >
                                        Next
                                    </button>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

export default AdminUsageHistory;
