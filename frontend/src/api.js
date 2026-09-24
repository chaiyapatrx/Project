// Central API helper.
//
// Auth is carried primarily by the HttpOnly `auth_token` cookie (set by the
// backend on login), so every request must send credentials. For state-changing
// requests we also echo the CSRF token (double-submit-cookie pattern).
//
// Browser requests rely on the HttpOnly auth cookie; legacy non-browser callers
// may still use the Authorization: Bearer header.

export function getBaseUrl() {
  const configuredUrl = import.meta.env.VITE_API_BASE_URL;
  if (configuredUrl) return configuredUrl.replace(/\/$/, '');

  if (typeof window !== 'undefined') {
    if (window.electronConfig && window.electronConfig.api_base_url) {
      return window.electronConfig.api_base_url.replace(/\/$/, '');
    }
    if (window.electronAPI && window.electronConfig && window.electronConfig.server_ip) {
      const scheme = window.location?.protocol === 'https:' ? 'https' : 'http';
      return `${scheme}://${window.electronConfig.server_ip}:8000`;
    }
    const hostname = window.location?.hostname;
    if (hostname && hostname !== 'localhost') {
      if (window.location?.protocol === 'https:') return window.location.origin;
      return `http://${hostname}:8000`;
    }
    return `${window.location?.protocol === 'https:' ? 'https' : 'http'}://localhost:8000`;
  }
  return 'http://localhost:8000';
}

// apiFetch sends cookies and the CSRF header. Non-browser callers may use a
// stored Bearer token; browser clients use only the HttpOnly session cookie.
export async function apiFetch(path, { method = 'GET', user = null, headers = {}, body } = {}) {
  const baseUrl = getBaseUrl();
  const url = path.startsWith('http') ? path : `${baseUrl}${path}`;

  let currentUser = user;
  if (!currentUser && typeof window !== 'undefined' && window.localStorage) {
    try {
      const stored = localStorage.getItem('user_data');
      if (stored) {
        currentUser = JSON.parse(stored);
      }
    } catch {
      // Ignore JSON parse error
    }
  }

  const finalHeaders = { ...headers };

  // Use Bearer auth only for non-browser callers.
  if (typeof window === 'undefined' && currentUser && currentUser.token && !finalHeaders['Authorization']) {
    finalHeaders['Authorization'] = `Bearer ${currentUser.token}`;
  }

  // Attach CSRF token for unsafe methods.
  const unsafe = !['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase());
  if (unsafe && currentUser && currentUser.csrfToken && !finalHeaders['X-CSRF-Token']) {
    finalHeaders['X-CSRF-Token'] = currentUser.csrfToken;
  }

  return fetch(url, {
    method,
    credentials: 'include',
    headers: finalHeaders,
    body,
  });
}

// logoutRequest clears the server-side session cookies.
export async function logoutRequest(user) {
  try {
    const response = await apiFetch('/api/auth/logout', { method: 'POST', user });
    if (!response.ok) throw new Error(`Logout failed: ${response.status}`);
    return true;
  } catch (e) {
    // Best-effort; local state is cleared regardless.
    console.error('Logout request failed:', e);
    return false;
  }
}
