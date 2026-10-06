import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { apiFetch, getBaseUrl, logoutRequest } from '../src/api.js';
import { csvCell, toCSV } from '../src/csv.js';
import { isNewPasswordValid, passwordByteLength } from '../src/password.js';

assert.equal(isNewPasswordValid('a'.repeat(14)), false);
assert.equal(isNewPasswordValid('a'.repeat(15)), true);
assert.equal(isNewPasswordValid('a'.repeat(72)), true);
assert.equal(isNewPasswordValid('a'.repeat(73)), false);
assert.equal(isNewPasswordValid('ก'.repeat(24)), true);
assert.equal(isNewPasswordValid('ก'.repeat(25)), false);
assert.equal(isNewPasswordValid('😀'.repeat(14)), false);
assert.equal(isNewPasswordValid('😀'.repeat(18)), true);
assert.equal(isNewPasswordValid('😀'.repeat(19)), false);
assert.equal(passwordByteLength('legacy123') <= 72, true, 'Existing shorter passwords must remain usable at login');

let expiredSessions = 0;
globalThis.window = {
  location: { hostname: 'lab.example', protocol: 'https:', origin: 'https://lab.example' },
  dispatchEvent: event => { if (event.type === 'aucc:session-expired') expiredSessions += 1; },
};
globalThis.localStorage = { getItem() { throw new Error('Storage denied'); } };
globalThis.document = { cookie: 'csrf_token=current-cookie-token' };
let request;
globalThis.fetch = async (url, options) => { request = { url, options }; return { ok: true }; };
assert.equal(getBaseUrl(), 'https://lab.example');
await apiFetch('/bookings', { method: 'POST', user: { csrfToken: 'stale-tab-token' } });
assert.equal(request.options.headers['X-CSRF-Token'], 'current-cookie-token');
assert.equal(request.options.credentials, 'include');
await apiFetch('/bookings', { method: 'DELETE' });
assert.equal(request.options.headers['X-CSRF-Token'], 'current-cookie-token');
await assert.rejects(apiFetch('https://attacker.example/collect', { method: 'POST' }), /configured server origin/);
await assert.rejects(apiFetch('//attacker.example/collect'), /configured server origin/);
globalThis.fetch = async () => ({ ok: false, status: 401 });
await apiFetch('/users/me');
assert.equal(expiredSessions, 1, 'Protected API 401 must clear the browser session');
await apiFetch('/api/auth/login', { method: 'POST' });
assert.equal(expiredSessions, 1, 'Incorrect password must not trigger session expiration');
assert.equal(await logoutRequest({}), true, 'A revoked session is already signed out');
assert.equal(csvCell('\t =2+2'), '"\'\t =2+2"');
assert.equal(csvCell('x"y'), '"x""y"');
assert.equal(toCSV(['name'], [['line\nbreak']]), '"name"\r\n"line\nbreak"\r\n');

// Run the hook's real effect handlers with small browser/React substitutes.
const effects = [];
let connected = false;
globalThis.auditHooks = {
  useEffect: effect => effects.push(effect),
  useRef: value => ({ current: value }),
  useState: () => [false, value => { connected = value; }],
};
globalThis.auditBaseUrl = () => 'https://lab.example';
const sockets = [];
let failConstructor = false;
globalThis.WebSocket = class {
  constructor(url) {
    if (failConstructor) throw new Error('Network unavailable');
    assert.equal(url, 'wss://lab.example/api/ws/monitor');
    sockets.push(this);
  }
  close() { this.closed = true; }
};
const timers = new Map();
globalThis.setTimeout = callback => { const id = Symbol(); timers.set(id, callback); return id; };
globalThis.clearTimeout = id => timers.delete(id);
const source = (await readFile(new URL('../src/useMonitorWebSocket.js', import.meta.url), 'utf8'))
  .replace("import { useEffect, useRef, useState } from 'react';", 'const { useEffect, useRef, useState } = globalThis.auditHooks;')
  .replace("import { getBaseUrl } from './api';", 'const getBaseUrl = globalThis.auditBaseUrl;');
const { useMonitorWebSocket } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
const messages = [];
useMonitorWebSocket({ user: { role: 'staff' }, onStatusChanged: value => messages.push(value), onSnapshot: value => messages.push(value) });
effects[0]();
const cleanupOld = effects[1]();
const oldSocket = sockets[0];
oldSocket.onopen();
assert.equal(connected, true);
oldSocket.onmessage({ data: JSON.stringify({ type: 'COMPUTER_STATUS_CHANGED', payload: null }) });
assert.equal(messages.length, 0);
cleanupOld();
assert.equal(connected, false);
assert.equal(oldSocket.closed, true);
const cleanupNew = effects[1]();
const newSocket = sockets[1];
newSocket.onopen();
oldSocket.onclose();
oldSocket.onmessage({ data: JSON.stringify({ type: 'SNAPSHOT', payload: [] }) });
assert.equal(connected, true, 'Old socket must not disconnect the new connection');
assert.equal(timers.size, 0, 'Old socket must not schedule reconnect after cleanup');
assert.equal(messages.length, 0, 'Old socket must not deliver stale snapshots');
newSocket.onmessage({ data: JSON.stringify({ type: 'SNAPSHOT', payload: [{ id: 1 }] }) });
assert.deepEqual(messages, [[{ id: 1 }]]);
newSocket.onclose();
assert.equal(timers.size, 1);
cleanupNew();
assert.equal(timers.size, 0, 'Cleanup must cancel pending reconnect');
failConstructor = true;
const previousWarn = console.warn;
console.warn = () => {};
const cleanupFailed = effects[1]();
console.warn = previousWarn;
assert.equal(timers.size, 1, 'Constructor failure must retry instead of disconnecting permanently');
cleanupFailed();
assert.equal(timers.size, 0);
console.log('Frontend regression checks passed');
