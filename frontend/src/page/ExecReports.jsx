import React, { useEffect, useMemo, useState } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

const formatDateTime = value => {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('th-TH', { dateStyle: 'medium', timeStyle: 'short' });
};

const statusLabel = reason => {
    if (reason === 'normal' || reason === 'timeout') return 'Completed';
    if (reason === 'cancelled') return 'Cancelled';
    return reason ? `${reason[0].toUpperCase()}${reason.slice(1)}` : 'Ended';
};

function ExecReports({ period = 'all' }) {
    const { user } = useAuth();
    const [reportTab, setReportTab] = useState('history');
    const [history, setHistory] = useState([]);
    const [weeklyHistory, setWeeklyHistory] = useState([]);
    const [computers, setComputers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState(false);

    // Filters and pagination for history tab
    const [searchQuery, setSearchQuery] = useState('');
    const [currentPage, setCurrentPage] = useState(1);
    const pageSize = 15;

    useEffect(() => {
        if (!user) return;
        let cancelled = false;

        const fetchData = async () => {
            setLoading(true);
            try {
                // ponytail: reports aggregate the API's 100-row cap; add server aggregates if full-history metrics are needed.
                const weeklyRequest = period === 'week'
                    ? Promise.resolve(null)
                    : apiFetch('/admin/usage-history?period=week');
                const [historyRes, weeklyRes, computersRes] = await Promise.all([
                    apiFetch(`/admin/usage-history?period=${encodeURIComponent(period)}`),
                    weeklyRequest,
                    apiFetch('/computers')
                ]);
                if (!historyRes.ok || !computersRes.ok || (weeklyRes && !weeklyRes.ok)) {
                    throw new Error('Report request failed');
                }
                const historyRows = await historyRes.json();
                const weekRows = period === 'week'
                    ? historyRows
                    : await weeklyRes.json();
                const computerRows = await computersRes.json();

                if (!cancelled) {
                    setHistory(Array.isArray(historyRows) ? historyRows : []);
                    setWeeklyHistory(Array.isArray(weekRows) ? weekRows : []);
                    setComputers(Array.isArray(computerRows) ? computerRows : []);
                    setLoadError(false);
                    setCurrentPage(1);
                }
            } catch (err) {
                if (!cancelled) {
                    setHistory([]);
                    setWeeklyHistory([]);
                    setComputers([]);
                    setLoadError(true);
                }
                console.error('Failed to load report data:', err);
            } finally {
                if (!cancelled) setLoading(false);
            }
        };

        fetchData();
        return () => { cancelled = true; };
    }, [user, period]);

    const weeklySummary = useMemo(() => {
        const days = new Map();
        for (const item of weeklyHistory) {
            const date = new Date(item.start_time);
            if (Number.isNaN(date.getTime())) continue;
            const key = `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
            if (!days.has(key)) {
                days.set(key, { date, sessions: 0, totalMinutes: 0, hours: {} });
            }
            const row = days.get(key);
            row.sessions += 1;
            row.totalMinutes += Number(item.duration_minutes) || 0;
            const hour = date.getHours();
            row.hours[hour] = (row.hours[hour] || 0) + 1;
        }
        return [...days.values()]
            .sort((a, b) => a.date - b.date)
            .map(row => {
                const peakHour = Object.entries(row.hours).sort((a, b) => b[1] - a[1])[0]?.[0];
                const startHour = Number(peakHour);
                return {
                    ...row,
                    label: row.date.toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric' }),
                    averageMinutes: Math.round(row.totalMinutes / row.sessions),
                    peak: peakHour === undefined ? '-' : `${String(startHour).padStart(2, '0')}:00-${String((startHour + 1) % 24).padStart(2, '0')}:00`
                };
            });
    }, [weeklyHistory]);

    const machineSummary = useMemo(() => {
        const machines = new Map();
        for (const item of history) {
            const name = item.computer_name || `Computer #${item.computer_id}`;
            const row = machines.get(name) || { name, sessions: 0, minutes: 0 };
            row.sessions += 1;
            row.minutes += Number(item.duration_minutes) || 0;
            machines.set(name, row);
        }
        return [...machines.values()].sort((a, b) => b.sessions - a.sessions).slice(0, 10);
    }, [history]);

    // Client-side filtering for history tab
    const filteredHistory = useMemo(() => {
        if (!searchQuery.trim()) return history;
        const q = searchQuery.toLowerCase().trim();
        return history.filter(item =>
            (item.user_name && item.user_name.toLowerCase().includes(q)) ||
            (item.computer_name && item.computer_name.toLowerCase().includes(q)) ||
            (item.department && item.department.toLowerCase().includes(q)) ||
            (item.termination_reason && item.termination_reason.toLowerCase().includes(q))
        );
    }, [history, searchQuery]);

    const totalPages = Math.max(1, Math.ceil(filteredHistory.length / pageSize));
    const paginatedHistory = useMemo(() => {
        const start = (currentPage - 1) * pageSize;
        return filteredHistory.slice(start, start + pageSize);
    }, [filteredHistory, currentPage, pageSize]);

    const onlineCount = computers.filter(computer => computer.is_online).length;
    const inUseCount = computers.filter(computer => computer.is_online && computer.status === 'in_use').length;
    const offlineCount = computers.length - onlineCount;
    const maintenanceCount = computers.filter(computer => computer.status === 'maintenance').length;
    const completedCount = history.filter(item => item.termination_reason === 'normal' || item.termination_reason === 'timeout').length;
    const cancelledCount = history.filter(item => item.termination_reason === 'cancelled').length;
    const otherEndCount = history.length - completedCount - cancelledCount;
    const maxWeeklySessions = Math.max(1, ...weeklySummary.map(day => day.sessions));
    const maxMachineSessions = Math.max(1, ...machineSummary.map(machine => machine.sessions));

    return (
        <div className="p-8 space-y-8 h-full overflow-y-auto custom-scrollbar">
            <div className="flex flex-wrap gap-2 bg-slate-100 p-1 rounded-xl w-fit">
                {[
                    ['history', 'Usage history'],
                    ['weekly', 'This week'],
                    ['machines', 'Computers']
                ].map(([tab, label]) => (
                    <button
                        key={tab}
                        type="button"
                        onClick={() => setReportTab(tab)}
                        aria-pressed={reportTab === tab}
                        className={`px-5 py-2.5 rounded-lg text-xs font-bold transition-all ${reportTab === tab ? 'bg-white text-violet-700 shadow-sm' : 'text-slate-500 hover:text-slate-700'}`}
                    >
                        {label}
                    </button>
                ))}
            </div>

            {loadError ? (
                <p role="alert" className="rounded-xl bg-rose-50 p-4 text-sm text-rose-700">Reports could not be loaded.</p>
            ) : loading ? (
                <div className="flex justify-center p-12" role="status" aria-label="Loading reports">
                    <div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-violet-500 animate-spin" />
                </div>
            ) : (
                <>
                    {reportTab === 'history' && (
                        <section className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm space-y-4">
                            <div className="grid grid-cols-2 md:grid-cols-4 border-b border-slate-100">
                                {[
                                    ['Records returned', history.length],
                                    ['Completed', completedCount],
                                    ['Cancelled', cancelledCount],
                                    ['Other end states', otherEndCount]
                                ].map(([label, value]) => (
                                    <div key={label} className="p-5 text-center">
                                        <p className="text-2xl font-black text-slate-800">{value}</p>
                                        <p className="text-xs font-semibold text-slate-500 mt-1">{label}</p>
                                    </div>
                                ))}
                            </div>

                            {/* Search and Page Size Filter */}
                            <div className="px-6 flex flex-col sm:flex-row items-center justify-between gap-3">
                                <div className="relative w-full sm:w-72">
                                    <span className="material-symbols-outlined absolute left-3 top-2.5 text-slate-400 text-lg">search</span>
                                    <input
                                        type="text"
                                        placeholder="Search records..."
                                        value={searchQuery}
                                        onChange={(e) => {
                                            setSearchQuery(e.target.value);
                                            setCurrentPage(1);
                                        }}
                                        className="w-full pl-9 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl text-xs focus:outline-none focus:ring-2 focus:ring-violet-500/20"
                                    />
                                </div>
                                <div className="text-xs text-slate-500">
                                    Showing {filteredHistory.length} of {history.length} records
                                </div>
                            </div>

                            <div className="overflow-x-auto">
                                <table className="w-full text-left border-collapse">
                                    <thead>
                                        <tr className="bg-slate-50 text-xs font-bold text-slate-500 uppercase">
                                            <th className="p-4 pl-6">User</th>
                                            <th className="p-4">Department</th>
                                            <th className="p-4">Computer</th>
                                            <th className="p-4">Start</th>
                                            <th className="p-4">End</th>
                                            <th className="p-4 text-center">Minutes</th>
                                            <th className="p-4 pr-6 text-right">Result</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-slate-100 bg-white/50">
                                        {paginatedHistory.map(item => (
                                            <tr key={item.id} className="text-sm text-slate-600 hover:bg-violet-50/30">
                                                <td className="p-4 pl-6 font-medium text-slate-800">{item.user_name || `User #${item.user_id}`}</td>
                                                <td className="p-4 text-slate-600">{item.department || '-'}</td>
                                                <td className="p-4 font-medium text-slate-700">{item.computer_name || `Computer #${item.computer_id}`}</td>
                                                <td className="p-4 whitespace-nowrap text-xs text-slate-500">{formatDateTime(item.start_time)}</td>
                                                <td className="p-4 whitespace-nowrap text-xs text-slate-500">{formatDateTime(item.end_time)}</td>
                                                <td className="p-4 text-center text-xs font-mono text-slate-700">{Number(item.duration_minutes) || 0}</td>
                                                <td className="p-4 pr-6 text-right font-semibold text-xs">
                                                    <span className={`px-2.5 py-0.5 rounded-full text-[10px] uppercase tracking-wider ${
                                                        item.termination_reason === 'cancelled'
                                                            ? 'bg-rose-50 text-rose-700 border border-rose-200'
                                                            : 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                                                    }`}>
                                                        {statusLabel(item.termination_reason)}
                                                    </span>
                                                </td>
                                            </tr>
                                        ))}
                                        {paginatedHistory.length === 0 && (
                                            <tr>
                                                <td colSpan="7" className="p-8 text-center text-slate-500">
                                                    {searchQuery ? 'No matching records.' : 'No usage records for this period.'}
                                                </td>
                                            </tr>
                                        )}
                                    </tbody>
                                </table>
                            </div>

                            {/* Pagination */}
                            {totalPages > 1 && (
                                <div className="border-t border-slate-100 px-6 py-3 flex items-center justify-between text-xs text-slate-500 bg-slate-50/40">
                                    <span>Page {currentPage} of {totalPages}</span>
                                    <div className="flex items-center gap-1.5">
                                        <button
                                            onClick={() => setCurrentPage(p => Math.max(1, p - 1))}
                                            disabled={currentPage === 1}
                                            className="px-2.5 py-1 rounded-lg border border-slate-200 bg-white font-bold disabled:opacity-40 hover:bg-slate-50"
                                        >
                                            Prev
                                        </button>
                                        <button
                                            onClick={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                                            disabled={currentPage === totalPages}
                                            className="px-2.5 py-1 rounded-lg border border-slate-200 bg-white font-bold disabled:opacity-40 hover:bg-slate-50"
                                        >
                                            Next
                                        </button>
                                    </div>
                                </div>
                            )}
                        </section>
                    )}

                    {reportTab === 'weekly' && (
                        <section className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                            <div className="p-6 border-b border-slate-100">
                                <h3 className="font-bold text-slate-800">Daily usage this week</h3>
                                <p className="text-xs text-slate-500 mt-1">Based on up to 100 latest usage records for the current week.</p>
                            </div>
                            <div className="overflow-x-auto">
                                <table className="w-full text-left border-collapse">
                                    <thead>
                                        <tr className="bg-slate-50 text-xs font-bold text-slate-500 uppercase">
                                            <th className="p-4">Day</th>
                                            <th className="p-4">Sessions</th>
                                            <th className="p-4">Average minutes</th>
                                            <th className="p-4">Peak hour</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-slate-100 bg-white/50">
                                        {weeklySummary.map(day => (
                                            <tr key={day.date.toISOString()} className="text-sm text-slate-600">
                                                <td className="p-4 font-semibold text-slate-700">{day.label}</td>
                                                <td className="p-4">
                                                    <div className="flex items-center gap-3">
                                                        <span>{day.sessions}</span>
                                                        <div className="w-24 h-1.5 rounded-full bg-slate-100 overflow-hidden">
                                                            <div className="h-full bg-violet-500" style={{ width: `${(day.sessions / maxWeeklySessions) * 100}%` }} />
                                                        </div>
                                                    </div>
                                                </td>
                                                <td className="p-4">{day.averageMinutes}</td>
                                                <td className="p-4">{day.peak}</td>
                                            </tr>
                                        ))}
                                        {weeklySummary.length === 0 && <tr><td colSpan="4" className="p-8 text-center text-slate-500">No usage records this week.</td></tr>}
                                    </tbody>
                                </table>
                            </div>
                        </section>
                    )}

                    {reportTab === 'machines' && (
                        <section className="space-y-6">
                            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                                {[
                                    ['Online', onlineCount, 'text-emerald-700 bg-emerald-50'],
                                    ['In use', inUseCount, 'text-violet-700 bg-violet-50'],
                                    ['Maintenance', maintenanceCount, 'text-amber-700 bg-amber-50'],
                                    ['Offline', offlineCount, 'text-slate-700 bg-slate-100']
                                ].map(([label, count, tone]) => (
                                    <div key={label} className="glass-card rounded-2xl p-5 shadow-sm">
                                        <p className="text-xs font-bold uppercase tracking-wider text-slate-500">{label}</p>
                                        <p className={`text-3xl font-black mt-2 inline-block px-3 py-1 rounded-xl ${tone}`}>{count}</p>
                                    </div>
                                ))}
                            </div>

                            <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                                <div className="p-6 border-b border-slate-100">
                                    <h3 className="font-bold text-slate-800">Top stations by recorded sessions</h3>
                                    <p className="text-xs text-slate-500 mt-1">Computers with the highest recorded traffic.</p>
                                </div>
                                <div className="overflow-x-auto">
                                    <table className="w-full text-left border-collapse">
                                        <thead>
                                            <tr className="bg-slate-50 text-xs font-bold text-slate-500 uppercase">
                                                <th className="p-4">Computer</th>
                                                <th className="p-4">Sessions</th>
                                                <th className="p-4">Total minutes</th>
                                            </tr>
                                        </thead>
                                        <tbody className="divide-y divide-slate-100 bg-white/50">
                                            {machineSummary.map(machine => (
                                                <tr key={machine.name} className="text-sm text-slate-600">
                                                    <td className="p-4 font-semibold text-slate-700">{machine.name}</td>
                                                    <td className="p-4">
                                                        <div className="flex items-center gap-3">
                                                            <span>{machine.sessions}</span>
                                                            <div className="w-24 h-1.5 rounded-full bg-slate-100 overflow-hidden">
                                                                <div className="h-full bg-violet-500" style={{ width: `${(machine.sessions / maxMachineSessions) * 100}%` }} />
                                                            </div>
                                                        </div>
                                                    </td>
                                                    <td className="p-4">{machine.minutes}</td>
                                                </tr>
                                            ))}
                                            {machineSummary.length === 0 && <tr><td colSpan="3" className="p-8 text-center text-slate-500">No computer records found.</td></tr>}
                                        </tbody>
                                    </table>
                                </div>
                            </div>
                        </section>
                    )}
                </>
            )}
        </div>
    );
}

export default ExecReports;
