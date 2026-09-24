import React, { useState, useRef, useEffect } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { toCSV } from '../csv';
import ExecOverview from './ExecOverview';
import ExecReports from './ExecReports';

// --- CSS Styles (Glassmorphism & Custom Scrollbar) ---
const styles = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');
  @import url('https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap');

  body {
    font-family: 'Inter', sans-serif;
    background-color: #f8f9fc;
    background-image: radial-gradient(at 0% 0%, rgba(124, 58, 237, 0.03) 0px, transparent 50%), 
                      radial-gradient(at 100% 100%, rgba(124, 58, 237, 0.03) 0px, transparent 50%);
  }

  .glass-card {
    background: rgba(255, 255, 255, 0.9);
    backdrop-filter: blur(8px);
    border: 1px solid rgba(124, 58, 237, 0.08);
  }

  .custom-scrollbar::-webkit-scrollbar { width: 4px; }
  .custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
  .custom-scrollbar::-webkit-scrollbar-thumb { background: #e2e8f0; border-radius: 10px; }

  .nav-item-active {
    background-color: #f5f3ff;
    color: #7c3aed;
    border-right: 4px solid #7c3aed;
  }
`;

function ExecDashboard() {
    const { logout, user } = useAuth();
    const [activeTab, setActiveTabState] = useState(() => localStorage.getItem('execActiveTab') || 'overview');
    const [filterPeriod, setFilterPeriod] = useState('month');
    const [isDropdownOpen, setIsDropdownOpen] = useState(false);
    const dropdownRef = useRef(null);

    // Close dropdown when clicking outside
    useEffect(() => {
        function handleClickOutside(event) {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
                setIsDropdownOpen(false);
            }
        }
        document.addEventListener("mousedown", handleClickOutside);
        return () => document.removeEventListener("mousedown", handleClickOutside);
    }, []);

    const filterOptions = [
        { value: 'day', label: 'วันนี้ (Today)' },
        { value: 'week', label: 'สัปดาห์นี้ (This Week)' },
        { value: 'month', label: 'เดือนนี้ (This Month)' },
        { value: 'all', label: 'ทั้งหมด (All Time)' }
    ];

    const setActiveTab = (tab) => {
        setActiveTabState(tab);
        localStorage.setItem('execActiveTab', tab);
    };

    const renderContent = () => {
        switch (activeTab) {
            case 'overview': return <ExecOverview />;
            case 'reports': return <ExecReports period={filterPeriod} />;
            default: return <ExecOverview />;
        }
    };

    const exportReport = async () => {
        try {
            const res = await apiFetch(`/admin/usage-history?period=${encodeURIComponent(filterPeriod)}`);
            if (!res.ok) return;
            const data = await res.json();
            const rows = data.map(item => [item.id, item.user_name || item.user_id, item.department || '-', item.computer_name || item.computer_id, item.start_time, item.end_time || '-', item.duration_minutes || 0, item.termination_reason || 'normal']);
            const blob = new Blob([toCSV(["Session ID", "User", "Department", "Computer", "Start Time", "End Time", "Duration Minutes", "Status"], rows)], { type: 'text/csv;charset=utf-8;' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = `executive_report_${new Date().toISOString().slice(0, 10)}.csv`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        } catch (err) {
            console.error("Export report error:", err);
        }
    };

    const headerTitle = {
        'overview': 'ภาพรวมสถิติ (Executive Overview)',
        'reports': 'รายงานเชิงลึก (Detailed Reports)',
    };

    return (
        <>
            <style>{styles}</style>
            <div className="min-h-screen text-slate-900 flex font-sans overflow-hidden">

                {/* ================= SIDEBAR ================= */}
                <aside className="w-64 border-r border-slate-200 bg-white flex flex-col h-screen sticky top-0 shrink-0 z-40">
                    {/* Logo */}
                    <div className="p-6 flex items-center gap-3">
                        <div className="w-10 h-10 bg-gradient-to-br from-[#7c3aed] to-[#a78bfa] rounded-xl flex items-center justify-center shadow-lg shadow-purple-200">
                            <span className="material-symbols-outlined text-white">insights</span>
                        </div>
                        <span className="font-extrabold text-xl tracking-tight text-slate-800">
                            Exec<span className="text-[#7c3aed]">View</span>
                        </span>
                    </div>

                    {/* Navigation */}
                    <nav className="flex-1 mt-4 overflow-y-auto custom-scrollbar">
                        <div className="px-4 py-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">การจัดการ (Management)</div>

                        <button
                            onClick={() => setActiveTab('overview')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'overview' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">dashboard</span>
                            <span className="text-sm font-semibold">ภาพรวมสถิติ (Overview)</span>
                        </button>

                        <div className="px-4 py-2 mt-6 text-[10px] font-bold text-slate-400 uppercase tracking-widest">รายงาน (Reporting)</div>

                        <button
                            onClick={() => setActiveTab('reports')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'reports' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">summarize</span>
                            <span className="text-sm font-semibold">รายงานเชิงลึก (Reports)</span>
                        </button>
                    </nav>

                    {/* User Profile Footer */}
                    <div className="p-4 border-t border-slate-100">
                        <div className="bg-slate-50 rounded-xl p-3 flex items-center gap-3">
                            <div className="w-9 h-9 rounded-full bg-white border border-slate-100 flex items-center justify-center text-[#7c3aed] font-bold text-xs shadow-sm uppercase">
                                {user?.username?.substring(0, 2) || 'EX'}
                            </div>
                            <div className="flex-1 min-w-0">
                                <p className="text-xs font-bold text-slate-800 truncate">{user?.full_name || user?.username || 'Executive User'}</p>
                                <p className="text-[10px] text-slate-500 font-medium capitalize truncate">ผู้บริหาร (Executive)</p>
                            </div>
                            <button onClick={logout} className="text-slate-400 hover:text-slate-600 transition-colors">
                                <span className="material-symbols-outlined text-lg">logout</span>
                            </button>
                        </div>
                    </div>
                </aside>

                {/* ================= MAIN CONTENT ================= */}
                <main className="flex-1 min-w-0 bg-[#f8f9fc] relative flex flex-col h-screen">
                    {/* Header */}
                    <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                        <h1 className="text-lg font-bold text-slate-800">
                            {headerTitle[activeTab] || 'Executive Dashboard'}
                        </h1>
                        <div className="flex items-center gap-3">
                            <div className="relative" ref={dropdownRef}>
                                <button
                                    onClick={() => setIsDropdownOpen(!isDropdownOpen)}
                                    className="bg-white px-4 py-2 rounded-xl border border-slate-200 text-xs font-bold text-slate-600 outline-none hover:border-purple-300 hover:text-purple-600 transition-colors flex items-center justify-between min-w-[200px] shadow-sm"
                                >
                                    <span className="flex items-center gap-2">
                                        <span className="material-symbols-outlined text-sm text-slate-400">calendar_month</span>
                                        {filterOptions.find(option => option.value === filterPeriod)?.label}
                                    </span>
                                    <span className={`material-symbols-outlined text-sm transition-transform duration-200 ${isDropdownOpen ? 'rotate-180 text-[#7c3aed]' : 'text-slate-400'}`}>expand_more</span>
                                </button>

                                {isDropdownOpen && (
                                    <div className="absolute top-full right-0 mt-2 w-full bg-white border border-slate-100 rounded-xl shadow-xl shadow-slate-200/50 overflow-hidden z-50 animate-zoom-in">
                                        <div className="py-1">
                                            {filterOptions.map((option) => (
                                                <button
                                                    key={option.value}
                                                    onClick={() => {
                                                        setFilterPeriod(option.value);
                                                        setIsDropdownOpen(false);
                                                    }}
                                                    className={`w-full text-left px-4 py-2.5 text-xs font-bold transition-colors flex items-center justify-between ${
                                                        filterPeriod === option.value
                                                            ? 'bg-purple-50 text-[#7c3aed]' 
                                                            : 'text-slate-600 hover:bg-slate-50 hover:text-slate-800'
                                                    }`}
                                                >
                                                    {option.label}
                                                    {filterPeriod === option.value && (
                                                        <span className="material-symbols-outlined text-sm">check</span>
                                                    )}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                )}
                            </div>
                            <button
                                onClick={exportReport}
                                className="bg-[#7c3aed] text-white px-5 py-2 rounded-xl text-xs font-bold flex items-center gap-2 shadow-lg shadow-purple-200 hover:bg-[#6d28d9] transition-all"
                            >
                                <span className="material-symbols-outlined text-sm">download</span>
                                ส่งออกรายงาน (Export)
                            </button>
                        </div>
                    </header>

                    {/* Dynamic Content */}
                    <div className="flex-1 overflow-hidden">
                        {renderContent()}
                    </div>
                </main>

            </div>
        </>
    );
}

export default ExecDashboard;
