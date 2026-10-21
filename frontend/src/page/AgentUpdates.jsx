import React, { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';
import { FileInput } from '../ui';
import { useDialog } from '../dialog';

export default function AgentUpdates() {
    const { user } = useAuth();
    const dialog = useDialog();
    const [releases, setReleases] = useState([]);
    const [activeId, setActiveId] = useState(0);
    const [stations, setStations] = useState([]);
    const [version, setVersion] = useState('');
    const [file, setFile] = useState(null);
    const [busy, setBusy] = useState(false);
    const [message, setMessage] = useState('');
    const [loadError, setLoadError] = useState(false);

    const refresh = useCallback(async () => {
        try {
            const [versionsResponse, stationsResponse] = await Promise.all([
                apiFetch('/api/admin/agent-releases', { user }),
                apiFetch('/admin/computers', { user }),
            ]);
            if (!versionsResponse.ok || !stationsResponse.ok) throw new Error('Cannot load Agent updates');
            const versions = await versionsResponse.json();
            setReleases(versions.releases);
            setActiveId(versions.active_id);
            setStations(await stationsResponse.json());
            setLoadError(false);
        } catch (error) {
            console.error(error);
            setLoadError(true);
        }
    }, [user]);

    useEffect(() => {
        refresh();
        const timer = setInterval(refresh, 15000);
        return () => clearInterval(timer);
    }, [refresh]);

    const upload = async (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        if (!file || !version) return;
        setBusy(true);
        setMessage('');
        try {
            const body = new FormData();
            body.append('version', version);
            body.append('file', file);
            const response = await apiFetch('/api/admin/agent-releases', { method: 'POST', user, body });
            const result = await response.json().catch(() => ({}));
            if (!response.ok) throw new Error(result.error || 'Upload failed');
            setMessage(`Uploaded ${version}. Select Roll out to update stations.`);
            setVersion('');
            setFile(null);
            form.reset();
            await refresh();
        } catch (error) {
            setMessage(error.message);
        } finally {
            setBusy(false);
        }
    };

    const changeRelease = async (id) => {
        const selected = releases.find(release => release.id === id);
        const confirmed = await dialog(id
            ? { title: `Roll out version ${selected?.version}?`, message: 'Every station updates to this version. Busy stations wait until their session ends.', confirmLabel: 'Roll out' }
            : { title: 'Pause updates?', message: 'Stations keep their current version until you roll out again.', confirmLabel: 'Pause' });
        if (!confirmed) return;
        setBusy(true);
        setMessage('');
        try {
            const response = await apiFetch(id ? `/api/admin/agent-releases/${id}/activate` : '/api/admin/agent-releases/active', {
                method: id ? 'POST' : 'DELETE', user,
            });
            const result = await response.json().catch(() => ({}));
            if (!response.ok) throw new Error(result.error || 'Could not change Agent rollout');
            setMessage(id ? `Agent ${selected.version} is rolling out. Idle online stations check within one minute.` : 'Agent rollout paused.');
            await refresh();
        } catch (error) {
            setMessage(error.message);
        } finally {
            setBusy(false);
        }
    };

    const active = releases.find(release => release.id === activeId);
    const updated = stations.filter(station => station.agent_version === active?.version).length;

    const percent = active && stations.length ? Math.round((updated / stations.length) * 100) : 0;
    const waiting = active ? stations.filter(station => station.agent_version !== active.version) : [];
    const formatSize = bytes => `${(bytes / 1048576).toFixed(1)} MB`;

    return (
        <section className="bg-white rounded-2xl border border-slate-100 shadow-sm p-6 mb-6">
            <div className="flex items-start justify-between gap-4 flex-wrap">
                <div className="flex items-start gap-3">
                    <span className="material-symbols-outlined w-10 h-10 rounded-xl bg-purple-50 text-[#7c3aed] flex items-center justify-center" aria-hidden="true">system_update_alt</span>
                    <div>
                        <h2 className="font-bold text-slate-800">Agent updates</h2>
                        <p className="text-sm text-slate-500 max-w-2xl">
                            {user?.role === 'admin' ? 'Upload a new LabAccessAgent.exe, then roll it out.' : 'Roll out a version an administrator uploaded.'} Busy stations wait until their session ends; offline stations update when they reconnect.
                        </p>
                    </div>
                </div>
                <div className="min-w-[220px]">
                    <div className="flex justify-between text-xs font-semibold text-slate-500 mb-1">
                        <span>{active ? `Rolling out ${active.version}` : 'Updates paused'}</span>
                        {active && <span>{updated}/{stations.length} stations</span>}
                    </div>
                    <div className="h-2 rounded-full bg-slate-100 overflow-hidden">
                        <div className="h-full rounded-full bg-[#7c3aed] transition-all" style={{ width: `${percent}%` }} />
                    </div>
                </div>
            </div>

            {loadError && <p role="alert" className="mt-4 text-sm text-rose-700 bg-rose-50 border border-rose-100 rounded-xl px-3 py-2">Cannot load Agent updates. Check the server connection.</p>}
            {message && <p role="status" className="mt-4 text-sm text-violet-800 bg-violet-50 border border-violet-100 rounded-xl px-3 py-2">{message}</p>}

            {user?.role === 'admin' && (
                <form onSubmit={upload} className="mt-5 grid gap-3 sm:grid-cols-[160px_1fr_auto] items-end">
                    <label className="block">
                        <span className="block text-xs font-bold text-slate-500 uppercase tracking-wider mb-1">Version</span>
                        <input required pattern="[0-9]+\.[0-9]+\.[0-9]+" placeholder="1.2.0" value={version} onChange={event => setVersion(event.target.value)}
                            className="w-full bg-white border border-slate-200 rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-300" />
                    </label>
                    <div>
                        <span className="block text-xs font-bold text-slate-500 uppercase tracking-wider mb-1">Agent file</span>
                        <FileInput accept=".exe" file={file} onChange={setFile} label="Choose LabAccessAgent.exe" required />
                    </div>
                    <button disabled={busy} className="flex items-center justify-center gap-2 px-4 py-2 rounded-xl bg-[#7c3aed] hover:bg-[#6d28d9] text-white text-sm font-bold shadow-sm disabled:opacity-50">
                        <span className="material-symbols-outlined text-lg" aria-hidden="true">upload</span>Upload
                    </button>
                </form>
            )}

            {releases.length > 0 && (
                <ul className="mt-5 divide-y divide-slate-100 border border-slate-100 rounded-xl overflow-hidden">
                    {releases.map(release => {
                        const isActive = release.id === activeId;
                        return (
                            <li key={release.id} className={`flex items-center justify-between gap-3 px-4 py-3 ${isActive ? 'bg-purple-50/50' : 'bg-white'}`}>
                                <div className="flex items-center gap-3 min-w-0">
                                    <span className="font-bold text-slate-800">v{release.version}</span>
                                    {isActive && <span className="px-2 py-0.5 rounded-full text-[11px] font-semibold bg-emerald-50 text-emerald-700 border border-emerald-200">Active</span>}
                                    <span className="text-xs text-slate-400 truncate">{formatSize(release.file_size)} · {new Date(release.created_at).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })}</span>
                                </div>
                                {isActive ? (
                                    <button disabled={busy} onClick={() => changeRelease(0)} className="px-3 py-1.5 rounded-lg text-sm font-semibold text-amber-700 hover:bg-amber-50 disabled:opacity-50">Pause</button>
                                ) : (
                                    <button disabled={busy} onClick={() => changeRelease(release.id)} className="px-3 py-1.5 rounded-lg text-sm font-semibold text-[#7c3aed] border border-purple-200 hover:bg-purple-50 disabled:opacity-50">Roll out</button>
                                )}
                            </li>
                        );
                    })}
                </ul>
            )}

            {waiting.length > 0 && (
                <details className="mt-4 text-sm text-slate-600">
                    <summary className="cursor-pointer font-semibold text-slate-700">{waiting.length === 1 ? '1 station is' : `${waiting.length} stations are`} waiting for the update</summary>
                    <ul className="mt-2 space-y-1">
                        {waiting.map(station => (
                            <li key={station.id} className="flex items-center gap-2">
                                <span className={`w-2 h-2 rounded-full ${station.is_online ? 'bg-emerald-500' : 'bg-slate-300'}`} aria-hidden="true" />
                                <span className="font-medium text-slate-800">{station.name}</span>
                                <span className="text-slate-400">{station.agent_version || 'version unknown'} · {station.is_online ? 'online' : 'offline'}</span>
                                {station.agent_update_error && <span className="text-rose-600">· {station.agent_update_error}</span>}
                            </li>
                        ))}
                    </ul>
                </details>
            )}
        </section>
    );
}
