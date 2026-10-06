import { useEffect, useRef, useState } from 'react';
import { getBaseUrl } from './api';

/**
 * useMonitorWebSocket connects to /api/ws/monitor for real-time station state updates.
 * Available to admin, staff, and executive roles.
 *
 * @param {Object} options
 * @param {Object} options.user Current authenticated user
 * @param {Function} [options.onStatusChanged] Callback when single station status changes: (ComputerStatus) => void
 * @param {Function} [options.onSnapshot] Callback when initial snapshot is received: (ComputerStatus[]) => void
 */
export function useMonitorWebSocket({ user, onStatusChanged, onSnapshot } = {}) {
  const [isConnected, setIsConnected] = useState(false);
  // Keep callback refs fresh
  const onStatusChangedRef = useRef(onStatusChanged);
  const onSnapshotRef = useRef(onSnapshot);
  useEffect(() => {
    onStatusChangedRef.current = onStatusChanged;
    onSnapshotRef.current = onSnapshot;
  }, [onStatusChanged, onSnapshot]);

  useEffect(() => {
    let cancelled = false;
    let socket = null;
    let reconnectTimeout = null;
    let retryCount = 0;

    const allowedRoles = ['admin', 'staff', 'executive'];
    if (!user || !allowedRoles.includes(user.role)) {
      return;
    }

    const reconnect = () => {
      if (cancelled) return;
      const delay = Math.min(2000 * Math.pow(1.5, retryCount++), 15000);
      reconnectTimeout = setTimeout(connect, delay);
    };

    const connect = () => {
      if (cancelled) return;

      try {
        const baseUrl = getBaseUrl();
        const wsUrl = `${baseUrl.replace(/^http/, 'ws')}/api/ws/monitor`;

        const ws = new WebSocket(wsUrl);
        socket = ws;

        ws.onopen = () => {
          if (cancelled) {
            ws.close();
            return;
          }
          setIsConnected(true);
          retryCount = 0;
        };

        ws.onmessage = (event) => {
          if (cancelled) return;
          try {
            const data = JSON.parse(event.data);
            if (data.type === 'SNAPSHOT' && Array.isArray(data.payload) && onSnapshotRef.current) {
              onSnapshotRef.current(data.payload);
            } else if (data.type === 'COMPUTER_STATUS_CHANGED' && data.payload && typeof data.payload === 'object' && onStatusChangedRef.current) {
              onStatusChangedRef.current(data.payload);
            }
          } catch (err) {
            console.error('[WS Monitor] Failed to parse message:', err);
          }
        };

        ws.onerror = (err) => {
          console.warn('[WS Monitor] Socket error:', err);
        };

        ws.onclose = () => {
          if (cancelled) return;
          setIsConnected(false);
          socket = null;
          reconnect();
        };
      } catch (err) {
        console.warn('[WS Monitor] Connection initialization error:', err);
        reconnect();
      }
    };

    connect();

    return () => {
      cancelled = true;
      clearTimeout(reconnectTimeout);
      if (socket) socket.close();
      setIsConnected(false);
    };
  }, [user]);

  return { isConnected };
}
