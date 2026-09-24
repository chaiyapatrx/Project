import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
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

function AdminComputerManagement() {
    const { user } = useAuth();
    const [toast, setToast] = useState(null);
    const [computers, setComputers] = useState([]);
    const [loading, setLoading] = useState(true);
    const [selectedComputer, setSelectedComputer] = useState(null);
    const [isUpdating, setIsUpdating] = useState(false);
    const [isAddModalOpen, setIsAddModalOpen] = useState(false);
    const [newComputerName, setNewComputerName] = useState('');
    const [provisionedAgent, setProvisionedAgent] = useState(null);

    const fetchComputers = useCallback(async () => {
        try {
            const response = await apiFetch("/admin/computers", { user });
            if (response.ok) {
                const data = await response.json();
                setComputers(data);
            } else {
                setToast({ type: 'error', message: "Failed to load stations" });
            }
        } catch (error) {
            console.error("Error fetching computers:", error);
        } finally {
            setLoading(false);
        }
    }, [user]);

    useEffect(() => {
        if (user) fetchComputers();
    }, [user, fetchComputers]);

    const handleAddComputer = async (e) => {
        e.preventDefault();
        try {
            const response = await apiFetch("/computers", {
                method: "POST",
                user,
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ name: newComputerName })
            });

            if (response.ok) {
                const data = await response.json();
                setIsAddModalOpen(false);
                setNewComputerName('');
                setProvisionedAgent({ name: data.name, secret: data.agent_secret });
                setToast({ type: 'success', message: "Station created. Save its one-time agent secret." });
                await fetchComputers();
            } else {
                const data = await response.json().catch(() => ({}));
                setToast({ type: 'error', message: data.error || "Failed to create computer" });
            }
        } catch (error) {
            console.error("Error creating computer:", error);
            setToast({ type: 'error', message: "Error creating computer" });
        }
    };

    const handleRotateAgentSecret = (computer) => {
        setAlert({
            title: "Rotate agent secret?",
            message: `The current agent for ${computer.name} will disconnect. Update its config.json with the new secret before reconnecting.`,
            type: 'warning',
            onConfirm: async () => {
                setAlert(null);
                try {
                    const response = await apiFetch(`/api/admin/computers/${computer.id}/agent-secret`, { method: "POST", user });
                    const data = await response.json();
                    if (!response.ok) throw new Error(data.error || "Failed to rotate agent secret");
                    setProvisionedAgent({ name: computer.name, secret: data.agent_secret });
                    setSelectedComputer(null);
                    await fetchComputers();
                } catch (error) {
                    setToast({ type: 'error', message: error.message || "Failed to rotate agent secret" });
                }
            }
        });
    };

    const [alert, setAlert] = useState(null); // { title, message, onConfirm, type: 'danger'|'warning' }

    // ... existing fetchComputers ...

    const handleBatchAction = (action) => {
        setAlert({
            title: `Confirm ${action} ALL?`,
            message: `Are you sure you want to ${action} ALL ${computers.length} computers? This cannot be undone.`,
            type: 'danger',
            onConfirm: async () => {
                setAlert(null);
                try {
                    const response = await apiFetch("/api/admin/computers/broadcast", {
                        method: "POST",
                        user,
                        headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ command: ({ logout: 'LOCK', restart: 'REBOOT', shutdown: 'SHUTDOWN' })[action.toLowerCase()] })
                    });

                    if (response.ok) {
                        const data = await response.json();
                        setToast({ type: 'success', message: `Batch ${action} sent to ${data.delivered_count || 0} active stations.` });
                    } else {
                        setToast({ type: 'error', message: `Failed to broadcast ${action}` });
                    }
                } catch {
                    setToast({ type: 'error', message: "Network error sending batch command" });
                }
            }
        });
    };

    const handleRemoteAction = (id, action, name) => {
        setAlert({
            title: `Confirm ${action}?`,
            message: `Are you sure you want to ${action} station ${name}?`,
            type: 'warning',
            onConfirm: async () => {
                try {
                    console.log(`Sending ${action} to ${id}`);
                    const cmdMap = {
                        'restart': 'REBOOT',
                        'shutdown': 'SHUTDOWN',
                        'logout': 'LOCK',
                        'lock': 'LOCK',
                        'unlock': 'UNLOCK',
                        'reboot': 'REBOOT'
                    };
                    const mappedCommand = cmdMap[action.toLowerCase()] || action.toUpperCase();

                    const response = await apiFetch(`/api/admin/computers/${id}/command`, {
                        method: "POST",
                        user,
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ command: mappedCommand })
                    });

                    if (response.ok) {
                        setToast({ type: 'success', message: `Command ${action} sent successfully.` });
                    } else {
                        const err = await response.json();
                        setToast({ type: 'error', message: `Failed: ${err.error || err.detail || 'Unknown error'}` });
                    }
                } catch (error) {
                    console.error("Error sending command:", error);
                    setToast({ type: 'error', message: "Network error sending command" });
                }
                setAlert(null);
            }
        });
    };

    const handleDeleteComputer = async (id) => {
        setAlert({
            title: "Deactivate Station?",
            message: "The station will be hidden while its usage history is kept. Cancel any active booking first.",
            type: 'danger',
            onConfirm: async () => {
                try {
                    const response = await apiFetch(`/computers/${id}`, {
                        method: "DELETE",
                        user
                    });
                    if (response.ok) {
                        setToast({ type: 'success', message: "Station deactivated" });
                        await fetchComputers();
                    } else {
                        const data = await response.json().catch(() => ({}));
                        setToast({ type: 'error', message: data.error || "Failed to deactivate station" });
                    }
                } catch (error) {
                    console.error("Error deleting:", error);
                }
                setAlert(null);
            }
        });
    };

    // Calculate Stats
    const total = computers.length;
    const offline = computers.filter(c => !c.is_online).length;
    const available = computers.filter(c => c.is_online && c.status === 'available').length;
    const inUse = computers.filter(c => c.is_online && (c.status === 'in_use' || c.status === 'in-use')).length;

    // ... exist code ...

    // ... exist code ...

    return (
        <div className="h-full overflow-hidden flex flex-col relative">
            <style>{`
                @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
            `}</style>
            {toast && <Toast message={toast.message} type={toast.type} onClose={() => setToast(null)} />}
            
            {/* Custom Alert Modal */}
            {alert && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm animate-zoom-in">
                    {/* ... Alert Content ... */}
                    <div className="bg-white rounded-2xl p-6 w-full max-w-sm shadow-2xl border ring-4 ring-red-50 text-center">
                        <div className={`w-14 h-14 rounded-full flex items-center justify-center mx-auto mb-4 ${alert.type === 'danger' ? 'bg-red-100 text-red-500' : 'bg-orange-100 text-orange-500'}`}>
                            <span className="material-symbols-outlined text-3xl">warning</span>
                        </div>
                        <h3 className="text-xl font-bold text-slate-800 mb-2">{alert.title}</h3>
                        <p className="text-slate-500 text-sm mb-6">{alert.message}</p>
                        <div className="grid grid-cols-2 gap-3">
                            <button onClick={() => setAlert(null)} className="py-2.5 rounded-xl font-bold text-slate-500 hover:bg-slate-100">Cancel</button>
                            <button onClick={alert.onConfirm} className={`py-2.5 rounded-xl font-bold text-white shadow-lg ${alert.type === 'danger' ? 'bg-red-500 hover:bg-red-600' : 'bg-orange-500 hover:bg-orange-600'}`}>Confirm</button>
                        </div>
                    </div>
                </div>
            )}

            {isAddModalOpen && (
                <div className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
                    <form onSubmit={handleAddComputer} className="bg-white rounded-2xl p-6 w-full max-w-sm shadow-2xl">
                        <h2 className="text-lg font-bold text-slate-800 mb-2">Add Station</h2>
                        <p className="text-sm text-slate-500 mb-4">Create a station first, then configure its agent with the one-time secret.</p>
                        <label className="block text-sm font-medium text-slate-600 mb-1" htmlFor="station-name">Station name</label>
                        <input id="station-name" autoFocus required maxLength={50} value={newComputerName} onChange={e => setNewComputerName(e.target.value)} className="w-full border rounded-lg px-3 py-2 mb-5" />
                        <div className="flex justify-end gap-2">
                            <button type="button" onClick={() => setIsAddModalOpen(false)} className="px-4 py-2 rounded-lg text-slate-600">Cancel</button>
                            <button type="submit" className="px-4 py-2 rounded-lg bg-violet-600 text-white font-bold">Create</button>
                        </div>
                    </form>
                </div>
            )}

            {provisionedAgent && (
                <div className="fixed inset-0 z-[70] flex items-center justify-center p-4 bg-slate-900/60 backdrop-blur-sm">
                    <div className="bg-white rounded-2xl p-6 w-full max-w-lg shadow-2xl">
                        <h2 className="text-lg font-bold text-slate-800">Save this agent secret</h2>
                        <p className="text-sm text-amber-700 bg-amber-50 border border-amber-100 rounded-lg p-3 my-3">Shown once. Put it in this station's <code>config.json</code>; rotating it disconnects the current agent.</p>
                        <label className="block text-xs font-bold text-slate-500 mb-1" htmlFor="agent-secret">{provisionedAgent.name}</label>
                        <input id="agent-secret" readOnly value={provisionedAgent.secret} onFocus={e => e.target.select()} className="w-full border rounded-lg px-3 py-2 font-mono text-sm" />
                        <div className="flex justify-end gap-2 mt-4">
                            <button onClick={async () => {
                                try {
                                    await navigator.clipboard.writeText(provisionedAgent.secret);
                                    setToast({ type: 'success', message: "Agent secret copied" });
                                } catch {
                                    setToast({ type: 'error', message: "Select the secret field and copy it manually" });
                                }
                            }} className="px-4 py-2 rounded-lg border font-bold">Copy</button>
                            <button onClick={() => setProvisionedAgent(null)} className="px-4 py-2 rounded-lg bg-violet-600 text-white font-bold">Done</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Manage Computer Modal */}
            {selectedComputer && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
                    <div className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm" onClick={() => setSelectedComputer(null)}></div>
                    <div className="bg-white rounded-2xl p-6 w-full max-w-md shadow-2xl animate-zoom-in relative glass-card border border-slate-100 ring-1 ring-slate-900/5 text-center z-10" onClick={e => e.stopPropagation()}>
                        <button onClick={() => setSelectedComputer(null)} className="absolute top-4 right-4 p-2 rounded-full hover:bg-slate-100 text-slate-400 hover:text-slate-600 transition-colors">
                            <span className="material-symbols-outlined">close</span>
                        </button>

                        <div className="mb-8">
                            <div className="w-16 h-16 bg-slate-100 rounded-2xl flex items-center justify-center mx-auto mb-4 text-slate-500 shadow-inner">
                                <span className="material-symbols-outlined text-4xl">desktop_windows</span>
                            </div>
                            <h3 className="text-2xl font-black text-slate-800 tracking-tight">{selectedComputer.name}</h3>
                            <div className="flex items-center justify-center gap-2 mt-2">
                                {selectedComputer.is_online ? (
                                    <span className="px-2.5 py-0.5 rounded-full bg-emerald-100 text-emerald-600 text-xs font-bold border border-emerald-200 flex items-center gap-1">
                                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse"></span> Online
                                    </span>
                                ) : (
                                    <span className="px-2.5 py-0.5 rounded-full bg-slate-100 text-slate-500 text-xs font-bold border border-slate-200">
                                        Offline
                                    </span>
                                )}
                                <span className="text-slate-300">|</span>
                                <span className="text-xs font-mono text-slate-400">{selectedComputer.hwid ? selectedComputer.hwid.substring(0, 8) : 'NO-HWID'}</span>
                            </div>
                        </div>

                        <div className="grid grid-cols-2 gap-4 mb-6">
                            {/* Toggle Maintenance */}
                            <button
                                disabled={isUpdating}
                                onClick={() => {
                                    setIsUpdating(true);
                                    const newStatus = selectedComputer.status === 'available' ? 'maintenance' : 'available';

                                    apiFetch(`/computers/${selectedComputer.id}`, {
                                        method: 'PUT',
                                        user,
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ status: newStatus })
                                    }).then(async (res) => {
                                        if (res.ok) {
                                            await fetchComputers();
                                            setSelectedComputer(null);
                                        }
                                    }).finally(() => setIsUpdating(false));
                                }}
                                className={`p-4 rounded-xl border-2 flex flex-col items-center gap-2 transition-all ${selectedComputer.status === 'available' ? 'border-orange-100 bg-orange-50 text-orange-600 hover:bg-orange-100' : 'border-emerald-100 bg-emerald-50 text-emerald-600 hover:bg-emerald-100'} ${isUpdating ? 'opacity-50 cursor-wait' : ''}`}
                            >
                                <span className="material-symbols-outlined text-3xl">{selectedComputer.status === 'available' ? 'build' : 'check_circle'}</span>
                                <span className="font-bold text-sm">{selectedComputer.status === 'available' ? 'Set Maintenance' : 'Set Available'}</span>
                            </button>

                            <button
                                disabled={isUpdating}
                                onClick={() => handleRotateAgentSecret(selectedComputer)}
                                className="p-4 rounded-xl border-2 border-violet-100 bg-violet-50 text-violet-700 hover:bg-violet-100 flex flex-col items-center gap-2 transition-all"
                            >
                                <span className="material-symbols-outlined text-3xl">key</span>
                                <span className="font-bold text-sm">{selectedComputer.agent_key_configured ? 'Rotate Agent Secret' : 'Create Agent Secret'}</span>
                            </button>

                            {/* Deactivate */}
                            <button
                                disabled={isUpdating}
                                onClick={() => {
                                    setSelectedComputer(null);
                                    handleDeleteComputer(selectedComputer.id);
                                }}
                                className="p-4 rounded-xl border-2 border-rose-100 bg-rose-50 text-rose-500 hover:bg-rose-100 hover:border-rose-200 flex flex-col items-center gap-2 transition-all"
                            >
                                <span className="material-symbols-outlined text-3xl">archive</span>
                                <span className="font-bold text-sm">Deactivate Station</span>
                            </button>
                        </div>

                        {selectedComputer.is_online && (
                            <div className="bg-slate-50 rounded-xl p-4 border border-slate-200/60 shadow-inner">
                                <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest mb-3 text-center">Remote Power Actions</p>
                                <div className="flex justify-between gap-3">
                                    <button onClick={() => { setSelectedComputer(null); handleRemoteAction(selectedComputer.id, 'Logout', selectedComputer.name); }} className="flex-1 py-2.5 bg-white border border-slate-200 rounded-lg text-slate-600 font-bold text-xs hover:border-slate-300 hover:shadow-sm hover:-translate-y-0.5 transition-all">Logout</button>
                                    <button onClick={() => { setSelectedComputer(null); handleRemoteAction(selectedComputer.id, 'Restart', selectedComputer.name); }} className="flex-1 py-2.5 bg-white border border-slate-200 rounded-lg text-orange-600 font-bold text-xs hover:border-orange-200 hover:bg-orange-50 hover:-translate-y-0.5 transition-all">Restart</button>
                                    <button onClick={() => { setSelectedComputer(null); handleRemoteAction(selectedComputer.id, 'Shutdown', selectedComputer.name); }} className="flex-1 py-2.5 bg-white border border-slate-200 rounded-lg text-red-600 font-bold text-xs hover:border-red-200 hover:bg-red-50 hover:-translate-y-0.5 transition-all">Shutdown</button>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            <header className="h-16 border-b border-slate-200 bg-white/80 backdrop-blur-md sticky top-0 z-30 px-8 flex items-center justify-between shrink-0">
                <h1 className="text-lg font-bold text-slate-800">Computer Management</h1>

                {/* Batch Actions & Add Toolbar */}
                <div className="flex gap-2">
                    <button
                        onClick={() => setIsAddModalOpen(true)}
                        className="bg-[#7c3aed] hover:bg-[#6d28d9] text-white px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-1.5 transition-all shadow-sm shadow-purple-200"
                    >
                        <span className="material-symbols-outlined text-sm">add</span> Add Station
                    </button>
                    <button onClick={() => handleBatchAction('Logout')} className="bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-1.5 transition-all">
                        <span className="material-symbols-outlined text-sm">logout</span> Logout All
                    </button>
                    <button onClick={() => handleBatchAction('Restart')} className="bg-white border border-slate-200 hover:bg-orange-50 text-orange-600 border-orange-100 px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-1.5 transition-all">
                        <span className="material-symbols-outlined text-sm">restart_alt</span> Restart All
                    </button>
                    <button onClick={() => handleBatchAction('Shutdown')} className="bg-red-50 border border-red-100 hover:bg-red-100 text-red-600 px-3 py-1.5 rounded-lg text-xs font-bold flex items-center gap-1.5 transition-all">
                        <span className="material-symbols-outlined text-sm">power_settings_new</span> Shutdown All
                    </button>
                </div>
            </header>

            <div className="flex-1 overflow-y-auto custom-scrollbar p-8">

                {/* Status Overview */}
                <div className="grid grid-cols-4 gap-4 mb-8">
                    {[
                        { label: 'Total Stations', val: total, color: 'text-slate-600', icon: 'computer' },
                        { label: 'Available', val: available, color: 'text-emerald-500', icon: 'check_circle' },
                        { label: 'In Use', val: inUse, color: 'text-blue-500', icon: 'person' },
                        { label: 'Offline', val: offline, color: 'text-slate-400', icon: 'power_off' },
                    ].map((s, i) => (
                        <div key={i} className="bg-white p-4 rounded-xl border border-slate-100 shadow-sm flex items-center justify-between">
                            <div>
                                <p className="text-xs text-slate-400 font-bold uppercase tracking-wider">{s.label}</p>
                                <p className={`text-2xl font-black ${s.color}`}>{s.val}</p>
                            </div>
                            <span className="material-symbols-outlined text-slate-300 text-3xl">{s.icon}</span>
                        </div>
                    ))}
                </div>

                {/* Stations Grid */}
                {loading ? (
                    <div className="flex justify-center p-12"><div className="w-8 h-8 rounded-full border-4 border-slate-200 border-t-purple-500 animate-spin"></div></div>
                ) : computers.length === 0 ? (
                    <div className="bg-white rounded-2xl border border-slate-200 p-12 text-center max-w-lg mx-auto my-8">
                        <div className="w-16 h-16 rounded-2xl bg-purple-50 text-[#7c3aed] flex items-center justify-center mx-auto mb-4">
                            <span className="material-symbols-outlined text-3xl">desktop_windows</span>
                        </div>
                        <h3 className="text-lg font-bold text-slate-800 mb-1">ยังไม่มีเครื่องในระบบ (No Stations Registered)</h3>
                        <p className="text-xs text-slate-500 mb-6">
                            เมื่อเครื่องลูกข่ายเปิดรันโปรแกรม Client Agent หรือเมื่อกด "Add Station" เครื่องจะเชื่อมต่อและแสดงขึ้นมาในหน้านี้โดยอัตโนมัติ
                        </p>
                        <button
                            onClick={() => setIsAddModalOpen(true)}
                            className="bg-[#7c3aed] hover:bg-[#6d28d9] text-white px-5 py-2.5 rounded-xl text-xs font-bold shadow-md shadow-purple-200 transition-all inline-flex items-center gap-2"
                        >
                            <span className="material-symbols-outlined text-sm">add</span>
                            เพิ่มเครื่องด้วยตนเอง (Add Station)
                        </button>
                    </div>
                ) : (
                    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
                        {computers.map((comp) => {
                            // Logic: Offline takes priority. If Online, check status.
                            let displayStatus = 'offline';
                            let statusColor = 'bg-slate-400';
                            let iconColor = 'text-slate-400 bg-slate-100';
                            let cardStyle = 'grayscale opacity-75';

                            if (comp.is_online) {
                                cardStyle = ''; // Active
                                if (comp.status === 'in_use' || comp.status === 'in-use' || comp.status === 'occupied') {
                                    displayStatus = 'in-use';
                                    statusColor = 'bg-blue-500';
                                    iconColor = 'text-blue-600 bg-blue-100';
                                } else if (comp.status === 'maintenance') {
                                    displayStatus = 'maintenance';
                                    statusColor = 'bg-orange-500';
                                    iconColor = 'text-orange-500 bg-orange-100';
                                } else {
                                    displayStatus = 'available';
                                    statusColor = 'bg-emerald-500';
                                    iconColor = 'text-emerald-500 bg-emerald-100'; // Fixed specific color request
                                }
                            }

                            return (
                                <div
                                    key={comp.id}
                                    onClick={() => setSelectedComputer(comp)}
                                    className={`group relative bg-white border border-slate-200 rounded-xl p-4 flex flex-col items-center gap-3 transition-all hover:shadow-md cursor-pointer hover:-translate-y-1 ${cardStyle} ${displayStatus === 'in-use' ? 'ring-1 ring-blue-100 bg-blue-50/10' : ''}`}
                                >

                                    <div className="absolute top-2 right-2">
                                        <div className={`w-2 h-2 rounded-full ${statusColor}`}></div>
                                    </div>

                                    <div className={`w-12 h-12 rounded-xl flex items-center justify-center ${iconColor}`}>
                                        <span className="material-symbols-outlined text-2xl">desktop_windows</span>
                                    </div>

                                    <div className="text-center">
                                        <p className="font-bold text-slate-700 text-sm whitespace-nowrap overflow-hidden text-ellipsis w-20" title={comp.name}>{comp.name}</p>
                                        <p className="text-[10px] uppercase font-bold text-slate-400 mt-0.5">{displayStatus}</p>
                                        {comp.hwid && <p className="text-[8px] text-slate-300 font-mono mt-1 truncate w-16">{comp.hwid.substring(0, 6)}...</p>}
                                    </div>

                                    {/* Hover Actions */}
                                    {/* Hover Actions removed: using Modal instead */}
                                </div>
                            )
                        })}
                    </div>
                )}
            </div>


        </div>
    );
}

export default AdminComputerManagement;
