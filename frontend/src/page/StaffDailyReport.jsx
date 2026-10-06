import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

function StaffDailyReport() {
    const { user } = useAuth();
    const [stats, setStats] = useState({
        total_users: 0,
        active_computers: 0,
        avg_session: 0
    });
    const [recentLogs, setRecentLogs] = useState([]);

    useEffect(() => {
        const fetchStats = async () => {
            if (!user) return;
            try {
                const [statsRes, logsRes] = await Promise.all([
                    apiFetch("/dashboard/stats", { user }),
                    apiFetch("/admin/usage-history?period=day", { user })
                ]);

                if (statsRes.ok) {
                    const data = await statsRes.json();
                    setStats({
                        total_users: data.total_users || 0,
                        avg_session: data.avg_duration_min || 0,
                        active_computers: data.active_computers || 0
                    });
                }

                if (logsRes.ok) {
                    const logs = await logsRes.json();
                    setRecentLogs(Array.isArray(logs) ? logs.filter(log => log.end_time).slice(0, 10) : []);
                }
            } catch (error) {
                console.error("Error fetching report data:", error);
            }
        };

        fetchStats();
    }, [user]);

    return (
        <div className="p-8 h-full overflow-y-auto custom-scrollbar">
            <h2 className="text-xl font-bold text-slate-800 mb-6">Daily Operation Report</h2>

            {/* Summary Cards */}
            <div className="glass-card p-6 rounded-2xl shadow-sm bg-gradient-to-br from-white to-purple-50/30 text-left mb-8">
                <div className="flex items-center justify-between mb-6">
                    <h2 className="text-sm font-bold text-slate-800">Overview</h2>
                    <span className="text-xs text-slate-400 font-medium italic tracking-tight">Today, {new Date().toLocaleDateString()}</span>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
                    <div className="flex items-center gap-4">
                        <div className="p-3 bg-[#7c3aed]/10 rounded-xl text-[#7c3aed]"><span className="material-symbols-outlined">groups</span></div>
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Registered Users</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.total_users}</h3></div>
                    </div>
                    <div className="flex items-center gap-4 border-l border-slate-100 pl-8">
                        <div className="p-3 bg-amber-50 rounded-xl text-amber-500"><span className="material-symbols-outlined">bolt</span></div>
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Stations In Use</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.active_computers}</h3></div>
                    </div>
                    <div className="flex items-center gap-4 border-l border-slate-100 pl-8">
                        <div className="p-3 bg-emerald-50 rounded-xl text-emerald-500"><span className="material-symbols-outlined">avg_pace</span></div>
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Historical Avg. Session</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.avg_session} <span className="text-sm font-normal text-slate-400">mins</span></h3></div>
                    </div>
                </div>
            </div>

            {/* Recent Completed Sessions Log */}
            <div className="glass-card rounded-2xl p-6 shadow-sm">
                <div className="flex items-center justify-between mb-6">
                    <h3 className="font-bold text-slate-800">Recent Completed Sessions Today</h3>
                </div>

                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                            <th className="p-4 rounded-tl-lg">Start Time</th>
                            <th className="p-4">Station</th>
                            <th className="p-4">User</th>
                            <th className="p-4">Department</th>
                            <th className="p-4">Duration</th>
                            <th className="p-4 rounded-tr-lg">Status</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 text-sm">
                        {recentLogs.length === 0 ? (
                            <tr>
                                <td colSpan="6" className="p-8 text-center text-slate-400 text-sm">
                                    No completed sessions recorded yet.
                                </td>
                            </tr>
                        ) : (
                            recentLogs.map((log, i) => (
                                <tr key={log.id || i} className="hover:bg-slate-50 transition-colors">
                                    <td className="p-4 text-slate-500 font-medium">
                                        {new Date(log.start_time).toLocaleTimeString('th-TH', { hour: '2-digit', minute: '2-digit' })}
                                    </td>
                                    <td className="p-4 font-bold text-slate-700">{log.computer_name || `COM-${log.computer_id}`}</td>
                                    <td className="p-4 text-slate-700">{log.user_name || `User #${log.user_id}`}</td>
                                    <td className="p-4 text-slate-500">{log.department || '-'}</td>
                                    <td className="p-4 font-bold text-[#7c3aed]">{log.duration_minutes || 0} mins</td>
                                    <td className="p-4">
                                        <span className={`px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider ${
                                            log.termination_reason === 'normal' ? 'bg-emerald-100 text-emerald-600' : 'bg-slate-100 text-slate-600'
                                        }`}>
                                            {log.termination_reason || 'completed'}
                                        </span>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>
        </div>
    );
}

export default StaffDailyReport;
