import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
// ✅ Import useAuth เพื่อดึงข้อมูล User ที่ Login เข้ามา
import { useAuth } from '../App';
import { apiFetch } from '../api';

// --- CSS Styles ---
const styles = `
  @import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700;900&display=swap');
  @import url('https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&display=swap');

  body {
    font-family: 'Inter', sans-serif;
    background-color: #ffffff;
    background-image: radial-gradient(at 0% 0%, rgba(124, 58, 237, 0.05) 0px, transparent 50%), 
                      radial-gradient(at 100% 100%, rgba(124, 58, 237, 0.05) 0px, transparent 50%);
  }
  .text-primary { color: #7c3aed; }
  .bg-primary { background-color: #7c3aed; }
  .bg-primary-dark:hover { background-color: #6d28d9; }
  .border-primary { border-color: #7c3aed; }
  .border-l-primary { border-left-color: #7c3aed; }
  .bg-primary-5 { background-color: rgba(124, 58, 237, 0.05); }
  .border-primary-20 { border-color: rgba(124, 58, 237, 0.2); }
  .shadow-primary-10 { --tw-shadow-color: rgba(124, 58, 237, 0.1); }
  .glass-card {
    background: rgba(255, 255, 255, 0.8);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(124, 58, 237, 0.1);
  }
  .custom-scrollbar::-webkit-scrollbar { width: 6px; }
  .custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
  .custom-scrollbar::-webkit-scrollbar-thumb { background: #e2e8f0; border-radius: 10px; }
  .station-grid {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1rem;
  }
  @keyframes modalPop { 0% { opacity: 0; transform: scale(0.95) translateY(10px); } 100% { opacity: 1; transform: scale(1) translateY(0); } }
  .modal-content { animation: modalPop 0.2s cubic-bezier(0.16, 1, 0.3, 1) forwards; }
  @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
  .toast-enter { animation: slideIn 0.3s ease-out forwards; }
`;

// --- Components Helper ---
const Toast = ({ message, type, onClose }) => {
    useEffect(() => { const timer = setTimeout(onClose, 3000); return () => clearTimeout(timer); }, [onClose]);
    const bgClass = type === 'success' ? 'bg-green-50 border-green-200 text-green-800' : 'bg-red-50 border-red-200 text-red-800';
    const icon = type === 'success' ? 'check_circle' : 'error';
    return (
        <div className={`fixed bottom-6 right-6 z-[200] flex items-center gap-3 px-4 py-3 rounded-xl border shadow-lg toast-enter ${bgClass}`}>
            <span className="material-symbols-outlined">{icon}</span><span className="text-sm font-bold">{message}</span>
        </div>
    );
};

