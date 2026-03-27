import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';

function StaffDailyReport() {
    const { user } = useAuth();
    const [stats, setStats] = useState({
        total_users: 342, // Default mock fallback
        peak_hours: "11:00 - 14:00",
        avg_session: 45
    });

    useEffect(() => {
        const fetchStats = async () => {
            if (!user) return;
            try {
                const baseUrl = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
                    ? 'http://localhost:8000'
                    : `http://${window.location.hostname}:8000`;

                const response = await fetch(`${baseUrl}/dashboard/stats`, {
                    headers: { "Authorization": `Bearer ${user.token}` }
                });
                if (response.ok) {
                    const data = await response.json();
                    setStats(prev => ({
                        ...prev,
                        total_users: data.total_users || prev.total_users,
                        // Could calculate actual avg if backend supported it
                    }));
                }
            } catch (error) {
                console.error("Error fetching stats:", error);
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
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Total Users</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.total_users}</h3></div>
                    </div>
                    <div className="flex items-center gap-4 border-l border-slate-100 pl-8">
                        <div className="p-3 bg-amber-50 rounded-xl text-amber-500"><span className="material-symbols-outlined">bolt</span></div>
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Peak Hours (Est.)</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.peak_hours}</h3></div>
                    </div>
                    <div className="flex items-center gap-4 border-l border-slate-100 pl-8">
                        <div className="p-3 bg-emerald-50 rounded-xl text-emerald-500"><span className="material-symbols-outlined">avg_pace</span></div>
                        <div><p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Avg. Session Time</p><h3 className="text-2xl font-black text-slate-800 tracking-tight">{stats.avg_session} <span className="text-sm font-normal text-slate-400">mins</span></h3></div>
                    </div>
                </div>
            </div>

            {/* Issues Log */}
            <div className="glass-card rounded-2xl p-6 shadow-sm">
                <div className="flex items-center justify-between mb-6">
                    <h3 className="font-bold text-slate-800">Reported Issues</h3>
                    <button className="text-xs font-bold text-[#7c3aed] hover:underline">View All History</button>
                </div>

                <table className="w-full text-left border-collapse">
                    <thead>
                        <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                            <th className="p-4 rounded-tl-lg">Time</th>
                            <th className="p-4">Station</th>
                            <th className="p-4">Issue</th>
                            <th className="p-4">Reporter</th>
                            <th className="p-4 rounded-tr-lg">Status</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 text-sm">
                        {[
                            { time: '10:42 AM', station: 'A-05', issue: 'Keyboard not working', reporter: 'User 640...', status: 'Resolved' },
                            { time: '11:15 AM', station: 'C-02', issue: 'Internet connectivity lost', reporter: 'System', status: 'Pending' },
                            { time: '01:30 PM', station: 'B-11', issue: 'Screen flickering', reporter: 'User 630...', status: 'Investigating' },
                        ].map((issue, i) => (
                            <tr key={i} className="hover:bg-slate-50 transition-colors">
                                <td className="p-4 text-slate-500 font-medium">{issue.time}</td>
                                <td className="p-4 font-bold text-slate-700">{issue.station}</td>
                                <td className="p-4 text-slate-600">{issue.issue}</td>
                                <td className="p-4 text-slate-500">{issue.reporter}</td>
                                <td className="p-4">
                                    <span className={`px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider ${issue.status === 'Resolved' ? 'bg-emerald-100 text-emerald-600' :
                                            issue.status === 'Pending' ? 'bg-amber-100 text-amber-600' :
                                                'bg-blue-100 text-blue-600'
                                        }`}>
                                        {issue.status}
                                    </span>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </div>
    );
}

export default StaffDailyReport;
