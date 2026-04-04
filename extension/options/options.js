/**
 * Golinks options page script.
 * Loads settings from chrome.storage.sync, saves them on submit,
 * and tests the connection to the configured server.
 */

const DEFAULTS = { serverUrl: 'http://localhost:8080', token: '' };

function showStatus(msg, type = 'success') {
  const el = document.getElementById('status');
  el.textContent = msg;
  el.className = type;
  el.hidden = false;
  if (type === 'success') {
    setTimeout(() => { el.hidden = true; }, 3000);
  }
}

document.addEventListener('DOMContentLoaded', async () => {
  // Load saved settings.
  const { serverUrl, token } = await chrome.storage.sync.get(DEFAULTS);
  document.getElementById('server-url').value = serverUrl;
  document.getElementById('token').value = token;

  // Save.
  document.getElementById('options-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = document.getElementById('save-btn');
    btn.disabled = true;

    const serverUrl = document.getElementById('server-url').value.trim().replace(/\/$/, '');
    const token = document.getElementById('token').value.trim();

    await chrome.storage.sync.set({ serverUrl, token });
    showStatus('Settings saved ✓');
    btn.disabled = false;
  });

  // Test connection.
  document.getElementById('test-btn').addEventListener('click', async () => {
    const btn = document.getElementById('test-btn');
    btn.disabled = true;
    btn.textContent = 'Testing…';

    const serverUrl = document.getElementById('server-url').value.trim().replace(/\/$/, '');
    const token = document.getElementById('token').value.trim();
    const headers = token ? { Authorization: `Bearer ${token}` } : {};

    try {
      const res = await fetch(`${serverUrl}/-/api/health`, { headers });
      if (res.ok) {
        const { status } = await res.json();
        showStatus(`Connected ✓  (server status: ${status})`);
      } else {
        showStatus(`Server returned HTTP ${res.status}`, 'error');
      }
    } catch (err) {
      showStatus(`Could not reach server: ${err.message}`, 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = 'Test connection';
    }
  });
});
