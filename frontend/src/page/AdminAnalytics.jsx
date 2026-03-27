import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';

function AdminAnalytics() {
    const { user } = useAuth();
    const [stats, setStats] = useState({
        total_users: 0,
        total_computers: 0,
        active_bookings: 0,
        todays_bookings: 0
    });

    useEffect(() => {
        const fetchStats = async () => {
            try {
                const response = await fetch("http://localhost:8000/dashboard/stats", {
                    headers: { "Authorization": `Bearer ${user.token}` }
                });
                if (response.ok) {
                    const data = await response.json();
                    setStats(data);
                }
            } catch (error) {
                console.error("Error fetching stats:", error);
            }
        };
        fetchStats();
    }, [user]);

    return (
        <div className="h-full overflow-hidden flex flex-col">
            {/* Header */}
            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">Analytics & Reports</h1>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8 space-y-8">

                {/* 1. Stat Cards Grid */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    {[
                        { title: 'Total Computers', value: stats.total_computers, sub: 'Total registered stations', icon: 'computer', color: 'text-purple-500', border: 'border-l-purple-500', bgIcon: 'bg-purple-50', textIcon: 'text-purple-500' },
                        { title: 'Total Users', value: stats.total_users, sub: 'Registered accounts', icon: 'group', color: 'text-blue-500', border: 'border-l-blue-500', bgIcon: 'bg-blue-50', textIcon: 'text-blue-500' },
                        { title: 'Active Bookings', value: stats.active_bookings, sub: 'Currently confirmed', icon: 'calendar_today', color: 'text-emerald-500', border: 'border-l-emerald-500', bgIcon: 'bg-emerald-50', textIcon: 'text-emerald-500' },
                        { title: 'Today\'s Bookings', value: stats.todays_bookings, sub: 'Approximate usage', icon: 'history', color: 'text-orange-500', border: 'border-l-orange-500', bgIcon: 'bg-orange-50', textIcon: 'text-orange-500' },
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
                                <div className={`p-3 rounded-xl ${stat.bgIcon} ${stat.textIcon}`}>
                                    <span className="material-symbols-outlined">{stat.icon}</span>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>

                {/* Placeholder Charts (Static for now as requested just real Top Level Data) */}
                <div className="grid grid-cols-12 gap-8">
                    <div className="col-span-12 glass-card rounded-2xl p-8 shadow-sm flex flex-col items-center justify-center text-center py-24 bg-slate-50/50 border-dashed border-2 border-slate-200">
                        <div className="w-16 h-16 bg-slate-100 rounded-full flex items-center justify-center mb-4 text-slate-400">
                            <span className="material-symbols-outlined text-3xl">bar_chart</span>
                        </div>
                        <h4 className="text-xl font-bold text-slate-700">Detailed Analytics Coming Soon</h4>
                        <p className="text-slate-500 max-w-md mx-auto mt-2">Real-time charts and historical trends will be available in the next update. The top-level statistics above are live data.</p>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default AdminAnalytics;
