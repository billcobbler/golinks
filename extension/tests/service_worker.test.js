'use strict';
/**
 * Service-worker tests — Node.js built-in test runner
 *
 * Run:  node --test extension/tests/service_worker.test.js
 *   or: make extension-test
 *
 * Strategy: load service_worker.js inside a vm context that has a mocked
 * `chrome` global. After loading, call captured listener functions directly
 * and assert on side-effects (declarativeNetRequest calls, tab navigations, …).
 */

const { describe, it, before } = require('node:test');
const assert = require('node:assert/strict');
const fs     = require('node:fs');
const path   = require('node:path');
const vm     = require('node:vm');

const { createChromeMock } = require('./helpers/chrome_mock.js');

const SW_PATH = path.join(__dirname, '..', 'background', 'service_worker.js');
const SW_SRC  = fs.readFileSync(SW_PATH, 'utf8');

/**
 * Load the service worker into a fresh vm context.
 * Returns { ctx, chrome } so tests can inspect state.
 *
 * @param {object} [chromeMock]  Pre-built chrome mock (optional).
 * @param {Function} [fetchMock] Replacement for the global `fetch`.
 */
function loadSW(chromeMock, fetchMock) {
  const chrome = chromeMock ?? createChromeMock();
  const ctx = vm.createContext({
    chrome,
    fetch:   fetchMock ?? (() => Promise.reject(new Error('fetch not mocked'))),
    console,
  });
  vm.runInContext(SW_SRC, ctx);
  return { ctx, chrome };
}

// ── declarativeNetRequest / updateRedirectRule ────────────────────────────────

describe('updateRedirectRule', () => {
  it('is called when the extension is installed', async () => {
    const { chrome } = loadSW();
    // Fire the onInstalled event
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();
    assert.equal(chrome._dnrCalls.length, 1, 'expected one updateDynamicRules call');
  });

  it('is called when the extension starts up', async () => {
    const { chrome } = loadSW();
    for (const fn of chrome._listeners.runtime.onStartup) await fn();
    assert.equal(chrome._dnrCalls.length, 1);
  });

  it('removes the old rule and adds a new one', async () => {
    const { chrome } = loadSW();
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();

    const call = chrome._dnrCalls[0];
    // Use element-by-element checks: arrays from the vm context have a different
    // prototype than arrays created in the test scope, so deepStrictEqual fails.
    assert.equal(call.removeRuleIds.length, 1);
    assert.equal(call.removeRuleIds[0], 1);
    assert.equal(call.addRules.length, 1);
    assert.equal(call.addRules[0].id, 1);
  });

  it('builds the correct regexSubstitution from the default serverUrl', async () => {
    const { chrome } = loadSW();
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();

    const rule = chrome._dnrCalls[0].addRules[0];
    assert.equal(
      rule.action.redirect.regexSubstitution,
      'http://localhost:8080/\\1',
    );
  });

  it('strips a trailing slash from the server URL', async () => {
    const chrome = createChromeMock({ storage: { serverUrl: 'http://myserver:9090/' } });
    loadSW(chrome);
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();

    const rule = chrome._dnrCalls[0].addRules[0];
    assert.equal(
      rule.action.redirect.regexSubstitution,
      'http://myserver:9090/\\1',
    );
  });

  it('uses a custom serverUrl from storage', async () => {
    const chrome = createChromeMock({ storage: { serverUrl: 'https://go.internal' } });
    loadSW(chrome);
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();

    const rule = chrome._dnrCalls[0].addRules[0];
    assert.equal(rule.action.redirect.regexSubstitution, 'https://go.internal/\\1');
  });

  it('uses the correct regex to match http://go/* navigations', async () => {
    const { chrome } = loadSW();
    for (const fn of chrome._listeners.runtime.onInstalled) await fn();

    const condition = chrome._dnrCalls[0].addRules[0].condition;
    assert.equal(condition.regexFilter, '^https?://go(?::\\d+)?/(.*)$');
    // Same vm-prototype issue as above — compare elements directly.
    assert.equal(condition.resourceTypes.length, 1);
    assert.equal(condition.resourceTypes[0], 'main_frame');
  });

  it('re-runs when serverUrl changes in storage', async () => {
    const { chrome } = loadSW();
    // Simulate storage.onChanged with a serverUrl change.
    // The listener calls updateRedirectRule() without returning its promise, so
    // we flush the microtask queue with setImmediate to let the async chain settle.
    const changes = { serverUrl: { oldValue: 'http://localhost:8080', newValue: 'http://new:1234' } };
    for (const fn of chrome._listeners.storage.onChanged) await fn(changes);
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(chrome._dnrCalls.length, 1);
  });

  it('does NOT re-run when an unrelated storage key changes', async () => {
    const { chrome } = loadSW();
    const changes = { token: { oldValue: '', newValue: 'abc' } };
    for (const fn of chrome._listeners.storage.onChanged) await fn(changes);
    // updateRedirectRule should NOT have been called
    assert.equal(chrome._dnrCalls.length, 0);
  });
});

