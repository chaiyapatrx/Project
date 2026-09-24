import React, { useEffect, useState } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

const emptyAnalytics = {
    total_sessions: 0,
    avg_duration_min: 0,
    unique_users: 0,
    active_computers: 0,
    total_computers: 0,
    student_users: 0,
    staff_users: 0,
    other_users: 0
};

function ExecOverview() {
    const { user } = useAuth();
    const [analytics, setAnalytics] = useState(emptyAnalytics);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(false);

    useEffect(() => {
        if (!user) return;
        let cancelled = false;

        const fetchAnalytics = async () => {
            try {
                const response = await apiFetch('/api/admin/analytics/summary');
                if (!response.ok) throw new Error('Analytics request failed');
                const data = await response.json();
                if (!cancelled) setAnalytics({ ...emptyAnalytics, ...data });
            } catch (err) {
                if (!cancelled) setError(true);
                console.error('Failed to load exec analytics:', err);
            } finally {
                if (!cancelled) setLoading(false);
            }
        };

        fetchAnalytics();
        return () => { cancelled = true; };
    }, [user]);

    const totalUsers = Number(analytics.student_users) + Number(analytics.staff_users) + Number(analytics.other_users);
    const roles = [
        { label: 'Students', value: Number(analytics.student_users), color: 'bg-violet-600' },
        { label: 'Staff', value: Number(analytics.staff_users), color: 'bg-violet-400' },
        { label: 'Other roles', value: Number(analytics.other_users), color: 'bg-slate-400' }
    ];
    const cards = [
        { label: 'Usage sessions', value: Number(analytics.total_sessions).toLocaleString(), icon: 'data_usage', color: 'text-violet-600 bg-violet-50' },
        { label: 'Stations in use', value: `${analytics.active_computers} / ${analytics.total_computers}`, icon: 'computer', color: 'text-amber-600 bg-amber-50' },
        { label: 'Average session', value: `${Math.round(Number(analytics.avg_duration_min))} min`, icon: 'schedule', color: 'text-blue-600 bg-blue-50' },
        { label: 'Unique users', value: Number(analytics.unique_users).toLocaleString(), icon: 'groups', color: 'text-emerald-600 bg-emerald-50' }
    ];

    return (
        <div className="p-8 space-y-8 h-full overflow-y-auto custom-scrollbar">
            <section>
                <h2 className="text-xl font-bold text-slate-800">Usage overview</h2>
                <p className="text-sm text-slate-500 mt-1">Metrics are calculated from recorded sessions and active accounts.</p>
            </section>

            {error && <p role="alert" className="rounded-xl bg-rose-50 p-4 text-sm text-rose-700">Analytics could not be loaded.</p>}

            <section className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5" aria-busy={loading}>
                {cards.map(card => (
                    <article key={card.label} className="glass-card p-6 rounded-2xl shadow-sm">
                        <div className="flex items-center justify-between">
                            <p className="text-xs font-bold text-slate-500 uppercase tracking-wider">{card.label}</p>
                            <span className={`material-symbols-outlined rounded-xl p-2 ${card.color}`}>{card.icon}</span>
                        </div>
                        <p className="text-3xl font-black text-slate-800 mt-5">{loading ? '—' : card.value}</p>
                    </article>
                ))}
            </section>

            <section className="glass-card rounded-2xl p-6 shadow-sm">
                <h3 className="font-bold text-slate-800">Active accounts by role</h3>
                <p className="text-sm text-slate-500 mt-1">{totalUsers.toLocaleString()} active accounts</p>
                <div className="mt-6 space-y-5">
                    {roles.map(role => {
                        const percent = totalUsers ? Math.round((role.value / totalUsers) * 100) : 0;
                        return (
                            <div key={role.label}>
                                <div className="flex justify-between text-sm mb-2">
                                    <span className="font-medium text-slate-700">{role.label}</span>
                                    <span className="text-slate-500">{role.value.toLocaleString()} ({percent}%)</span>
                                </div>
                                <div className="h-2 rounded-full bg-slate-100 overflow-hidden">
                                    <div className={`h-full ${role.color}`} style={{ width: `${percent}%` }} />
                                </div>
                            </div>
                        );
                    })}
                </div>
            </section>
        </div>
    );
}

export default ExecOverview;
