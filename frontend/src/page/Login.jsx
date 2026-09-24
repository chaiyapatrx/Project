// src/page/Login.jsx
import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../App';
import { apiFetch } from '../api';

const styles = `
  .glass-panel {
    background: rgba(255, 255, 255, 0.7);
    backdrop-filter: blur(24px);
    -webkit-backdrop-filter: blur(24px);
    border: 1px solid rgba(255, 255, 255, 0.5);
  }
  .glass-input {
    background: rgba(255, 255, 255, 0.5);
    border: 1px solid rgba(124, 58, 237, 0.1);
  }
  .glass-input:focus {
    background: rgba(255, 255, 255, 0.9);
    border-color: rgba(124, 58, 237, 0.4);
    outline: none;
  }
  @keyframes fade-in {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .animate-fade-in {
    animation: fade-in 0.5s ease-out forwards;
  }
`;

function Login() {
    // ✅ 2. ดึงฟังก์ชัน login มาจาก Context
    const { login } = useAuth();
    const navigate = useNavigate();

    // --- State สำหรับ Logic Login ---
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [isLoading, setIsLoading] = useState(false);
    const [errorMsg, setErrorMsg] = useState("");

    // --- State สำหรับ UI (Announcements) ---
    const announcements = [
        { title: "Vision 2025: Innovation Hub", desc: "Building the future of integrated systems together. Our new campus opens this December." },
        { title: "New System Update v4.2", desc: "Security patches have been applied to all corporate accounts. Please check your mail." },
        { title: "Annual Tech Summit", desc: "Join us this Friday for the yearly innovation showcase at the main hall." }
    ];
    const [currentIndex, setCurrentIndex] = useState(0);

    // --- State สำหรับ UI (Switch Mode) ---
    const isInternal = true;

    useEffect(() => {
        const timer = setInterval(() => {
            setCurrentIndex((prevIndex) => (prevIndex + 1) % announcements.length);
        }, 3000);
        return () => clearInterval(timer);
    }, [announcements.length]);

    // --- ฟังก์ชัน Login หลัก ---
    const handleLogin = async (e) => {
        e.preventDefault();
        setErrorMsg("");
        setIsLoading(true);

        if (window.electronAPI) {
            try {
                const config = await window.electronAPI.getConfig();
                if (config && config.server_ip) {
                    window.electronConfig = config;
                }
            } catch (err) {
                console.error("Failed to get config from Electron:", err);
            }
        }

        try {
            const formData = new FormData();
            formData.append('username', username);
            formData.append('password', password);

            const response = await apiFetch("/api/auth/login", {
                method: "POST",
                body: formData,
            });

            if (!response.ok) {
                throw new Error("Invalid credentials");
            }

            const data = await response.json();
            const csrfToken = data.csrf_token;

            // 3. ดึงข้อมูล User จริงจาก Backend (/users/me) เพื่อเอา Role
            const userResponse = await apiFetch("/users/me");

            if (!userResponse.ok) {
                throw new Error("Failed to fetch user profile");
            }

            const userProfile = await userResponse.json();

            const userData = {
                username: userProfile.username,
                csrfToken: csrfToken, // for the double-submit CSRF header
                role: userProfile.role, // ใช้ Role จริงจาก DB
                full_name: userProfile.full_name,
                id: userProfile.id
            };

            // เรียกใช้ฟังก์ชันจาก App.jsx (มันจะพาเปลี่ยนหน้าให้อัตโนมัติ)
            login(userData);

        } catch (error) {
            console.error("Login Error:", error);
            setErrorMsg("Incorrect username or password.");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <>
            <style>{styles}</style>
            {/* Conditional Layout: Fullscreen for Kiosk (Electron), Card for Web */}
            <div className={`min-h-screen w-full flex items-center justify-center font-['Inter'] ${window.electronAPI ? 'bg-white' : 'bg-gradient-to-br from-white via-purple-50/30 to-slate-50 p-6'}`}>
                <div className={`w-full flex flex-col md:flex-row overflow-hidden ${window.electronAPI ? 'h-screen max-w-none' : 'layout-container max-w-[1100px] rounded-xl glass-panel shadow-[0_32px_64px_-16px_rgba(124,58,237,0.12)] min-h-[600px]'}`}>

                    {/* ฝั่งซ้าย: Banner & Announcements */}
                    <div className="w-full md:w-1/2 relative bg-slate-100 flex flex-col">
                        <div className="relative flex-1 group overflow-hidden">
                            <div
                                className="absolute inset-0 bg-cover bg-center transition-transform duration-1000 group-hover:scale-105"
                                style={{
                                    backgroundImage: `linear-gradient(rgba(124,58,237,0.1), rgba(46,16,101,0.6)), url("https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?q=80&w=1964&auto=format&fit=crop")`
                                }}
                            ></div>

                            <div className="absolute top-8 left-8">
                                <div className="flex h-9 shrink-0 items-center justify-center gap-x-2 rounded-full bg-white/20 backdrop-blur-xl border border-white/30 pl-3 pr-4 shadow-sm">
                                    <span className="material-symbols-outlined text-white text-[20px]">campaign</span>
                                    <p className="text-white text-xs font-bold uppercase tracking-widest leading-none">Announcements</p>
                                </div>
                            </div>

                            <div className="absolute bottom-0 left-0 p-12 w-full bg-gradient-to-t from-black/40 via-transparent to-transparent min-h-[220px] flex flex-col justify-end">
                                <div key={currentIndex} className="animate-fade-in flex flex-col gap-3">
                                    <h3 className="text-white text-3xl font-bold leading-tight">{announcements[currentIndex].title}</h3>
                                    <p className="text-white/90 text-sm font-light max-w-sm leading-relaxed text-left">{announcements[currentIndex].desc}</p>
                                    <div className="flex gap-2.5 mt-6">
                                        {announcements.map((_, index) => (
                                            <div key={index} className={`h-1.5 transition-all duration-500 rounded-full ${index === currentIndex ? 'w-14 bg-[#7c3aed]' : 'w-4 bg-white/30'}`}></div>
                                        ))}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* ฝั่งขวา: ฟอร์ม Login */}
                    <div className="w-full md:w-1/2 p-8 md:p-16 flex flex-col justify-center bg-white/40 text-left relative">

                        {/* ปุ่มย้อนกลับ */}
                        {/* ปุ่มย้อนกลับ */}
                        {/* ปุ่มย้อนกลับ (ซ่อนถ้าเป็น Kiosk) */}
                        {!window.electronAPI && (
                            <button
                                onClick={() => navigate('/')}
                                className="absolute top-6 right-6 flex items-center gap-2 px-4 py-2.5 rounded-full bg-white/40 hover:bg-white/80 backdrop-blur-md border border-slate-200/60 shadow-sm hover:shadow-md text-slate-500 hover:text-purple-600 transition-all group z-20"
                                title="Back to Home"
                            >
                                <span className="material-symbols-outlined text-lg group-hover:-translate-x-0.5 transition-transform">arrow_back</span>
                                <span className="text-xs font-bold uppercase tracking-wider">Back</span>
                            </button>
                        )}

                        <div className="mb-12 animate-fade-in" key={isInternal ? 'internal-head' : 'external-head'}>
                            <h1 className="text-slate-900 text-4xl font-black tracking-tight leading-tight">
                                {isInternal ? 'System Access' : 'External Portal'}
                            </h1>
                            <p className="text-slate-500 text-base mt-3 font-normal text-left">
                                {isInternal ? 'Enter your secure credentials to proceed.' : 'Access for authorized external partners only.'}
                            </p>
                        </div>

                        {/* แสดง Error Message ถ้ามี */}
                        {errorMsg && (
                            <div className="mb-6 p-3 rounded-lg bg-red-50 border border-red-100 text-red-600 text-sm font-medium flex items-center gap-2 animate-fade-in">
                                <span className="material-symbols-outlined text-lg">error</span>
                                {errorMsg}
                            </div>
                        )}

                        <form className="flex flex-col gap-8" onSubmit={handleLogin} key={isInternal ? 'internal-form' : 'external-form'}>

                            {/* Input 1: Username / Key */}
                            <div className="flex flex-col gap-2 animate-fade-in">
                                <label className="flex flex-col w-full text-left">
                                    <p className="text-slate-500 text-[11px] font-bold uppercase tracking-[0.2em] pb-2.5">
                                        {isInternal ? 'Identity' : 'Partner Key'}
                                    </p>
                                    <div className="relative group">
                                        <span className="material-symbols-outlined absolute left-4 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-[#7c3aed] transition-colors">
                                            {isInternal ? 'person' : 'vpn_key'}
                                        </span>
                                        <input
                                            className="glass-input flex w-full min-w-0 rounded-lg text-slate-900 h-14 placeholder:text-slate-300 pl-12 pr-4 text-base font-normal transition-all focus:ring-4 focus:ring-[#7c3aed]/10"
                                            placeholder={isInternal ? "Username" : "Enter your partner access key"}
                                            type="text"
                                            required
                                            value={username}
                                            onChange={(e) => setUsername(e.target.value)}
                                        />
                                    </div>
                                </label>
                            </div>

                            {/* Input 2: Password (เฉพาะ Internal) */}
                            {isInternal && (
                                <div className="flex flex-col gap-2 animate-fade-in">
                                    <label className="flex flex-col w-full text-left">
                                        <p className="text-slate-500 text-[11px] font-bold uppercase tracking-[0.2em] pb-2.5">Password</p>
                                        <div className="relative group">
                                            <span className="material-symbols-outlined absolute left-4 top-1/2 -translate-y-1/2 text-slate-400 group-focus-within:text-[#7c3aed] transition-colors">lock</span>
                                            <input
                                                className="glass-input flex w-full min-w-0 rounded-lg text-slate-900 h-14 placeholder:text-slate-300 pl-12 pr-4 text-base font-normal transition-all focus:ring-4 focus:ring-[#7c3aed]/10"
                                                placeholder="••••••••"
                                                type="password"
                                                required
                                                value={password}
                                                onChange={(e) => setPassword(e.target.value)}
                                            />
                                        </div>
                                    </label>
                                </div>
                            )}

                            {/* Buttons */}
                            <div className="flex flex-col sm:flex-row items-center gap-4 mt-4">
                                <button
                                    className="w-full h-14 flex items-center justify-center rounded-lg bg-[#7c3aed] text-white text-sm font-bold shadow-xl shadow-[#7c3aed]/25 hover:bg-[#6d28d9] active:scale-[0.98] transition-all disabled:opacity-70 disabled:cursor-not-allowed"
                                    type="submit"
                                    disabled={isLoading}
                                >
                                    {isLoading ? (
                                        <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin"></div>
                                    ) : (
                                        'Login'
                                    )}
                                </button>
                            </div>
                        </form>

                        <div className="mt-16 pt-8 border-t border-slate-100 flex items-center justify-end gap-3">
                            <div className={`px-2 py-1 rounded text-[10px] font-bold uppercase tracking-widest ${window.electronAPI ? 'bg-purple-100 text-purple-700' : 'bg-red-100 text-red-600'}`}>
                                {window.electronAPI ? 'ELECTRON MODE' : 'WEB MODE (NOT KIOSK)'}
                            </div>
                            <p className="text-slate-300 text-[10px] font-bold tracking-widest uppercase">v4.2.0-secure</p>
                        </div>
                    </div>

                </div>
            </div>
        </>
    );
}

export default Login;
