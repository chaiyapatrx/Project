import React, { useState } from 'react';
import { useAuth } from '../App';
import StaffStationManagement from './StaffStationManagement';
import StaffSessionControl from './StaffSessionControl';
import StaffDailyReport from './StaffDailyReport';

// --- CSS Styles (Glassmorphism & Custom Scrollbar) ---
const styles = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;800&display=swap');
  @import url('https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap');

  body {
    font-family: 'Inter', sans-serif;
    background-color: #f8f9fc;
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
  
  /* Animation for Modal */
  @keyframes zoomIn {
    from { opacity: 0; transform: scale(0.95); }
    to { opacity: 1; transform: scale(1); }
  }
  .animate-zoom-in {
    animation: zoomIn 0.2s ease-out forwards;
  }
`;

function StaffDashboard() {
    const { logout, user } = useAuth();
    const [activeTab, setActiveTab] = useState('stations');

    // ฟังก์ชันสำหรับเลือก Render Component ตาม Tab ที่เลือก
    const renderContent = () => {
        switch (activeTab) {
            case 'stations':
                return <StaffStationManagement />;
            case 'sessions':
                return <StaffSessionControl />;
            case 'reports':
                return <StaffDailyReport />;
            default:
                return <StaffStationManagement />;
        }
    };

    return (
        <>
            <style>{styles}</style>
            <div className="min-h-screen text-slate-900 flex font-['Inter'] overflow-hidden bg-[#f8f9fc]">

                {/* ================= SIDEBAR ================= */}
                <div className="w-64 bg-white border-r border-slate-100 flex flex-col shrink-0 z-20 shadow-xl shadow-slate-200/50">
                    {/* Logo Area */}
                    <div className="h-16 flex items-center px-6 border-b border-slate-50">
                        <div className="flex items-center gap-2 text-slate-800">
                            <div className="w-8 h-8 bg-[#7c3aed] rounded-lg flex items-center justify-center text-white shadow-lg shadow-purple-200">
                                <span className="material-symbols-outlined text-lg">monitor_heart</span>
                            </div>
                            <span className="font-extrabold text-lg tracking-tight">Staff<span className="text-[#7c3aed]">Hub</span></span>
                        </div>
                    </div>

                    {/* Navigation Menu */}
                    <nav className="flex-1 overflow-y-auto custom-scrollbar py-6">
                        <div className="px-6 mb-2 text-[10px] font-extrabold text-slate-400 uppercase tracking-widest">Main Operations</div>

                        <button
                            onClick={() => setActiveTab('stations')}
                            className={`w-full flex items-center gap-3 px-6 py-3 text-[13px] font-bold transition-all duration-200 ${activeTab === 'stations' ? 'nav-item-active' : 'text-slate-500 hover:bg-slate-50 hover:text-slate-700'}`}
                        >
                            <span className={`material-symbols-outlined text-[20px] ${activeTab === 'stations' ? 'text-[#7c3aed]' : 'text-slate-400'}`}>grid_view</span>
                            Station Management
                        </button>

                        <button
                            onClick={() => setActiveTab('sessions')}
                            className={`w-full flex items-center gap-3 px-6 py-3 text-[13px] font-bold transition-all duration-200 ${activeTab === 'sessions' ? 'nav-item-active' : 'text-slate-500 hover:bg-slate-50 hover:text-slate-700'}`}
                        >
                            <span className={`material-symbols-outlined text-[20px] ${activeTab === 'sessions' ? 'text-[#7c3aed]' : 'text-slate-400'}`}>timer</span>
                            Session Control
                        </button>

                        <div className="px-6 mt-8 mb-2 text-[10px] font-extrabold text-slate-400 uppercase tracking-widest">Analytics</div>

                        <button
                            onClick={() => setActiveTab('reports')}
                            className={`w-full flex items-center gap-3 px-6 py-3 text-[13px] font-bold transition-all duration-200 ${activeTab === 'reports' ? 'nav-item-active' : 'text-slate-500 hover:bg-slate-50 hover:text-slate-700'}`}
                        >
                            <span className={`material-symbols-outlined text-[20px] ${activeTab === 'reports' ? 'text-[#7c3aed]' : 'text-slate-400'}`}>summarize</span>
                            Daily Report
                        </button>
                    </nav>

                    {/* User Profile Footer */}
                    <div className="p-4 border-t border-slate-100">
                        <div className="bg-slate-50 rounded-xl p-3 flex items-center gap-3">
                            <div className="w-9 h-9 rounded-full bg-white border border-slate-100 flex items-center justify-center text-[#7c3aed] font-bold text-xs shadow-sm uppercase">
                                {user?.username?.substring(0, 2) || 'ST'}
                            </div>
                            <div className="flex-1 min-w-0">
                                <p className="text-xs font-bold text-slate-800 truncate">{user?.full_name || user?.username || 'Staff Member'}</p>
                                <p className="text-[10px] text-slate-500 font-medium capitalize truncate">{user?.role || 'Staff'}</p>
                            </div>
                            <button onClick={logout} className="text-slate-400 hover:text-slate-600 transition-colors">
                                <span className="material-symbols-outlined text-lg">logout</span>
                            </button>
                        </div>
                    </div>
                </div>

                {/* ================= MAIN CONTENT ================= */}
                <main className="flex-1 min-w-0 bg-[#f8f9fc] relative flex flex-col h-screen">
                    {/* Header */}
                    <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                        <h1 className="text-lg font-bold text-slate-800 capitalize">
                            {activeTab.replace('-', ' ')}
                        </h1>
                        <div className="flex items-center gap-4">
                            <span className="bg-emerald-100 text-emerald-700 text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider flex items-center gap-1">
                                <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span>
                                Live System
                            </span>
                            <div className="h-4 w-px bg-slate-200"></div>
                            <button className="text-slate-400 hover:text-slate-600 transition-colors">
                                <span className="material-symbols-outlined">notifications</span>
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

export default StaffDashboard;