function StudentDashboard() {
    const navigate = useNavigate();

    // ✅ เรียกใช้ Context เพื่อเอาข้อมูล User และฟังก์ชัน Logout
    const { user, logout } = useAuth();

    const [selectedStation, setSelectedStation] = useState(null);
    const [isBooking, setIsBooking] = useState(false);
    const [bookingResult, setBookingResult] = useState(null); // { success: true, code: 'ABC-123' }
    const [toast, setToast] = useState(null);

    // Real Data State
    const [stations, setStations] = useState([]);
    const [loading, setLoading] = useState(true);

    const [myBookings, setMyBookings] = useState([]);
    const [showBookingsModal, setShowBookingsModal] = useState(false);
    const [confirmCancel, setConfirmCancel] = useState(null);

    const fetchMyBookings = useCallback(async () => {
        if (!user) return;
        try {
            const response = await apiFetch("/my-bookings");
            if (response.ok) {
                const data = await response.json();
                setMyBookings(data.filter(b => b.status === "active"));
            }
        } catch (error) {
            console.error(error);
        }
    }, [user]);

    const executeCancelBooking = async (bookingId) => {
        setConfirmCancel(null);
        try {
            const response = await apiFetch(`/bookings/${bookingId}`, {
                method: "DELETE"
            });
            if (response.ok) {
                setToast({ type: 'success', message: "Booking cancelled successfully" });
                fetchMyBookings();
                fetchStations();
            } else {
                setToast({ type: 'error', message: "Failed to cancel booking" });
            }
        } catch {
            setToast({ type: 'error', message: "Network error" });
        }
    };

    const fetchStations = useCallback(async () => {
        try {
            const response = await apiFetch("/computers");
            if (response.ok) {
                const data = await response.json();
                setStations(data);
            }
        } catch (error) {
            console.error("Error fetching stations:", error);
            setToast({ type: 'error', message: "Failed to load stations" });
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchStations();
        fetchMyBookings();
        const interval = setInterval(() => {
            fetchStations();
            fetchMyBookings();
        }, 10000);
        return () => clearInterval(interval);
    }, [user, fetchStations, fetchMyBookings]);

    const availableCount = stations.filter(s => s.is_online && s.status === 'available').length;

    // --- Logic: Booking ---
    const handleStationClick = (station) => {
        if (!user) {
            setSelectedStation({ ...station, mode: 'login_required' });
            return;
        }
        setSelectedStation({ ...station, mode: 'confirm' });
    };

    const confirmBooking = async () => {
        if (!selectedStation) return;
        setIsBooking(true);

        try {
            const response = await apiFetch("/bookings", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json"
                },
                body: JSON.stringify({
                    computer_id: selectedStation.id
                })
            });

            if (response.ok) {
                const result = await response.json();
                setBookingResult({ success: true, code: result.access_code, station: selectedStation.name });
                setSelectedStation(null);
                fetchStations(); // Refresh status
                fetchMyBookings(); // Refresh bookings
            } else {
                const err = await response.json();
                setToast({ type: 'error', message: err.error || "Booking failed" });
            }
        } catch {
            setToast({ type: 'error', message: "Network error during booking" });
        } finally {
            setIsBooking(false);
        }
    };

    // Helper Date/Time
    const getCurrentTime = () => new Date().toLocaleTimeString('th-TH', { hour: '2-digit', minute: '2-digit' });
    const getCurrentDate = () => new Date().toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });

    return (
        <div className="min-h-screen text-slate-900 overflow-hidden font-sans relative">
            <style>{styles}</style>

            {/* Toast */}
            {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}

            {/* Navbar */}
            <nav className="sticky top-0 z-50 w-full border-b border-purple-100 bg-white/80 backdrop-blur-md">
                <div className="max-w-[1600px] mx-auto px-6 h-16 flex items-center justify-between">
                    <div className="flex items-center gap-2 cursor-pointer" onClick={() => navigate('/')}>
                        <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
                            <span className="material-symbols-outlined text-white text-xl">computer</span>
                        </div>
                        <span className="font-bold text-xl tracking-tight text-slate-800">User<span className="text-primary">Space</span></span>
                    </div>

                    <div className="flex items-center gap-4">
                        
                        {/* ✅ My Bookings Menu Button */}
                        {user && (
                            <button onClick={() => setShowBookingsModal(true)} className="flex items-center gap-2 bg-purple-50 text-primary px-4 py-2 rounded-full text-sm font-bold border border-purple-100 hover:bg-purple-100 transition-colors shadow-sm">
                                <span className="material-symbols-outlined text-[18px]">receipt_long</span> 
                                My Bookings
                                {myBookings.length > 0 && (
                                    <span className="bg-rose-500 text-white text-[10px] w-5 h-5 flex items-center justify-center rounded-full ml-1">
                                        {myBookings.length}
                                    </span>
                                )}
                            </button>
                        )}

                        {/* User Profile / Logout */}
                        {user ? (
                            <div className="flex items-center gap-2">
                                <div className="flex items-center gap-3 pl-2 pr-4 py-1.5 rounded-full hover:bg-slate-50 transition-colors border border-transparent hover:border-purple-100">
                                    <div className="w-9 h-9 rounded-full bg-purple-100 border border-purple-200 flex items-center justify-center text-primary shrink-0">
                                        <span className="material-symbols-outlined text-xl">person</span>
                                    </div>
                                    <div className="flex flex-col items-start justify-center">
                                        {/* แสดงชื่อ User จริงจาก Context */}
                                        <p className="text-xs font-bold text-slate-800 leading-tight">
                                            {user.username || user.full_name || "User"}
                                        </p>
                                        <p className="text-[10px] text-slate-500 font-medium leading-tight mt-0.5 capitalize">{user.role || 'User'}</p>
                                    </div>
                                </div>
                                {/* ปุ่ม Logout เรียกฟังก์ชันจาก Context */}
                                <button onClick={logout} className="w-9 h-9 rounded-full flex items-center justify-center text-slate-400 hover:text-red-500 hover:bg-red-50 transition-colors" title="Logout">
                                    <span className="material-symbols-outlined text-xl">logout</span>
                                </button>
                            </div>
                        ) : (
                            <button onClick={() => navigate('/login')} className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-5 py-2 rounded-full text-sm font-bold shadow-md transition-all">
                                Login
                            </button>
                        )}
                    </div>
                </div>
            </nav>

            {/* Main Content */}
            <main className="max-w-[1600px] mx-auto px-6 py-6 flex gap-6 h-[calc(100vh-64px)]">
                <div className="flex-1 flex flex-col min-w-0">

                    <div className="flex items-center justify-between mb-6 shrink-0">
                        <div>
                            <h2 className="text-2xl font-bold text-slate-900">Computer Stations</h2>
                            <p className="text-slate-500 text-sm mt-0.5">Computer Lab • {stations.length} Total Stations</p>
                        </div>
                        <div className="flex items-center gap-2 px-4 py-2 rounded-full bg-purple-50 border border-purple-100 shadow-sm">
                            <div className="w-2.5 h-2.5 rounded-full bg-primary animate-pulse"></div>
                            <span className="text-sm font-bold text-primary">{availableCount} Available</span>
                        </div>
                    </div>

                    <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 pb-6">
                        {loading ? (
                            <div className="flex justify-center p-12" role="status">Loading stations…</div>
                        ) : stations.length === 0 ? (
                            <div className="text-center p-12 text-slate-500">No stations are registered yet.</div>
                        ) : (
                        <div className="station-grid">
                            {stations.map((station) => {
                                const isOnline = station.is_online;
                                const isAvailable = isOnline && station.status === 'available';
                                const isMaintenance = isOnline && station.status === 'maintenance';
                                const displayStatus = !isOnline ? 'Offline' : (
                                    isAvailable ? 'Available' : 
                                    isMaintenance ? 'Maintenance' : 'Occupied'
                                );

                                const statusColor = isAvailable ? 'bg-purple-50 text-primary' : 
                                                    isMaintenance ? 'bg-amber-50 text-amber-500' : 
                                                    isOnline ? 'bg-rose-50 text-rose-500' : 'bg-slate-50 text-slate-300';
                                const badgeColors = isAvailable ? 'border-primary-20 text-primary bg-primary-5' : 
                                                    isMaintenance ? 'border-amber-200 text-amber-600 bg-amber-50' :
                                                    isOnline ? 'border-rose-200 text-rose-600 bg-rose-50' : 'border-slate-200 text-slate-400 bg-slate-50';
                                const cardBorder = isAvailable ? 'hover:shadow-md border-l-4 border-l-primary' : 'opacity-90 border-l-4 border-l-slate-200';

                                return (
                                    <div key={station.id} className={`glass-card p-4 rounded-xl flex flex-col gap-3 transition-all ${cardBorder}`}>
                                        <div className="flex items-center justify-between">
                                            <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${statusColor}`}>
                                                <span className="material-symbols-outlined text-2xl">{!isOnline ? 'power_off' : isMaintenance ? 'build' : 'desktop_windows'}</span>
                                            </div>
                                            <span className={`px-2 py-0.5 rounded-full border text-[9px] font-bold uppercase tracking-wider ${badgeColors}`}>
                                                {displayStatus}
                                            </span>
                                        </div>
                                        <div><h3 className={`font-bold text-sm ${isAvailable ? 'text-slate-800' : 'text-slate-600'}`}>{station.name}</h3></div>

                                        {isAvailable ? (
                                            <button onClick={() => handleStationClick(station)} className="w-full py-2 bg-primary hover:bg-primary-dark text-white rounded-lg text-xs font-bold shadow-md shadow-primary-10 transition-all active:scale-[0.98]">
                                                Book Station
                                            </button>
                                        ) : (
                                            <div className="w-full py-2 bg-slate-100 text-slate-400 rounded-lg text-xs font-bold flex items-center justify-center gap-1.5 cursor-not-allowed">
                                                <span className="material-symbols-outlined text-xs">lock</span> {displayStatus}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                        )}
                    </div>
                </div>
            </main>

            {/* Modal */}
            {selectedStation && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4">
                    <div className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm" onClick={() => !isBooking && setSelectedStation(null)}></div>
                    <div className="glass-card bg-white rounded-2xl shadow-2xl w-full max-w-sm relative z-10 overflow-hidden modal-content border-0 ring-1 ring-slate-900/5">
                        {selectedStation.mode === 'login_required' ? (
                            <div className="p-8 text-center">
                                <div className="w-16 h-16 bg-red-50 text-red-500 rounded-full flex items-center justify-center mx-auto mb-4 border border-red-100"><span className="material-symbols-outlined text-3xl">lock</span></div>
                                <h3 className="text-xl font-bold text-slate-900 mb-2">Login Required</h3>
                                <p className="text-slate-500 text-sm mb-8">Please log in to book <span className="font-bold text-slate-700">{selectedStation.name}</span>.</p>
                                <div className="flex gap-3">
                                    <button onClick={() => setSelectedStation(null)} className="flex-1 py-3 rounded-xl text-sm font-bold text-slate-600 hover:bg-slate-100">Cancel</button>
                                    <button onClick={() => navigate('/login')} className="flex-1 py-3 rounded-xl text-sm font-bold bg-slate-900 text-white hover:bg-slate-800">Go to Login</button>
                                </div>
                            </div>
                        ) : (
                            <div className="flex flex-col">
                                <div className="px-6 py-5 border-b border-slate-100 flex items-center justify-between bg-white">
                                    <h3 className="text-lg font-bold text-slate-800">Confirm Booking</h3>
                                    <button onClick={() => !isBooking && setSelectedStation(null)} disabled={isBooking} className="text-slate-400 hover:text-slate-600"><span className="material-symbols-outlined">close</span></button>
                                </div>
                                <div className="p-6 bg-slate-50/50">
                                    <div className="flex flex-col items-center mb-6">
                                        <span className="text-2xl font-black text-slate-800">{selectedStation.name}</span>
                                        <span className="text-xs font-semibold text-green-600 bg-green-50 px-2 py-0.5 rounded-md mt-1 border border-green-100">Available Now</span>
                                    </div>
                                    <div className="bg-white rounded-xl border border-slate-200 p-4 space-y-3 shadow-sm mb-6">
                                        <div className="flex justify-between items-center text-sm border-b border-slate-100 pb-2"><span className="text-slate-500 font-medium">Date</span><span className="text-slate-800 font-bold">{getCurrentDate()}</span></div>
                                        <div className="flex justify-between items-center text-sm border-b border-slate-100 pb-2"><span className="text-slate-500 font-medium">Time</span><span className="text-slate-800 font-bold">{getCurrentTime()}</span></div>
                                        <div className="flex justify-between items-center text-sm">
                                            <span className="text-slate-500 font-medium">User</span>
                                            {/* แสดงชื่อ User */}
                                            <span className="text-slate-800 font-bold">{user?.username || user?.full_name || 'Guest'}</span>
                                        </div>
                                    </div>
                                    <div className="grid grid-cols-2 gap-3">
                                        <button onClick={() => setSelectedStation(null)} disabled={isBooking} className="py-3 rounded-xl text-sm font-bold text-slate-500 bg-white border border-slate-200 hover:bg-slate-50">Cancel</button>
                                        <button onClick={confirmBooking} disabled={isBooking} className={`py-3 rounded-xl text-sm font-bold text-white shadow-lg transition-all flex items-center justify-center gap-2 ${isBooking ? 'bg-primary/80 cursor-wait' : 'bg-primary hover:bg-primary-dark'}`}>
                                            {isBooking ? 'Processing...' : 'Confirm'}
                                        </button>
                                    </div>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {/* My Bookings Modal */}
            {showBookingsModal && (
                <div className="fixed inset-0 z-[110] flex items-center justify-center p-4">
                    <div className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm" onClick={() => setShowBookingsModal(false)}></div>
                    <div className="glass-card bg-white rounded-2xl shadow-2xl w-full max-w-lg relative z-10 overflow-hidden modal-content flex flex-col max-h-[85vh]">
                        <div className="px-6 py-5 border-b border-slate-100 flex items-center justify-between bg-white shrink-0">
                            <h3 className="text-lg font-bold text-slate-800 flex items-center gap-2">
                                <span className="material-symbols-outlined text-primary">receipt_long</span>
                                My Active Bookings
                            </h3>
                            <button onClick={() => setShowBookingsModal(false)} className="text-slate-400 hover:text-slate-600"><span className="material-symbols-outlined">close</span></button>
                        </div>
                        
                        <div className="p-6 bg-slate-50 overflow-y-auto custom-scrollbar flex-1">
                            {myBookings.length === 0 ? (
                                <div className="text-center py-10">
                                    <span className="material-symbols-outlined text-4xl text-slate-300 mb-2">inbox</span>
                                    <p className="text-sm font-bold text-slate-500">No active bookings found.</p>
                                </div>
                            ) : (
                                <div className="flex flex-col gap-4">
                                    {myBookings.map(b => {
                                        const stationName = b.computer_name || b.computer?.name || (b.computer_id ? `COM-${b.computer_id}` : 'Unassigned Station');
                                        return (
                                            <div key={b.id} className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm relative overflow-hidden">
                                                <div className="absolute top-0 left-0 w-1.5 h-full bg-emerald-400"></div>
                                                <div className="flex justify-between items-start mb-4 pl-2">
                                                    <div>
                                                        <p className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-0.5">Station</p>
                                                        <p className="font-bold text-slate-800 text-lg">{stationName}</p>
                                                    </div>
                                                    <span className="px-2 py-1 bg-emerald-50 text-emerald-600 text-[10px] font-bold uppercase rounded border border-emerald-100">
                                                        Active
                                                    </span>
                                                </div>
                                                
                                                <div className="bg-slate-50 rounded-lg p-3 grid grid-cols-2 gap-3 mb-4 pl-2 border border-slate-100">
                                                    <div>
                                                        <p className="text-xs text-slate-500 font-medium">Access Code</p>
                                                        <p className="font-mono font-bold text-emerald-600 text-sm tracking-widest">{b.access_code}</p>
                                                    </div>
                                                    <div>
                                                        <p className="text-xs text-slate-500 font-medium">Valid until</p>
                                                        <p className="font-bold text-slate-700 text-sm">{new Date(b.end_time).toLocaleTimeString()}</p>
                                                    </div>
                                                </div>
                                                
                                                <div className="flex justify-end">
                                                    <button
                                                        onClick={() => setConfirmCancel(b.id)}
                                                        className="py-2 px-3 bg-white border border-rose-200 text-rose-600 hover:bg-rose-50 hover:border-rose-300 rounded-lg text-xs font-bold transition-all shadow-sm"
                                                    >
                                                        Cancel
                                                    </button>
                                                </div>
                                            </div>
                                        );
                                    })}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}

            {/* Custom Confirm Cancel Modal */}
            {confirmCancel && (
                <div className="fixed inset-0 z-[120] flex items-center justify-center p-4 animate-fade-in">
                    <div className="absolute inset-0 bg-slate-900/60 backdrop-blur-sm" onClick={() => setConfirmCancel(null)}></div>
                    <div className="glass-card bg-white rounded-2xl shadow-2xl w-full max-w-sm relative z-10 overflow-hidden animate-zoom-in">
                        <div className="p-6 text-center">
                            <div className="w-16 h-16 bg-rose-50 text-rose-500 rounded-full flex items-center justify-center mx-auto mb-4 border border-rose-100">
                                <span className="material-symbols-outlined text-3xl">warning</span>
                            </div>
                            <h3 className="text-xl font-bold text-slate-900 mb-2">Cancel Booking?</h3>
                            <p className="text-slate-500 text-sm mb-6">Are you sure you want to cancel this booking? This action cannot be undone.</p>
                            <div className="flex gap-3">
                                <button onClick={() => setConfirmCancel(null)} className="flex-1 py-3 rounded-xl text-sm font-bold text-slate-600 bg-slate-100 hover:bg-slate-200 transition-colors">No, Keep it</button>
                                <button onClick={() => executeCancelBooking(confirmCancel)} className="flex-1 py-3 rounded-xl text-sm font-bold bg-rose-500 text-white hover:bg-rose-600 transition-colors shadow-lg shadow-rose-500/30">Yes, Cancel</button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Booking Success Modal */}
            {bookingResult && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm animate-zoom-in">
                    <div className="bg-white rounded-2xl p-8 w-full max-w-sm shadow-2xl text-center border ring-4 ring-purple-100">
                        <div className="w-16 h-16 bg-green-100 text-green-600 rounded-full flex items-center justify-center mx-auto mb-4 border border-green-200">
                            <span className="material-symbols-outlined text-3xl">check_circle</span>
                        </div>
                        <h3 className="text-2xl font-black text-slate-800 mb-2">Booking Confirmed!</h3>
                        <p className="text-slate-500 text-sm mb-6">Use this access code to login at <span className="font-bold text-slate-700">{bookingResult.station}</span></p>

                        <div className="bg-slate-50 border border-slate-200 rounded-xl p-4 mb-6">
                            <p className="text-xs uppercase font-bold text-slate-400 mb-1">Access Code</p>
                            <p className="text-3xl font-mono font-bold text-primary tracking-widest">{bookingResult.code}</p>
                        </div>

                        <button
                            onClick={() => setBookingResult(null)}
                            className="w-full py-3 bg-slate-900 hover:bg-slate-800 text-white rounded-xl font-bold transition-colors"
                        >
                            Close
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}

export default StudentDashboard;
