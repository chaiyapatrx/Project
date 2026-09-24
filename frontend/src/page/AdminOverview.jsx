import React, { useEffect, useState } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

function AdminOverview() {
    const { user } = useAuth();
    const [stats, setStats] = useState({
        total_users: 0,
        active_bookings: 0,
        todays_bookings: 0,
        total_computers: 0
    });
    const [recentActivity, setRecentActivity] = useState([]);

    useEffect(() => {
        const fetchStats = async () => {
            try {
                const response = await apiFetch("/dashboard/stats");
                if (response.ok) {
                    const data = await response.json();
                    setStats(data);
                }
            } catch (error) {
                console.error("Error fetching stats:", error);
            }
        };

        const fetchActivity = async () => {
            try {
                const response = await apiFetch("/admin/bookings");
                if (response.ok) {
                    const data = await response.json();
                    // Sort desc by ID or Date (assuming ID correlates with time if timestamps absent, but we have time)
                    const sorted = data.sort((a, b) => b.id - a.id).slice(0, 5);
                    setRecentActivity(sorted);
                }
            } catch (error) {
                console.error("Error fetching activity:", error);
            }
        };

        if (user) {
            fetchStats();
            fetchActivity();
        }
    }, [user]);

    return (
        <div className="h-full overflow-hidden flex flex-col">
            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">Dashboard Overview</h1>
                <div className="text-sm text-slate-500">Welcome back, {user?.full_name || 'Admin'}</div>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8 space-y-8">
                {/* Quick Stats */}
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                    <div className="glass-card p-6 rounded-2xl shadow-sm flex items-center justify-between">
                        <div>
                            <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Total Users</p>
                            <h3 className="text-2xl font-black text-slate-800">{stats.total_users}</h3>
                        </div>
                        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-purple-50 text-purple-500">
                            <span className="material-symbols-outlined text-2xl">group</span>
                        </div>
                    </div>

                    <div className="glass-card p-6 rounded-2xl shadow-sm flex items-center justify-between">
                        <div>
                            <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Active Bookings</p>
                            <h3 className="text-2xl font-black text-slate-800">{stats.active_bookings}</h3>
                        </div>
                        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-blue-50 text-blue-500">
                            <span className="material-symbols-outlined text-2xl">desktop_windows</span>
                        </div>
                    </div>

                    <div className="glass-card p-6 rounded-2xl shadow-sm flex items-center justify-between">
                        <div>
                            <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Total Computers</p>
                            <h3 className="text-2xl font-black text-slate-800">{stats.total_computers}</h3>
                        </div>
                        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-emerald-50 text-emerald-500">
                            <span className="material-symbols-outlined text-2xl">computer</span>
                        </div>
                    </div>

                    <div className="glass-card p-6 rounded-2xl shadow-sm flex items-center justify-between">
                        <div>
                            <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-1">Today's Bookings</p>
                            <h3 className="text-2xl font-black text-slate-800">{stats.todays_bookings}</h3>
                        </div>
                        <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-slate-50 text-slate-500">
                            <span className="material-symbols-outlined text-2xl">calendar_today</span>
                        </div>
                    </div>
                </div>

                {/* Recent Activity Section */}
                <div className="glass-card rounded-2xl p-6 shadow-sm">
                    <h3 className="font-bold text-slate-800 mb-4">Recent System Activity</h3>
                    <div className="space-y-4">
                        {recentActivity.length === 0 ? (
                            <p className="text-slate-400 text-sm">No recent activity recorded.</p>
                        ) : (
                            recentActivity.map((item, i) => (
                                <div key={i} className="flex items-center gap-4 p-3 hover:bg-slate-50 rounded-xl transition-colors border border-transparent hover:border-slate-100">
                                    <div className="w-10 h-10 rounded-full bg-purple-50 flex items-center justify-center text-[#7c3aed]">
                                        <span className="material-symbols-outlined text-lg">calendar_today</span>
                                    </div>
                                    <div className="flex-1">
                                        <p className="text-sm font-bold text-slate-700">Booking: {item.computer_name || `COM-${item.computer_id}`}</p>
                                        <p className="text-xs text-slate-500">by {item.user_name || item.user_username || `User #${item.user_id}`}</p>
                                    </div>
                                    <span className="text-xs font-bold text-slate-400">{new Date(item.start_time).toLocaleTimeString('th-TH', { hour: '2-digit', minute: '2-digit' })}</span>
                                </div>
                            ))
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}

export default AdminOverview;