// ── Omnibox: onInputChanged ───────────────────────────────────────────────────

describe('omnibox onInputChanged', () => {
  function makeFetch(links) {
    return () => Promise.resolve({
      ok:   true,
      json: () => Promise.resolve({ links }),
    });
  }

  it('calls fetch with the correct URL and query', async () => {
    let capturedUrl;
    const fetchMock = (url) => {
      capturedUrl = url;
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ links: [] }) });
    };

    const chrome = createChromeMock();
    loadSW(chrome, fetchMock);

    const suggest = () => {};
    await chrome._listeners.omnibox.inputChanged('gh', suggest);
    assert.equal(capturedUrl, 'http://localhost:8080/-/api/links?q=gh&limit=6');
  });

  it('passes the Bearer token in the Authorization header when set', async () => {
    let capturedHeaders;
    const fetchMock = (_url, opts) => {
      capturedHeaders = opts.headers;
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ links: [] }) });
    };

    const chrome = createChromeMock({ storage: { token: 'secret123' } });
    loadSW(chrome, fetchMock);

    await chrome._listeners.omnibox.inputChanged('x', () => {});
    assert.equal(capturedHeaders.Authorization, 'Bearer secret123');
  });

  it('omits the Authorization header when token is empty', async () => {
    let capturedHeaders;
    const fetchMock = (_url, opts) => {
      capturedHeaders = opts.headers;
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ links: [] }) });
    };

    const chrome = createChromeMock({ storage: { token: '' } });
    loadSW(chrome, fetchMock);

    await chrome._listeners.omnibox.inputChanged('x', () => {});
    assert.equal(capturedHeaders.Authorization, undefined);
  });

  it('calls suggest with one entry per returned link', async () => {
    const links = [
      { shortname: 'gh',      target_url: 'https://github.com' },
      { shortname: 'gh/repo', target_url: 'https://github.com/myrepo' },
    ];
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch(links));

    const results = [];
    await chrome._listeners.omnibox.inputChanged('gh', (s) => results.push(...s));
    assert.equal(results.length, 2);
    assert.equal(results[0].content, 'gh');
    assert.equal(results[1].content, 'gh/repo');
  });

  it('does NOT call suggest when the server returns no links', async () => {
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch([]));

    let called = false;
    await chrome._listeners.omnibox.inputChanged('zzz', () => { called = true; });
    assert.equal(called, false);
  });

  it('does NOT call fetch when input text is empty', async () => {
    let fetchCalled = false;
    const fetchMock = () => { fetchCalled = true; return Promise.resolve(); };

    const chrome = createChromeMock();
    loadSW(chrome, fetchMock);

    await chrome._listeners.omnibox.inputChanged('   ', () => {});
    assert.equal(fetchCalled, false);
  });

  it('does NOT call fetch when input text is only whitespace', async () => {
    let fetchCalled = false;
    const fetchMock = () => { fetchCalled = true; return Promise.resolve(); };

    const chrome = createChromeMock();
    loadSW(chrome, fetchMock);

    await chrome._listeners.omnibox.inputChanged('\t\n', () => {});
    assert.equal(fetchCalled, false);
  });

  it('silently ignores fetch errors', async () => {
    const chrome = createChromeMock();
    loadSW(chrome, () => Promise.reject(new Error('network error')));

    // Should not throw
    await assert.doesNotReject(
      () => chrome._listeners.omnibox.inputChanged('x', () => {}),
    );
  });

  it('silently ignores non-OK responses', async () => {
    const chrome = createChromeMock();
    loadSW(chrome, () => Promise.resolve({ ok: false, status: 401 }));

    let called = false;
    await chrome._listeners.omnibox.inputChanged('x', () => { called = true; });
    assert.equal(called, false);
  });

  it('URL-encodes the query text', async () => {
    let capturedUrl;
    const fetchMock = (url) => {
      capturedUrl = url;
      return Promise.resolve({ ok: true, json: () => Promise.resolve({ links: [] }) });
    };
    const chrome = createChromeMock();
    loadSW(chrome, fetchMock);

    await chrome._listeners.omnibox.inputChanged('hello world', () => {});
    assert.ok(capturedUrl.includes('q=hello%20world'), `URL was: ${capturedUrl}`);
  });
});

