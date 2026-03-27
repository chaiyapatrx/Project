import React, { useState, useEffect } from 'react';
import { useAuth } from '../App';
const Toast = ({ message, type, onClose }) => {
    useEffect(() => { const timer = setTimeout(onClose, 3000); return () => clearTimeout(timer); }, [onClose]);
    const bgClass = type === 'success' ? 'bg-emerald-50 border-emerald-200 text-emerald-800' : 'bg-rose-50 border-rose-200 text-rose-800';
    const icon = type === 'success' ? 'check_circle' : 'error';
    return (
        <div className={`fixed bottom-6 right-6 z-[200] flex items-center gap-3 px-4 py-3 rounded-xl border shadow-lg ${bgClass}`} style={{ animation: 'slideIn 0.3s ease-out forwards' }}>
            <span className="material-symbols-outlined">{icon}</span><span className="text-sm font-bold">{message}</span>
        </div>
    );
};

function AdminUserManagement() {
    const { user } = useAuth();
    const [toast, setToast] = useState(null);
    const [users, setUsers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        const fetchUsers = async () => {
            try {
                const response = await fetch("http://localhost:8000/users", {
                    method: "GET",
                    headers: {
                        "Authorization": `Bearer ${user?.token}`, // ส่ง Token ไปด้วย
                        "Content-Type": "application/json"
                    }
                });

                if (!response.ok) {
                    throw new Error("Failed to fetch users");
                }

                const data = await response.json();
                setUsers(data);
            } catch (err) {
                console.error("Error fetching users:", err);
                setError(err.message);
            } finally {
                setLoading(false);
            }
        };

        if (user && user.token) {
            fetchUsers();
        }
    }, [user]);

    const [editingUser, setEditingUser] = useState({ id: null, role: '', department: '' });
    const [roleDropdownOpen, setRoleDropdownOpen] = useState(false);

    const roles = [
        { value: 'admin', label: 'Admin' },
        { value: 'executive', label: 'Executive' },
        { value: 'staff', label: 'Staff' },
        { value: 'user', label: 'User' }
    ];

    const saveUser = async (userId) => {
        try {
            const response = await fetch(`http://localhost:8000/admin/users/${userId}/info`, {
                method: "PUT",
                headers: {
                    "Authorization": `Bearer ${user?.token}`,
                    "Content-Type": "application/json"
                },
                body: JSON.stringify({ role: editingUser.role, department: editingUser.department })
            });
            if (response.ok) {
                setUsers(users.map(u => u.id === userId ? { ...u, role: editingUser.role, department: editingUser.department } : u));
                setEditingUser({ id: null, role: '', department: '' });
                setRoleDropdownOpen(false);
                setToast({ type: 'success', message: "User updated successfully" });
            } else {
                setToast({ type: 'error', message: "Failed to update user" });
            }
        } catch (err) {
            setToast({ type: 'error', message: "Error updating user" });
        }
    };

    return (
        <div className="h-full overflow-hidden flex flex-col relative">
            <style>{`
                @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
            `}</style>
            {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}

            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">User Management</h1>
                <button className="bg-[#7c3aed] hover:bg-[#6d28d9] text-white px-4 py-2 rounded-lg text-sm font-bold flex items-center gap-2 transition-all shadow-md shadow-purple-200">
                    <span className="material-symbols-outlined text-lg">add</span>
                    <span>Add New User</span>
                </button>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8">
                <div className="glass-card rounded-2xl border border-slate-100 overflow-hidden shadow-sm">
                    {/* Toolbar */}
                    <div className="p-4 border-b border-slate-100 flex items-center justify-between gap-4 bg-white/50">
                        <div className="relative flex-1 max-w-md">
                            <span className="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 text-lg">search</span>
                            <input
                                type="text"
                                className="pl-10 pr-4 py-2 bg-white border border-slate-200 rounded-lg text-sm w-full focus:ring-2 focus:ring-purple-200 focus:border-purple-300 transition-all outline-none"
                                placeholder="Search users by name, username, or ID..."
                            />
                        </div>
                        <div className="flex items-center gap-2">
                            <button className="p-2 text-slate-500 hover:bg-slate-100 rounded-lg transition-colors" title="Filter">
                                <span className="material-symbols-outlined">filter_list</span>
                            </button>
                            <button className="p-2 text-slate-500 hover:bg-slate-100 rounded-lg transition-colors" title="Export">
                                <span className="material-symbols-outlined">download</span>
                            </button>
                        </div>
                    </div>

                    {/* Content Area */}
                    {loading ? (
                        <div className="p-12 flex flex-col items-center justify-center text-slate-400">
                            <div className="w-8 h-8 border-4 border-purple-200 border-t-[#7c3aed] rounded-full animate-spin mb-4"></div>
                            <p className="text-sm font-semibold">Loading users...</p>
                        </div>
                    ) : error ? (
                        <div className="p-12 flex flex-col items-center justify-center text-rose-500">
                            <span className="material-symbols-outlined text-4xl mb-2">error</span>
                            <p className="text-sm font-bold">Error: {error}</p>
                        </div>
                    ) : (
                        <>
                            {/* Table */}
                            <table className="w-full text-left border-collapse">
                                <thead>
                                    <tr className="bg-slate-50/50 border-b border-slate-100 text-xs font-bold text-slate-500 uppercase tracking-wider">
                                        <th className="p-4 pl-6">User</th>
                                        <th className="p-4">Role</th>
                                        <th className="p-4">Department</th>
                                        <th className="p-4">Status</th>
                                        <th className="p-4 pr-6 text-right">Actions</th>
                                    </tr>
                                </thead>
                                <tbody className="divide-y divide-slate-100 bg-white/50">
                                    {users.length === 0 ? (
                                        <tr>
                                            <td colSpan="5" className="p-8 text-center text-slate-500 text-sm">
                                                No users found.
                                            </td>
                                        </tr>
                                    ) : (
                                        users.map((userData, i) => (
                                            <tr key={userData.id || i} className="hover:bg-purple-50/50 transition-colors group">
                                                <td className="p-4 pl-6">
                                                    <div className="flex items-center gap-3">
                                                        <div className="w-10 h-10 rounded-full bg-slate-200 flex items-center justify-center text-slate-500 font-bold text-xs uppercase">
                                                            {(userData.full_name || userData.username || '?').substring(0, 2)}
                                                        </div>
                                                        <div>
                                                            <p className="font-bold text-slate-700 text-sm">{userData.full_name || userData.username}</p>
                                                            <p className="text-xs text-slate-400">@{userData.username}</p>
                                                        </div>
                                                    </div>
                                                </td>
                                                <td className="p-4 relative">
                                                    {editingUser.id === userData.id ? (
                                                        <div className="relative inline-block w-32">
                                                            <button 
                                                                onClick={(e) => { e.stopPropagation(); setRoleDropdownOpen(!roleDropdownOpen); }}
                                                                className="w-full flex items-center justify-between bg-white border border-purple-200 px-3 py-1.5 rounded-lg text-xs font-bold text-slate-700 focus:ring-2 focus:ring-purple-300 hover:border-purple-300 transition-colors shadow-sm"
                                                            >
                                                                <span className="capitalize">{editingUser.role}</span>
                                                                <span className="material-symbols-outlined text-[16px] text-slate-400">
                                                                    {roleDropdownOpen ? 'expand_less' : 'expand_more'}
                                                                </span>
                                                            </button>
                                                            
                                                            {roleDropdownOpen && (
                                                                <>
                                                                    <div className="fixed inset-0 z-40" onClick={() => setRoleDropdownOpen(false)}></div>
                                                                    <div className="absolute top-full left-0 mt-1 w-full bg-white border border-slate-100 rounded-xl shadow-xl z-50 py-1 flex flex-col animate-zoom-in origin-top">
                                                                        {roles.map(r => (
                                                                            <button 
                                                                                key={r.value}
                                                                                onClick={() => { setEditingUser({ ...editingUser, role: r.value }); setRoleDropdownOpen(false); }}
                                                                                className={`px-3 py-2 text-left text-xs font-bold hover:bg-purple-50 transition-colors flex items-center justify-between ${editingUser.role === r.value ? 'bg-purple-50/50 text-primary' : 'text-slate-600'}`}
                                                                            >
                                                                                <span className="capitalize">{r.label}</span>
                                                                                {editingUser.role === r.value && <span className="material-symbols-outlined text-[14px]">check</span>}
                                                                            </button>
                                                                        ))}
                                                                    </div>
                                                                </>
                                                            )}
                                                        </div>
                                                    ) : (
                                                        <span className={`px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider ${
                                                                userData.role === 'admin' ? 'bg-purple-100 text-purple-600' :
                                                                userData.role === 'executive' ? 'bg-amber-100 text-amber-600' :
                                                                userData.role === 'staff' ? 'bg-blue-100 text-blue-600' : 'bg-slate-100 text-slate-600'
                                                            }`}>
                                                            {userData.role}
                                                        </span>
                                                    )}
                                                </td>
                                                <td className="p-4 text-sm text-slate-600 font-medium">
                                                    {editingUser.id === userData.id ? (
                                                        <input 
                                                            type="text" 
                                                            value={editingUser.department} 
                                                            onChange={(e) => setEditingUser({ ...editingUser, department: e.target.value })}
                                                            className="bg-white border border-purple-200 px-3 py-1.5 text-xs font-medium text-slate-700 focus:ring-2 focus:ring-purple-300 rounded-lg w-32 shadow-sm transition-all outline-none"
                                                            placeholder="e.g. IT, Library"
                                                        />
                                                    ) : (
                                                        userData.department || '-'
                                                    )}
                                                </td>
                                                <td className="p-4">
                                                    <div className="flex items-center gap-2">
                                                        <div className="w-2 h-2 rounded-full bg-emerald-500"></div>
                                                        <span className="text-sm font-medium text-slate-600">Active</span>
                                                    </div>
                                                </td>
                                                <td className="p-4 pr-6 text-right relative z-0">
                                                    {editingUser.id === userData.id ? (
                                                        <>
                                                            <button onClick={() => saveUser(userData.id)} className="text-emerald-500 hover:bg-emerald-50 p-1 rounded transition-colors" title="Save">
                                                                <span className="material-symbols-outlined text-lg">check</span>
                                                            </button>
                                                            <button onClick={() => { setEditingUser({ id: null, role: '', department: '' }); setRoleDropdownOpen(false); }} className="text-slate-400 hover:bg-rose-50 hover:text-rose-500 p-1 rounded transition-colors ml-1" title="Cancel">
                                                                <span className="material-symbols-outlined text-lg">close</span>
                                                            </button>
                                                        </>
                                                    ) : (
                                                        <>
                                                            <button onClick={() => setEditingUser({ id: userData.id, role: userData.role, department: userData.department || '' })} className="text-slate-400 hover:text-purple-600 p-1 rounded transition-colors opacity-0 group-hover:opacity-100" title="Edit User">
                                                                <span className="material-symbols-outlined text-lg">edit</span>
                                                            </button>
                                                            <button className="text-slate-400 hover:text-rose-500 p-1 rounded transition-colors opacity-0 group-hover:opacity-100 ml-1">
                                                                <span className="material-symbols-outlined text-lg">delete</span>
                                                            </button>
                                                        </>
                                                    )}
                                                </td>
                                            </tr>
                                        ))
                                    )}
                                </tbody>
                            </table>

                            {/* Pagination (Mock UI for now) */}
                            <div className="p-4 border-t border-slate-100 bg-slate-50/50 text-xs text-slate-500 flex items-center justify-between font-medium">
                                <span>Showing {users.length} users</span>
                                <div className="flex gap-1">
                                    <button className="px-3 py-1 rounded bg-white border border-slate-200 hover:border-purple-300 hover:text-purple-600 transition-colors disabled:opacity-50">Prev</button>
                                    <button className="px-3 py-1 rounded bg-[#7c3aed] text-white shadow-sm shadow-purple-200">1</button>
                                    <button className="px-3 py-1 rounded bg-white border border-slate-200 hover:border-purple-300 hover:text-purple-600 transition-colors">Next</button>
                                </div>
                            </div>
                        </>
                    )}
                </div>
            </div>
        </div>
    );
}

export default AdminUserManagement;
