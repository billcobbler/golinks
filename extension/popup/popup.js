/**
 * Golinks popup script.
 *
 * On open:
 *   - Pre-fills the URL from the active tab.
 *   - Suggests a shortname derived from the page title.
 *   - Checks whether the current URL is already a golink and shows a badge.
 *
 * On submit:
 *   - POSTs to /-/api/links and shows success/error feedback.
 */

const DEFAULTS = { serverUrl: 'http://localhost:8080', token: '' };

async function getSettings() {
  return chrome.storage.sync.get(DEFAULTS);
}

function authHeaders(token, extra = {}) {
  const h = { 'Content-Type': 'application/json', ...extra };
  if (token) h['Authorization'] = `Bearer ${token}`;
  return h;
}

function showStatus(msg, type = 'success') {
  const el = document.getElementById('status');
  el.textContent = msg;
  el.className = type; // 'success' | 'error'
  el.hidden = false;
}

function slugify(title) {
  return title
    .toLowerCase()
    .replace(/[^a-z0-9/]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40);
}

// ── Init ──────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', async () => {
  const { serverUrl, token } = await getSettings();

  // Wire nav links.
  document.getElementById('open-dashboard').addEventListener('click', (e) => {
    e.preventDefault();
    chrome.tabs.create({ url: `${serverUrl}/-/links` });
    window.close();
  });
  document.getElementById('open-options').addEventListener('click', (e) => {
    e.preventDefault();
    chrome.runtime.openOptionsPage();
    window.close();
  });

  // Populate form from the active tab.
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab?.url && !tab.url.startsWith('chrome://') && !tab.url.startsWith('about:')) {
    document.getElementById('url').value = tab.url;
    if (tab.title) {
      document.getElementById('shortname').value = slugify(tab.title);
    }
    // Check if this URL is already a golink.
    checkExisting(serverUrl, token, tab.url);
  }

  // Handle form submission.
  document.getElementById('add-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = document.getElementById('submit-btn');
    btn.disabled = true;
    btn.textContent = 'Adding…';

    const { serverUrl, token } = await getSettings();
    try {
      const res = await fetch(`${serverUrl}/-/api/links`, {
        method: 'POST',
        headers: authHeaders(token),
        body: JSON.stringify({
          shortname:   document.getElementById('shortname').value.trim(),
          target_url:  document.getElementById('url').value.trim(),
          description: document.getElementById('description').value.trim(),
          is_pattern:  document.getElementById('is-pattern').checked,
        }),
      });

      if (res.status === 201) {
        const link = await res.json();
        showStatus(`✓ Created go/${link.shortname}`);
        document.getElementById('add-form').reset();
      } else if (res.status === 409) {
        showStatus('Shortname already exists', 'error');
      } else {
        const data = await res.json().catch(() => ({}));
        showStatus(data.error || `Server error ${res.status}`, 'error');
      }
    } catch {
      showStatus('Could not reach the golinks server.\nCheck settings.', 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = 'Add link';
    }
  });
});

// ── Existing badge ─────────────────────────────────────────────────────────────

async function checkExisting(serverUrl, token, tabUrl) {
  try {
    const q = encodeURIComponent(tabUrl);
    const res = await fetch(`${serverUrl}/-/api/links?q=${q}&limit=20`, {
      headers: authHeaders(token, { 'Content-Type': '' }),
    });
    if (!res.ok) return;
    const { links = [] } = await res.json();
    // Look for an exact target_url match.
    const match = links.find((l) => l.target_url === tabUrl);
    if (match) {
      document.getElementById('existing-name').textContent = `go/${match.shortname}`;
      document.getElementById('existing-badge').hidden = false;
    }
  } catch {
    // Silently ignore — badge is informational only.
  }
}
