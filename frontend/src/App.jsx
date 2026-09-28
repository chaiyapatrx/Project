// src/App.jsx
import React, { useState, useEffect, useContext, createContext } from 'react';
import { HashRouter, Routes, Route, useNavigate, Navigate, useLocation } from 'react-router-dom';
import { logoutRequest, apiFetch } from './api';

// --- Import ไฟล์หน้าเว็บ ---
import Login from './page/Login';
import StudentDashboard from './page/StudentDashboard';
import AdminDashboard from './page/AdminDashboard';
import StaffDashboard from './page/StaffDashboard'; // ✅ 1. เพิ่ม Import Staff
import ExecDashboard from './page/ExecDashboard';

// ==========================================
// 1. Context จัดการระบบ Login
// ==========================================
const AuthContext = createContext(null);

export const AuthProvider = ({ children }) => {
  const [user, setUser] = useState(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    let active = true;
    const restoreSession = async () => {
      try {
        const storedUser = localStorage.getItem('user_data');
        if (!storedUser) return;
        const parsed = JSON.parse(storedUser);
        if (!parsed || typeof parsed !== 'object' || !parsed.role) throw new Error('Invalid stored session');
        if (parsed.token) {
          delete parsed.token;
          localStorage.setItem('user_data', JSON.stringify(parsed));
        }
        try {
          const response = await apiFetch('/users/me', { user: parsed });
          if (!response.ok) {
            localStorage.removeItem('user_data');
            return;
          }
          const profile = await response.json();
          const refreshedUser = { ...parsed, ...profile, csrfToken: parsed.csrfToken };
          if (active) setUser(refreshedUser);
          localStorage.setItem('user_data', JSON.stringify(refreshedUser));
        } catch {
          if (active) setUser(parsed);
        }
      } catch {
        try { localStorage.removeItem('user_data'); } catch { /* Storage may be unavailable. */ }
      } finally {
        if (active) setLoading(false);
      }
    };
    void restoreSession();
    return () => { active = false; };
  }, []);

  const login = (userData) => {
    setUser(userData);
    localStorage.setItem('user_data', JSON.stringify(userData));

    // ✅ CHECK FOR ELECTRON KIOSK MODE
    if (window.electronAPI && window.electronAPI.unlock) {
      // Unlock Station (Minimize App)
      console.log("Kiosk Login Success -> Unlocking Station");
      window.electronAPI.unlock();
      return;
    }

    // ✅ 2. ปรับ Logic Redirect ตาม Role (Web Mode)
    if (userData.role === 'admin') {
      navigate('/admin');
    } else if (userData.role === 'staff') { // เพิ่มเงื่อนไข Staff
      navigate('/staff');
    } else if (userData.role === 'executive') { // Executive Route
      navigate('/executive');
    } else {
      navigate('/user');
    }
  };

  const logout = async () => {
    // Clear the server-side HttpOnly session cookie first, then local state.
    await logoutRequest(user);
    setUser(null);
    localStorage.removeItem('user_data');
    navigate('/login');
  };

  return (
    <AuthContext.Provider value={{ user, login, logout }}>
      {!loading && children}
    </AuthContext.Provider>
  );
};

// eslint-disable-next-line react-refresh/only-export-components
export const useAuth = () => useContext(AuthContext);

// ==========================================
// 2. ProtectedRoute
// ==========================================
const ProtectedRoute = ({ children, allowedRoles }) => {
  const { user } = useAuth();
  const location = useLocation();

  if (!user) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // ถ้า Role ไม่ตรงกับที่อนุญาต ให้ดีดกลับไปหน้า Dashboard ของตัวเอง
  if (allowedRoles && !allowedRoles.includes(user.role)) {
    let redirectPath = '/user'; // Default
    if (user.role === 'admin') redirectPath = '/admin';
    else if (user.role === 'staff') redirectPath = '/staff'; // ✅ 3. รองรับ Staff
    else if (user.role === 'executive') redirectPath = '/executive';

    return <Navigate to={redirectPath} replace />;
  }

  return children;
};

const GuestRoute = ({ children }) => {
  const { user } = useAuth();
  if (user) {
    let redirectPath = '/user';
    if (user.role === 'admin') redirectPath = '/admin';
    else if (user.role === 'staff') redirectPath = '/staff';
    else if (user.role === 'executive') redirectPath = '/executive';
    return <Navigate to={redirectPath} replace />;
  }
  return children;
};

// ==========================================
// 3. หน้า Public (Guest)
// ==========================================
// --- Shared Styles (Copied from StudentDashboard) ---
const publicStyles = `
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
  .bg-primary-5 { background-color: rgba(124, 58, 237, 0.05); }
  .border-primary-20 { border-color: rgba(124, 58, 237, 0.2); }
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
`;

