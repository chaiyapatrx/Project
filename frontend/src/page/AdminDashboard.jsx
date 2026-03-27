import React, { useState } from 'react';
import { useAuth } from '../App';
import AdminOverview from './AdminOverview';
import AdminUserManagement from './AdminUserManagement';
import AdminComputerManagement from './AdminComputerManagement';
import AdminAnalytics from './AdminAnalytics';
import AdminUsageHistory from './AdminUsageHistory';
import AdminSettings from './AdminSettings';

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

function AdminDashboard() {
    const { logout } = useAuth();
    // Initialize from localStorage or default to 'overview'
    const [activeTab, setActiveTabState] = useState(() => localStorage.getItem('adminActiveTab') || 'overview');

    const setActiveTab = (tab) => {
        setActiveTabState(tab);
        localStorage.setItem('adminActiveTab', tab);
    };

    const renderContent = () => {
        switch (activeTab) {
            case 'overview': return <AdminOverview />;
            case 'users': return <AdminUserManagement />;
            case 'computers': return <AdminComputerManagement />;
            case 'analytics': return <AdminAnalytics />;
            case 'history': return <AdminUsageHistory />;
            case 'settings': return <AdminSettings />;
            default: return <AdminOverview />;
        }
    };

    return (
        <>
            <style>{styles}</style>
            <div className="min-h-screen text-slate-900 flex font-sans overflow-hidden">

                {/* --- Sidebar --- */}
                <aside className="w-64 border-r border-slate-200 bg-white flex flex-col h-screen sticky top-0 shrink-0 z-40">
                    <div className="p-6 flex items-center gap-3">
                        <div className="w-10 h-10 bg-[#7c3aed] rounded-xl flex items-center justify-center shadow-lg shadow-purple-200">
                            <span className="material-symbols-outlined text-white">admin_panel_settings</span>
                        </div>
                        <span className="font-extrabold text-xl tracking-tight text-slate-800">
                            Admin<span className="text-[#7c3aed]">Core</span>
                        </span>
                    </div>

                    <nav className="flex-1 mt-4 overflow-y-auto custom-scrollbar">
                        <div className="px-4 py-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">General</div>

                        <button
                            onClick={() => setActiveTab('overview')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'overview' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">dashboard</span>
                            <span className="text-sm font-semibold">Dashboard Overview</span>
                        </button>

                        <button
                            onClick={() => setActiveTab('users')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'users' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">group</span>
                            <span className="text-sm font-semibold">User Management</span>
                        </button>

                        <button
                            onClick={() => setActiveTab('computers')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'computers' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">desktop_windows</span>
                            <span className="text-sm font-semibold">Computer Management</span>
                        </button>

                        <div className="px-4 py-2 mt-6 text-[10px] font-bold text-slate-400 uppercase tracking-widest">Reporting</div>

                        <button
                            onClick={() => setActiveTab('analytics')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'analytics' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">analytics</span>
                            <span>Analytics / Reports</span>
                        </button>

                        <button
                            onClick={() => setActiveTab('history')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'history' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">history</span>
                            <span className="text-sm font-semibold">Usage History</span>
                        </button>

                        <div className="px-4 py-2 mt-6 text-[10px] font-bold text-slate-400 uppercase tracking-widest">Settings</div>

                        <button
                            onClick={() => setActiveTab('settings')}
                            className={`w-full flex items-center gap-3 px-6 py-3 transition-colors ${activeTab === 'settings' ? 'nav-item-active' : 'text-slate-600 hover:bg-purple-50 hover:text-[#7c3aed]'}`}
                        >
                            <span className="material-symbols-outlined">settings</span>
                            <span className="text-sm font-semibold">System Settings</span>
                        </button>
                    </nav>

                    <div className="p-4 border-t border-slate-100">
                        <div className="bg-slate-50 rounded-xl p-3 flex items-center gap-3">
                            <div className="w-10 h-10 rounded-full bg-purple-100 flex items-center justify-center text-[#7c3aed] font-bold">AD</div>
                            <div className="flex-1 min-w-0">
                                <p className="text-xs font-bold text-slate-800 truncate">Admin User</p>
                                <p className="text-[10px] text-slate-500 truncate">System Controller</p>
                            </div>
                            <button onClick={logout} className="text-slate-400 hover:text-slate-600 transition-colors">
                                <span className="material-symbols-outlined text-lg">logout</span>
                            </button>
                        </div>
                    </div>
                </aside>

                {/* --- Main Content --- */}
                <main className="flex-1 min-w-0 bg-[#f8f9fc] h-screen">
                    {renderContent()}
                </main>
            </div>
        </>
    );
}

export default AdminDashboard;