// ── Omnibox: onInputEntered ───────────────────────────────────────────────────

describe('omnibox onInputEntered', () => {
  it('navigates the current tab for "currentTab" disposition', async () => {
    const chrome = createChromeMock();
    loadSW(chrome);

    await chrome._listeners.omnibox.inputEntered('gh', 'currentTab');
    assert.ok(chrome._tabUpdate, 'tabs.update should have been called');
    assert.equal(chrome._tabUpdate[0].url, 'http://localhost:8080/gh');
  });

  it('creates a foreground tab for "newForegroundTab" disposition', async () => {
    const chrome = createChromeMock();
    loadSW(chrome);

    await chrome._listeners.omnibox.inputEntered('gh', 'newForegroundTab');
    assert.ok(chrome._tabCreate, 'tabs.create should have been called');
    assert.equal(chrome._tabCreate[0].url,    'http://localhost:8080/gh');
    assert.equal(chrome._tabCreate[0].active, undefined); // not explicitly set means foreground
  });

  it('creates a background tab for "newBackgroundTab" disposition', async () => {
    const chrome = createChromeMock();
    loadSW(chrome);

    await chrome._listeners.omnibox.inputEntered('gh', 'newBackgroundTab');
    assert.ok(chrome._tabCreate);
    assert.equal(chrome._tabCreate[0].active, false);
  });

  it('trims whitespace from the entered text', async () => {
    const chrome = createChromeMock();
    loadSW(chrome);

    await chrome._listeners.omnibox.inputEntered('  gh  ', 'currentTab');
    assert.equal(chrome._tabUpdate[0].url, 'http://localhost:8080/gh');
  });

  it('builds the URL from the configured serverUrl', async () => {
    const chrome = createChromeMock({ storage: { serverUrl: 'https://go.example.com' } });
    loadSW(chrome);

    await chrome._listeners.omnibox.inputEntered('slack', 'currentTab');
    assert.equal(chrome._tabUpdate[0].url, 'https://go.example.com/slack');
  });
});

// ── Utilities: escapeXml & truncate (via suggestion description) ─────────────

describe('escapeXml (via suggest output)', () => {
  function makeFetch(links) {
    return () => Promise.resolve({ ok: true, json: () => Promise.resolve({ links }) });
  }

  it('escapes & in shortnames', async () => {
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch([{ shortname: 'a&b', target_url: 'https://example.com' }]));

    const results = [];
    await chrome._listeners.omnibox.inputChanged('a', (s) => results.push(...s));
    assert.ok(results[0].description.includes('a&amp;b'), results[0].description);
  });

  it('escapes < and > in target URLs', async () => {
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch([{ shortname: 'x', target_url: 'https://ex.com/<path>' }]));

    const results = [];
    await chrome._listeners.omnibox.inputChanged('x', (s) => results.push(...s));
    assert.ok(results[0].description.includes('&lt;path&gt;'), results[0].description);
  });
});

describe('truncate (via suggest output)', () => {
  function makeFetch(links) {
    return () => Promise.resolve({ ok: true, json: () => Promise.resolve({ links }) });
  }

  it('does not truncate URLs shorter than 60 chars', async () => {
    const url = 'https://short.example.com';
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch([{ shortname: 's', target_url: url }]));

    const results = [];
    await chrome._listeners.omnibox.inputChanged('s', (s) => results.push(...s));
    assert.ok(results[0].description.includes('short.example.com'));
    assert.ok(!results[0].description.includes('…'));
  });

  it('truncates URLs longer than 60 chars with an ellipsis', async () => {
    const url = 'https://example.com/' + 'a'.repeat(60);
    const chrome = createChromeMock();
    loadSW(chrome, makeFetch([{ shortname: 'long', target_url: url }]));

    const results = [];
    await chrome._listeners.omnibox.inputChanged('long', (s) => results.push(...s));
    assert.ok(results[0].description.includes('…'), 'expected truncation ellipsis');
  });
});
