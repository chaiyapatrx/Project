import React, { useCallback, useEffect, useState } from 'react';
import { useAuth } from '../App';
import { apiFetch } from '../api';

export default function AgentUpdates() {
    const { user } = useAuth();
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
        const prompt = id ? `Roll out Agent ${selected?.version} to every station?` : 'Pause Agent updates for every station?';
        if (!window.confirm(prompt)) return;
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

    return (
        <section className="bg-white rounded-xl border border-slate-200 p-4 mb-6">
            <h2 className="font-bold text-slate-800">Agent updates</h2>
            <p className="text-sm text-slate-500 mb-3">Admin uploads AUCCAgent.exe once. Staff or Admin selects a version. Busy stations wait until their session ends; offline stations update after reconnecting.</p>
            {loadError && <p role="alert" className="text-sm text-rose-600 mb-3">Cannot load Agent updates. Check backend and migration 000007.</p>}
            {message && <p role="status" className="text-sm text-violet-700 mb-3">{message}</p>}
            {user?.role === 'admin' && (
                <form onSubmit={upload} className="flex flex-wrap items-end gap-2 mb-4">
                    <label className="text-xs font-semibold">Version (for example 1.0.1)<input required pattern="[0-9]+\.[0-9]+\.[0-9]+" value={version} onChange={event => setVersion(event.target.value)} className="block border rounded px-2 py-1 text-sm" /></label>
                    <label className="text-xs font-semibold">Agent EXE<input required type="file" accept=".exe" onChange={event => setFile(event.target.files[0])} className="block text-sm" /></label>
                    <button disabled={busy} className="px-3 py-2 rounded bg-violet-600 text-white text-sm font-bold disabled:opacity-50">Upload</button>
                </form>
            )}
            <p className="text-sm mb-2">Current rollout: <strong>{active?.version || 'paused'}</strong>{active && ` · ${updated}/${stations.length} stations on this version`}</p>
            <div className="flex flex-wrap gap-2 mb-3">
                {releases.map(release => <button key={release.id} disabled={busy || release.id === activeId} onClick={() => changeRelease(release.id)} className="border rounded px-3 py-1 text-sm disabled:opacity-50">{release.version}{release.id === activeId ? ' (active)' : ' · Roll out'}</button>)}
                {activeId > 0 && <button disabled={busy} onClick={() => changeRelease(0)} className="border border-amber-300 text-amber-700 rounded px-3 py-1 text-sm">Pause</button>}
            </div>
            {active && stations.some(station => station.agent_version !== active.version) && (
                <details className="text-sm text-slate-600"><summary className="cursor-pointer">Stations waiting for update</summary>
                    <ul className="mt-2 space-y-1">{stations.filter(station => station.agent_version !== active.version).map(station => <li key={station.id}>{station.name}: {station.agent_version || 'version unknown'} · {station.is_online ? 'online' : 'offline'}{station.agent_update_error && <span className="text-rose-600"> · {station.agent_update_error}</span>}</li>)}</ul>
                </details>
            )}
        </section>
    );
}
