/**
 * Golinks service worker (Manifest V3)
 *
 * Responsibilities:
 *   1. Omnibox: "go/<shortname>" navigates to the configured server.
 *      Live suggestions are fetched from /-/api/links as the user types.
 *   2. declarativeNetRequest: intercepts bare http://go/<shortname>
 *      browser navigations and redirects them to the configured server.
 *      The dynamic rule is rebuilt whenever the server URL changes.
 */

const DEFAULTS = { serverUrl: 'http://localhost:8080', token: '' };
const REDIRECT_RULE_ID = 1;

// ── Settings cache ────────────────────────────────────────────────────────────
// Cached in memory so omnibox suggest() is called without an extra storage
// round-trip.  The cache is seeded on first use and kept in sync via onChanged.

let _settingsCache = null;

async function getSettings() {
  if (!_settingsCache) {
    _settingsCache = await chrome.storage.sync.get(DEFAULTS);
  }
  return _settingsCache;
}

chrome.storage.onChanged.addListener((changes) => {
  // Keep the in-memory cache fresh.
  if (_settingsCache) {
    for (const [key, { newValue }] of Object.entries(changes)) {
      _settingsCache[key] = newValue;
    }
  }
  // Rebuild the declarativeNetRequest redirect rule if the server URL changed.
  if (changes.serverUrl) updateRedirectRule();
});

function authHeaders(token) {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// ── Omnibox ───────────────────────────────────────────────────────────────────

chrome.omnibox.setDefaultSuggestion({
  description: 'Navigate to <match>go/</match><dim>&lt;shortname&gt;</dim> — type to search your links',
});

chrome.omnibox.onInputChanged.addListener(async (text, suggest) => {
  if (!text.trim()) return;
  // Use the in-memory cache so we don't await a storage round-trip before
  // calling suggest() — stale suggest callbacks are silently discarded by
  // Chromium when called after too much async time has elapsed.
  const { serverUrl, token } = await getSettings();
  try {
    const res = await fetch(
      `${serverUrl}/-/api/links?q=${encodeURIComponent(text)}&limit=6`,
      { headers: authHeaders(token) }
    );
    if (!res.ok) return;
    const { links = [] } = await res.json();
    if (!links.length) return;
    suggest(
      links.map((l) => ({
        content: l.shortname,
        description: `<match>${escapeXml(l.shortname)}</match> → <dim>${escapeXml(truncate(l.target_url, 60))}</dim>`,
      }))
    );
  } catch (err) {
    // Server unreachable — user can still type a shortname manually.
    console.debug('[golinks] omnibox fetch failed:', err);
  }
});

chrome.omnibox.onInputEntered.addListener(async (text, disposition) => {
  const { serverUrl } = await getSettings();
  const url = `${serverUrl}/${text.trim()}`;
  switch (disposition) {
    case 'currentTab':
      chrome.tabs.update({ url });
      break;
    case 'newForegroundTab':
      chrome.tabs.create({ url });
      break;
    case 'newBackgroundTab':
      chrome.tabs.create({ url, active: false });
      break;
  }
});

// ── declarativeNetRequest: redirect http://go/* → server ─────────────────────

/**
 * Rebuilds the single dynamic redirect rule that intercepts bare `go/`
 * navigations. Called on install, startup, and whenever serverUrl changes.
 *
 * Rule: http://go/<anything>  →  <serverUrl>/<anything>
 */
async function updateRedirectRule() {
  const { serverUrl } = await getSettings();
  const base = serverUrl.replace(/\/$/, '');

  await chrome.declarativeNetRequest.updateDynamicRules({
    removeRuleIds: [REDIRECT_RULE_ID],
    addRules: [
      {
        id: REDIRECT_RULE_ID,
        priority: 1,
        action: {
          type: 'redirect',
          redirect: { regexSubstitution: `${base}/\\1` },
        },
        condition: {
          // Matches http://go/<path> — the leading "go" hostname only.
          regexFilter: '^https?://go(?::\\d+)?/(.*)$',
          resourceTypes: ['main_frame'],
        },
      },
    ],
  });
}

chrome.runtime.onInstalled.addListener(updateRedirectRule);
chrome.runtime.onStartup.addListener(updateRedirectRule);

// ── Utilities ─────────────────────────────────────────────────────────────────

function escapeXml(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function truncate(str, n) {
  return str.length <= n ? str : str.slice(0, n - 1) + '…';
}
