'use strict';

/**
 * createChromeMock(opts)
 *
 * Returns a minimal chrome API mock that captures registered event listeners,
 * records declarativeNetRequest calls, and simulates chrome.storage.sync.
 *
 * The returned object exposes `_listeners`, `_dnrCalls`, and `_storage` for
 * inspection inside tests.
 *
 * @param {object} [opts]
 * @param {object} [opts.storage]  Initial storage values (merged over defaults).
 * @param {object} [opts.tabUrl]   URL to return from tabs.query (active tab).
 * @param {string} [opts.tabTitle] Title to return from tabs.query.
 */
function createChromeMock(opts = {}) {
  const storage = {
    serverUrl: 'http://localhost:8080',
    token: '',
    ...opts.storage,
  };

  const tabStub = {
    url:   opts.tabUrl   ?? 'https://example.com/page',
    title: opts.tabTitle ?? 'Example Page',
  };

  // Captured listener references so tests can invoke them directly.
  const listeners = {
    omnibox: {
      inputChanged: null,
      inputEntered:  null,
    },
    runtime: {
      onInstalled: [],
      onStartup:   [],
    },
    storage: {
      onChanged: [],
    },
  };

  // All calls to declarativeNetRequest.updateDynamicRules are pushed here.
  const dnrCalls = [];

  const mock = {
    // ── Test introspection ──────────────────────────────────────────────────
    _listeners: listeners,
    _dnrCalls:  dnrCalls,
    _storage:   storage,

    // ── chrome.storage ──────────────────────────────────────────────────────
    storage: {
      sync: {
        get:  (defaults) => Promise.resolve({ ...defaults, ...storage }),
        set:  (values)   => { Object.assign(storage, values); return Promise.resolve(); },
      },
      onChanged: {
        addListener: (fn) => listeners.storage.onChanged.push(fn),
      },
    },

    // ── chrome.omnibox ──────────────────────────────────────────────────────
    omnibox: {
      setDefaultSuggestion: () => {},
      onInputChanged: {
        addListener: (fn) => { listeners.omnibox.inputChanged = fn; },
      },
      onInputEntered: {
        addListener: (fn) => { listeners.omnibox.inputEntered = fn; },
      },
    },

    // ── chrome.runtime ──────────────────────────────────────────────────────
    runtime: {
      onInstalled: { addListener: (fn) => listeners.runtime.onInstalled.push(fn) },
      onStartup:   { addListener: (fn) => listeners.runtime.onStartup.push(fn) },
      openOptionsPage: () => {},
    },

    // ── chrome.tabs ─────────────────────────────────────────────────────────
    tabs: {
      update: (...args) => { mock._tabUpdate = args; },
      create: (...args) => { mock._tabCreate = args; },
      query:  ()        => Promise.resolve([tabStub]),
    },

    // ── chrome.declarativeNetRequest ────────────────────────────────────────
    declarativeNetRequest: {
      updateDynamicRules: (args) => {
        dnrCalls.push(args);
        return Promise.resolve();
      },
    },
  };

  return mock;
}

module.exports = { createChromeMock };
