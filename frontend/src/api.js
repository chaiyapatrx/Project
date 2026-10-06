// Central API helper.
//
// Auth is carried primarily by the HttpOnly `auth_token` cookie (set by the
// backend on login), so every request must send credentials. For state-changing
// requests we also echo the CSRF token (double-submit-cookie pattern).
//
// Browser requests rely on the HttpOnly auth cookie; legacy non-browser callers
// may still use the Authorization: Bearer header.

export function getBaseUrl() {
  const configuredUrl = import.meta.env?.VITE_API_BASE_URL;
  if (configuredUrl) return configuredUrl.replace(/\/$/, '');

  if (typeof window !== 'undefined') {
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
  const url = new URL(path, `${baseUrl}/`);
  if (!['http:', 'https:'].includes(url.protocol) || url.origin !== new URL(baseUrl).origin) {
    throw new Error('API requests must use the configured server origin');
  }

  let currentUser = user;
  if (!currentUser && typeof window !== 'undefined') {
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
  if (unsafe && !finalHeaders['X-CSRF-Token']) {
    const cookieToken = typeof document !== 'undefined'
      ? document.cookie.split('; ').find(cookie => cookie.startsWith('csrf_token='))?.slice('csrf_token='.length)
      : null;
    const csrfToken = cookieToken || currentUser?.csrfToken;
    if (csrfToken) finalHeaders['X-CSRF-Token'] = csrfToken;
  }

  const response = await fetch(url, {
    method,
    credentials: 'include',
    headers: finalHeaders,
    body,
  });
  if (response.status === 401 && typeof window !== 'undefined' && !['/api/auth/login', '/token'].includes(url.pathname)) {
    window.dispatchEvent(new Event('aucc:session-expired'));
  }
  return response;
}

// logoutRequest clears the server-side session cookies.
export async function logoutRequest(user) {
  try {
    const response = await apiFetch('/api/auth/logout', { method: 'POST', user });
    // An expired/revoked session is already signed out.
    if (!response.ok && response.status !== 401) throw new Error(`Logout failed: ${response.status}`);
    return true;
  } catch (e) {
    // Keep the caller informed so it does not report a successful sign-out.
    console.error('Logout request failed:', e);
    return false;
  }
}