// ==========================================
// 3. หน้า Public (Guest)
// ==========================================
const PublicHome = () => {
  const navigate = useNavigate();
  const { user } = useAuth(); // ดึง UseAuth เข้ามาเช็ค
  const [stations, setStations] = useState([]);
  const [initialLoading, setInitialLoading] = useState(true);
  const [stationError, setStationError] = useState("");
  const [retryCount, setRetryCount] = useState(0);

  // Fetch Real Data
  useEffect(() => {
    const fetchStations = async () => {
      try {
        const response = await apiFetch("/computers");
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const data = await response.json();
        if (!Array.isArray(data)) throw new Error("Unexpected station response");
        setStations(data);
        setStationError("");
      } catch (error) {
        console.error("Failed to fetch public stations:", error);
        setStationError("Unable to load station status. Check the server connection and try again.");
      } finally {
        setInitialLoading(false);
      }
    };
    fetchStations();

    // Auto refresh every 10s for live status
    const interval = setInterval(fetchStations, 10000);
    return () => clearInterval(interval);
  }, [retryCount]);

  const availableCount = stations.filter(s => s.is_online && s.status === 'available').length;

  const handleBookingAttempt = () => {
    // ใช้ confirm แบบ browser native ไปก่อน หรือจะทำ Modal สวยๆ ก็ได้
    // แต่เพื่อให้เหมือน StudentDashboard จะเน้น UI หลัก
    if (window.confirm("🔒 Restricted Access\nYou must log in to book a station.\n\nProceed to Login?")) {
      navigate('/login');
    }
  };

  return (
    <div className="min-h-screen text-slate-900 overflow-hidden font-sans relative">
      <style>{publicStyles}</style>

      {/* Navbar */}
      <nav className="sticky top-0 z-50 w-full border-b border-purple-100 bg-white/80 backdrop-blur-md">
        <div className="max-w-[1600px] mx-auto px-6 h-16 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center">
              <span className="material-symbols-outlined text-white text-xl">computer</span>
            </div>
            <span className="font-bold text-xl tracking-tight text-slate-800">Station<span className="text-primary">Dash</span></span>
          </div>

          <div className="flex items-center gap-4">
            {user ? (
                <button onClick={() => {
                    if (user.role === 'admin') navigate('/admin');
                    else if (user.role === 'staff') navigate('/staff');
                    else if (user.role === 'executive') navigate('/executive');
                    else navigate('/user');
                }} className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-5 py-2 rounded-full text-sm font-bold shadow-md transition-all">
                    Go to Dashboard
                </button>
            ) : (
                <>
                <span className="text-xs font-bold text-slate-400 uppercase tracking-wider hidden sm:block">Guest Mode</span>
                <button onClick={() => navigate('/login')} className="flex items-center gap-2 bg-primary hover:bg-primary-dark text-white px-5 py-2 rounded-full text-sm font-bold shadow-md transition-all">
                  Login
                </button>
                </>
            )}
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <main className="max-w-[1600px] mx-auto px-6 py-6 flex gap-6 h-[calc(100vh-64px)]">
        <div className="flex-1 flex flex-col min-w-0">

          {/* Header Section */}
          <div className="flex items-center justify-between mb-6 shrink-0">
            <div>
              <h2 className="text-2xl font-bold text-slate-900">Computer Stations Overview</h2>
              <p className="text-slate-500 text-sm mt-0.5">Live status from Computer Lab</p>
            </div>
            <div className="flex items-center gap-2 px-4 py-2 rounded-full bg-purple-50 border border-purple-100 shadow-sm">
              <div className="w-2.5 h-2.5 rounded-full bg-primary animate-pulse"></div>
              <span className={`text-sm font-bold ${stationError ? 'text-rose-600' : 'text-primary'}`}>
                {stationError ? 'Status unavailable' : `${availableCount} Available`}
              </span>
            </div>
          </div>

          {/* Table/Grid Section */}
          <div className="flex-1 overflow-y-auto custom-scrollbar pr-2 pb-6">
            {initialLoading ? (
              <div className="flex justify-center items-center h-64">
                <div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-[#7c3aed] animate-spin"></div>
              </div>
            ) : stationError && stations.length === 0 ? (
              <div className="h-64 flex flex-col items-center justify-center gap-3 text-center" role="alert">
                <p className="text-slate-600">{stationError}</p>
                <button onClick={() => setRetryCount(count => count + 1)} className="px-4 py-2 rounded-lg bg-primary text-white text-sm font-bold">
                  Try again
                </button>
              </div>
            ) : (
              <>
              {stationError && (
                <div className="mb-4 flex items-center justify-between gap-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800" role="alert">
                  <span>Unable to refresh station status. The displayed information may be out of date.</span>
                  <button onClick={() => setRetryCount(count => count + 1)} className="shrink-0 font-bold underline">Try again</button>
                </div>
              )}
              {stations.length === 0 ? (
                <div className="h-64 flex items-center justify-center text-center text-slate-500">
                  No stations are currently registered.
                </div>
              ) : <div className="station-grid">
                {stations.map((station) => {
                  let isAvailable = station.is_online && station.status === 'available';
                  let isMaintenance = station.is_online && station.status === 'maintenance';
                  
                  let statusColor = isAvailable ? 'bg-purple-50 text-primary' : 
                                    isMaintenance ? 'bg-amber-50 text-amber-500' : 
                                    (station.is_online ? 'bg-rose-50 text-rose-500' : 'bg-slate-50 text-slate-300');
                  let cardBorder = isAvailable ? 'hover:shadow-md border-l-4 border-l-primary' : 'opacity-90 border-l-4 border-l-slate-200';

                  let statusTextDisplay = station.is_online ? (station.status === 'available' ? 'Available' : station.status === 'maintenance' ? 'Maintenance' : 'Occupied') : 'Offline';
                  let badgeColors = isAvailable ? 'border-primary-20 text-primary bg-primary-5' : 
                                    isMaintenance ? 'border-amber-200 text-amber-600 bg-amber-50' :
                                    (station.is_online ? 'border-rose-200 text-rose-600 bg-rose-50' : 'border-slate-200 text-slate-400 bg-slate-50');

                  return (
                    <div
                      key={station.id}
                      onClick={handleBookingAttempt}
                      className={`glass-card p-4 rounded-xl flex flex-col gap-3 transition-all cursor-pointer ${cardBorder}`}
                    >
                      <div className="flex items-center justify-between">
                        <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${statusColor}`}>
                          <span className="material-symbols-outlined text-2xl">{!station.is_online ? 'power_off' : isMaintenance ? 'build' : 'desktop_windows'}</span>
                        </div>
                        <span className={`px-2 py-0.5 rounded-full border text-[9px] font-bold uppercase tracking-wider ${badgeColors}`}>
                          {statusTextDisplay}
                        </span>
                      </div>
                      <div>
                        <h3 className={`font-bold text-sm ${isAvailable ? 'text-slate-800' : 'text-slate-600'}`}>{station.name}</h3>
                      </div>

                      {isAvailable ? (
                        <button className="w-full py-2 bg-primary hover:bg-primary-dark text-white rounded-lg text-xs font-bold shadow-md shadow-primary-10 transition-all">
                          Login to Book
                        </button>
                      ) : (
                        <div className="w-full py-2 bg-slate-100 text-slate-400 rounded-lg text-xs font-bold flex items-center justify-center gap-1.5">
                          <span className="material-symbols-outlined text-xs">lock</span> {statusTextDisplay}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>}
              </>
            )}
          </div>

        </div>
      </main>
    </div>
  );
};

// ==========================================
// 4. App หลัก
// ==========================================
function App() {
  return (
    <HashRouter>
      <AuthProvider>
        <Routes>
          {/* KIOSK MODE ENFORCEMENT */}
          {window.electronAPI ? (
            /* In Electron, FORCE Login Page ONLY. No other routes accessible. */
            <>
              <Route path="/" element={<Login />} />
              <Route path="*" element={<Navigate to="/" replace />} />
            </>
          ) : (
            /* Normal Web Routing */
            <>
              <Route path="/" element={<GuestRoute><PublicHome /></GuestRoute>} />
              <Route path="/login" element={<GuestRoute><Login /></GuestRoute>} />

              {/* User */}
              <Route path="/user" element={
                <ProtectedRoute allowedRoles={['user', 'student']}>
                  <StudentDashboard />
                </ProtectedRoute>
              } />

              {/* Staff ✅ 4. เพิ่ม Route สำหรับ Staff */}
              <Route path="/staff" element={
                <ProtectedRoute allowedRoles={['staff']}>
                  <StaffDashboard />
                </ProtectedRoute>
              } />

              {/* Admin */}
              <Route path="/admin" element={
                <ProtectedRoute allowedRoles={['admin']}>
                  <AdminDashboard />
                </ProtectedRoute>
              } />

              {/* Executive */}
              <Route path="/executive" element={
                <ProtectedRoute allowedRoles={['executive']}>
                  <ExecDashboard />
                </ProtectedRoute>
              } />

              <Route path="*" element={<Navigate to="/" replace />} />
            </>
          )}
        </Routes>
      </AuthProvider>
    </HashRouter>
  );
}

export default App;
