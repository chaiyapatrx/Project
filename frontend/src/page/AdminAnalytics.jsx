import React, { useState, useEffect, useMemo } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

function AdminAnalytics() {
    const { user } = useAuth();
    const [stats, setStats] = useState({
        total_users: 0,
        total_computers: 0,
        active_bookings: 0,
        todays_bookings: 0,
        total_sessions: 0,
        avg_duration_min: 0,
        unique_users: 0,
        active_computers: 0,
        student_users: 0,
        staff_users: 0,
        other_users: 0
    });
    const [usageLogs, setUsageLogs] = useState([]);
    const [computers, setComputers] = useState([]);
    const [period, setPeriod] = useState('all'); // all, week, day
    const [loading, setLoading] = useState(true);
    const [loadError, setLoadError] = useState(false);

    useEffect(() => {
        let active = true;
        const fetchData = async () => {
            setLoading(true);
            try {
                const [statsRes, logsRes, compsRes] = await Promise.all([
                    apiFetch("/dashboard/stats"),
                    apiFetch(`/admin/usage-history?period=${encodeURIComponent(period)}`),
                    apiFetch("/computers")
                ]);
                if (!statsRes.ok || !logsRes.ok || !compsRes.ok) throw new Error('Analytics request failed');

                if (statsRes.ok) {
                    const data = await statsRes.json();
                    if (active) setStats(prev => ({ ...prev, ...data }));
                }
                if (logsRes.ok) {
                    const data = await logsRes.json();
                    if (active) setUsageLogs(Array.isArray(data) ? data : []);
                }
                if (compsRes.ok) {
                    const data = await compsRes.json();
                    if (active) setComputers(Array.isArray(data) ? data : []);
                }
                if (active) setLoadError(false);
            } catch (error) {
                console.error("Error fetching analytics data:", error);
                if (active) setLoadError(true);
            } finally {
                if (active) setLoading(false);
            }
        };

        fetchData();
        return () => { active = false; };
    }, [user, period]);

    // Hourly distribution computation (08:00 - 20:00)
    const hourlyData = useMemo(() => {
        const hours = Array.from({ length: 12 }, (_, i) => ({
            hour: i + 8,
            label: `${String(i + 8).padStart(2, '0')}:00`,
            count: 0
        }));

        usageLogs.forEach(log => {
            if (!log.start_time) return;
            const d = new Date(log.start_time);
            if (Number.isNaN(d.getTime())) return;
            const h = d.getHours();
            if (h >= 8 && h <= 19) {
                hours[h - 8].count += 1;
            }
        });

        const maxCount = Math.max(...hours.map(h => h.count), 1);
        return hours.map(h => ({
            ...h,
            percentage: Math.round((h.count / maxCount) * 100)
        }));
    }, [usageLogs]);

    // Computer status counts
    const computerStats = useMemo(() => {
        const total = computers.length || stats.total_computers || 0;
        let available = 0;
        let inUse = 0;
        let maintenance = 0;
        let offline = 0;

        computers.forEach(c => {
            if (!c.is_online) {
                offline += 1;
            } else if (c.status === 'in_use' || c.status === 'in-use') {
                inUse += 1;
            } else if (c.status === 'maintenance') {
                maintenance += 1;
            } else {
                available += 1;
            }
        });

        // Fallback to stats if computers array empty
        if (computers.length === 0) {
            inUse = stats.active_computers || stats.active_bookings || 0;
            available = Math.max(0, total - inUse);
        }

        return { total, available, inUse, maintenance, offline };
    }, [computers, stats]);

    // Role breakdown
    const roleStats = useMemo(() => {
        const student = stats.student_users || 0;
        const staff = stats.staff_users || 0;
        const other = stats.other_users || 0;
        const total = student + staff + other || stats.total_users || 1;

        return {
            student,
            staff,
            other,
            studentPct: Math.round((student / total) * 100),
            staffPct: Math.round((staff / total) * 100),
            otherPct: Math.round((other / total) * 100)
        };
    }, [stats]);

    const peakHour = useMemo(() => {
        let best = { label: 'None', count: 0 };
        hourlyData.forEach(h => {
            if (h.count > best.count) best = h;
        });
        return best;
    }, [hourlyData]);

    const utilizationRate = computerStats.total > 0
        ? Math.round((computerStats.inUse / computerStats.total) * 100)
        : 0;

    return (
        <div className="h-full overflow-hidden flex flex-col" aria-busy={loading}>
            {/* Header */}
            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <div>
                    <h1 className="text-lg font-bold text-slate-800">Analytics & Insights</h1>
                    <p className="text-xs text-slate-500">Real-time lab operations & historical utilization</p>
                </div>
                <div className="flex items-center gap-1 bg-slate-100 p-1 rounded-xl text-xs font-bold">
                    {[
                        { id: 'day', label: 'Today' },
                        { id: 'week', label: 'This Week' },
                        { id: 'all', label: 'All Time' }
                    ].map(btn => (
                        <button
                            key={btn.id}
                            onClick={() => setPeriod(btn.id)}
                            className={`px-3 py-1.5 rounded-lg transition-all ${
                                period === btn.id
                                    ? 'bg-white text-violet-700 shadow-sm'
                                    : 'text-slate-600 hover:text-slate-900'
                            }`}
                        >
                            {btn.label}
                        </button>
                    ))}
                </div>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8 space-y-8">
                {loadError && <p role="alert" className="rounded-xl bg-rose-50 p-4 text-sm text-rose-700">Analytics could not be refreshed. Displayed values may be unavailable or out of date.</p>}
                {/* 1. Stat Cards Grid */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    {[
                        { title: 'Total Stations', value: computerStats.total, sub: `${computerStats.available} available now`, icon: 'computer', color: 'text-violet-600', border: 'border-l-violet-600', bgIcon: 'bg-violet-50' },
                        { title: 'Registered Users', value: stats.total_users, sub: `${roleStats.student} students, ${roleStats.staff} staff`, icon: 'group', color: 'text-blue-600', border: 'border-l-blue-600', bgIcon: 'bg-blue-50' },
                        { title: 'Active Sessions', value: computerStats.inUse, sub: `${utilizationRate}% utilization rate`, icon: 'desktop_windows', color: 'text-emerald-600', border: 'border-l-emerald-600', bgIcon: 'bg-emerald-50' },
                        { title: 'Completed Sessions', value: stats.total_sessions || usageLogs.length, sub: `Avg. ${Math.round(stats.avg_duration_min || 0)} mins / session`, icon: 'history', color: 'text-amber-600', border: 'border-l-amber-600', bgIcon: 'bg-amber-50' },
                    ].map((stat, idx) => (
                        <div key={idx} className={`glass-card p-6 rounded-2xl shadow-sm border-l-4 ${stat.border}`}>
                            <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">{stat.title}</p>
                            <div className="flex items-end justify-between">
                                <div>
                                    <h3 className="text-3xl font-black text-slate-800">{stat.value}</h3>
                                    <p className={`text-xs font-bold flex items-center gap-1 mt-1 ${stat.color}`}>
                                        {stat.sub}
                                    </p>
                                </div>
                                <div className={`p-3 rounded-xl ${stat.bgIcon} ${stat.color}`}>
                                    <span className="material-symbols-outlined text-2xl">{stat.icon}</span>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>

                {/* 2. Visual Charts Row */}
                <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
                    {/* Hourly Usage Bar Chart */}
                    <div className="lg:col-span-8 glass-card rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                        <div className="flex items-center justify-between mb-6">
                            <div>
                                <h3 className="font-bold text-slate-800 text-base">Hourly Station Activity (08:00 – 20:00)</h3>
                                <p className="text-xs text-slate-500 mt-0.5">Session launch distribution across opening hours</p>
                            </div>
                            <span className="px-3 py-1 rounded-full bg-violet-50 text-violet-700 text-xs font-bold border border-violet-100">
                                Peak: {peakHour.count > 0 ? `${peakHour.label} (${peakHour.count} sessions)` : 'No activity'}
                            </span>
                        </div>

                        {/* Bar Graph */}
                        <div className="h-56 flex items-end justify-between gap-2 pt-6 pb-2 px-2 border-b border-slate-100">
                            {hourlyData.map(h => (
                                <div key={h.hour} className="flex-1 flex flex-col items-center h-full justify-end group relative">
                                    {/* Tooltip */}
                                    <div className="absolute -top-8 opacity-0 group-hover:opacity-100 transition-opacity bg-slate-900 text-white text-[11px] font-bold py-1 px-2 rounded-lg pointer-events-none whitespace-nowrap z-10">
                                        {h.label}: {h.count} sessions
                                    </div>
                                    {/* Bar */}
                                    <div className="w-full max-w-[28px] bg-slate-100 rounded-t-lg overflow-hidden flex flex-col justify-end h-full">
                                        <div
                                            className="w-full bg-gradient-to-t from-violet-600 to-indigo-500 rounded-t-lg transition-all duration-500 group-hover:from-violet-700 group-hover:to-indigo-600"
                                            style={{ height: `${Math.max(h.percentage, h.count > 0 ? 8 : 2)}%` }}
                                        />
                                    </div>
                                </div>
                            ))}
                        </div>

                        {/* X-Axis Labels */}
                        <div className="flex items-center justify-between gap-2 px-2 pt-2 text-[11px] font-bold text-slate-400">
                            {hourlyData.map(h => (
                                <span key={h.hour} className="flex-1 text-center truncate">{h.label}</span>
                            ))}
                        </div>
                    </div>

                    {/* Station Status Donut / Distribution */}
                    <div className="lg:col-span-4 glass-card rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                        <div>
                            <h3 className="font-bold text-slate-800 text-base">Station Fleet Status</h3>
                            <p className="text-xs text-slate-500 mt-0.5">Real-time breakdown of {computerStats.total} total computers</p>
                        </div>

                        {/* Progress Multi-Bar */}
                        <div className="my-6">
                            <div className="h-4 w-full bg-slate-100 rounded-full overflow-hidden flex shadow-inner">
                                {computerStats.total > 0 ? (
                                    <>
                                        <div style={{ width: `${(computerStats.available / computerStats.total) * 100}%` }} className="bg-emerald-500 h-full transition-all" title="Available" />
                                        <div style={{ width: `${(computerStats.inUse / computerStats.total) * 100}%` }} className="bg-violet-600 h-full transition-all" title="In Use" />
                                        <div style={{ width: `${(computerStats.maintenance / computerStats.total) * 100}%` }} className="bg-amber-500 h-full transition-all" title="Maintenance" />
                                        <div style={{ width: `${(computerStats.offline / computerStats.total) * 100}%` }} className="bg-slate-300 h-full transition-all" title="Offline" />
                                    </>
                                ) : (
                                    <div className="w-full bg-slate-200 h-full" />
                                )}
                            </div>
                        </div>

                        {/* Legend items */}
                        <div className="space-y-3">
                            {[
                                { label: 'Available for Booking', count: computerStats.available, color: 'bg-emerald-500', text: 'text-emerald-700' },
                                { label: 'In Use (Active Session)', count: computerStats.inUse, color: 'bg-violet-600', text: 'text-violet-700' },
                                { label: 'Maintenance Mode', count: computerStats.maintenance, color: 'bg-amber-500', text: 'text-amber-700' },
                                { label: 'Agent Offline', count: computerStats.offline, color: 'bg-slate-300', text: 'text-slate-600' }
                            ].map((item, idx) => (
                                <div key={idx} className="flex items-center justify-between text-xs py-1 border-b border-slate-50 last:border-0">
                                    <div className="flex items-center gap-2">
                                        <span className={`w-3 h-3 rounded-md ${item.color}`} />
                                        <span className="font-medium text-slate-700">{item.label}</span>
                                    </div>
                                    <span className={`font-bold ${item.text}`}>{item.count}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                {/* 3. Bottom Row: User Roles & Operational Performance */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
                    {/* User Accounts Distribution */}
                    <div className="glass-card rounded-2xl p-6 shadow-sm">
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="font-bold text-slate-800 text-base">User Base Distribution</h3>
                            <span className="text-xs font-bold text-slate-400">Total: {stats.total_users} users</span>
                        </div>
                        <div className="space-y-4">
                            <div>
                                <div className="flex justify-between text-xs font-bold text-slate-700 mb-1">
                                    <span>Students</span>
                                    <span>{roleStats.student} ({roleStats.studentPct}%)</span>
                                </div>
                                <div className="h-2.5 w-full bg-slate-100 rounded-full overflow-hidden">
                                    <div className="h-full bg-violet-600 rounded-full transition-all" style={{ width: `${roleStats.studentPct}%` }} />
                                </div>
                            </div>

                            <div>
                                <div className="flex justify-between text-xs font-bold text-slate-700 mb-1">
                                    <span>Staff & Lab Supervisors</span>
                                    <span>{roleStats.staff} ({roleStats.staffPct}%)</span>
                                </div>
                                <div className="h-2.5 w-full bg-slate-100 rounded-full overflow-hidden">
                                    <div className="h-full bg-blue-500 rounded-full transition-all" style={{ width: `${roleStats.staffPct}%` }} />
                                </div>
                            </div>

                            <div>
                                <div className="flex justify-between text-xs font-bold text-slate-700 mb-1">
                                    <span>Administrators & Executives</span>
                                    <span>{roleStats.other} ({roleStats.otherPct}%)</span>
                                </div>
                                <div className="h-2.5 w-full bg-slate-100 rounded-full overflow-hidden">
                                    <div className="h-full bg-slate-400 rounded-full transition-all" style={{ width: `${roleStats.otherPct}%` }} />
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Operational KPIs */}
                    <div className="glass-card rounded-2xl p-6 shadow-sm flex flex-col justify-between">
                        <h3 className="font-bold text-slate-800 text-base mb-4">Lab Efficiency Insights</h3>
                        <div className="grid grid-cols-2 gap-4">
                            <div className="p-4 rounded-xl bg-purple-50/60 border border-purple-100">
                                <p className="text-xs font-bold text-violet-700 uppercase">Average Session Length</p>
                                <p className="text-2xl font-black text-slate-800 mt-2">{Math.round(stats.avg_duration_min || 0)} <span className="text-sm font-normal text-slate-500">mins</span></p>
                                <p className="text-[11px] text-slate-500 mt-1">Calculated from completed sessions</p>
                            </div>

                            <div className="p-4 rounded-xl bg-emerald-50/60 border border-emerald-100">
                                <p className="text-xs font-bold text-emerald-700 uppercase">Lab Utilization</p>
                                <p className="text-2xl font-black text-slate-800 mt-2">{utilizationRate}%</p>
                                <p className="text-[11px] text-slate-500 mt-1">{computerStats.inUse} of {computerStats.total} stations in use</p>
                            </div>

                            <div className="p-4 rounded-xl bg-blue-50/60 border border-blue-100">
                                <p className="text-xs font-bold text-blue-700 uppercase">Distinct Users</p>
                                <p className="text-2xl font-black text-slate-800 mt-2">{stats.unique_users || 0}</p>
                                <p className="text-[11px] text-slate-500 mt-1">Users who logged in to stations</p>
                            </div>

                            <div className="p-4 rounded-xl bg-amber-50/60 border border-amber-100">
                                <p className="text-xs font-bold text-amber-700 uppercase">Today's Bookings</p>
                                <p className="text-2xl font-black text-slate-800 mt-2">{stats.todays_bookings || 0}</p>
                                <p className="text-[11px] text-slate-500 mt-1">Reservations placed today</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default AdminAnalytics;
