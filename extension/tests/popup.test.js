'use strict';
/**
 * Popup tests — Node.js built-in test runner
 *
 * Run:  node --test extension/tests/popup.test.js
 *   or: make extension-test
 *
 * The popup script uses `document` and `DOMContentLoaded`, so full UI
 * interaction tests would require a DOM environment.  These tests focus on:
 *   • the pure `slugify` utility (loaded via vm, no DOM needed)
 *   • the `authHeaders` helper
 */

const { describe, it } = require('node:test');
const assert = require('node:assert/strict');
const fs     = require('node:fs');
const path   = require('node:path');
const vm     = require('node:vm');

const { createChromeMock } = require('./helpers/chrome_mock.js');

const POPUP_SRC = fs.readFileSync(
  path.join(__dirname, '..', 'popup', 'popup.js'),
  'utf8',
);

/**
 * Load popup.js in a vm context that stubs out the browser globals it
 * touches at module load-time.  Returns the context so tests can call
 * functions that are hoisted onto it.
 */
function loadPopup(chromeMock) {
  const chrome = chromeMock ?? createChromeMock();

  // Minimal document stub — prevents errors when the script registers the
  // DOMContentLoaded listener (the callback itself is never fired here).
  const document = {
    addEventListener: () => {},
    getElementById:   () => null,
  };

  const ctx = vm.createContext({ chrome, document, window: {}, console });
  vm.runInContext(POPUP_SRC, ctx);
  return ctx;
}

// ── slugify ───────────────────────────────────────────────────────────────────

describe('slugify', () => {
  let ctx;
  // Load once; slugify is a pure function — safe to share across tests.
  ctx = loadPopup();

  it('lowercases the title', () => {
    assert.equal(ctx.slugify('Hello World'), 'hello-world');
  });

  it('replaces spaces with hyphens', () => {
    assert.equal(ctx.slugify('my page title'), 'my-page-title');
  });

  it('replaces non-alphanumeric characters (except /) with hyphens', () => {
    assert.equal(ctx.slugify('GitHub: Issues & PRs'), 'github-issues-prs');
  });

  it('preserves forward slashes', () => {
    assert.equal(ctx.slugify('org/repo'), 'org/repo');
  });

  it('collapses multiple consecutive separators into one hyphen', () => {
    assert.equal(ctx.slugify('foo   ---   bar'), 'foo-bar');
  });

  it('strips leading hyphens', () => {
    assert.equal(ctx.slugify('---hello'), 'hello');
  });

  it('strips trailing hyphens', () => {
    assert.equal(ctx.slugify('hello---'), 'hello');
  });

  it('truncates to 40 characters', () => {
    const long = 'a'.repeat(50);
    const result = ctx.slugify(long);
    assert.ok(result.length <= 40, `Expected ≤40 chars, got ${result.length}`);
  });

  it('returns an empty string for an all-separator title', () => {
    // "!!! ???" → all non-alphanumeric → collapsed to '-' → trimmed → ''
    assert.equal(ctx.slugify('!!! ???'), '');
  });

  it('handles a title that is already a valid shortname', () => {
    assert.equal(ctx.slugify('github'), 'github');
  });

  it('handles numbers', () => {
    assert.equal(ctx.slugify('Phase 2 — CLI'), 'phase-2-cli');
  });
});

// ── authHeaders ───────────────────────────────────────────────────────────────

describe('authHeaders', () => {
  let ctx;
  ctx = loadPopup();

  it('includes Authorization header when token is set', () => {
    const headers = ctx.authHeaders('mytoken');
    assert.equal(headers['Authorization'], 'Bearer mytoken');
  });

  it('omits Authorization header when token is empty string', () => {
    const headers = ctx.authHeaders('');
    assert.equal(headers['Authorization'], undefined);
  });

  it('always includes Content-Type: application/json', () => {
    const headers = ctx.authHeaders('');
    assert.equal(headers['Content-Type'], 'application/json');
  });

  it('merges extra headers', () => {
    const headers = ctx.authHeaders('', { 'X-Custom': 'yes' });
    assert.equal(headers['X-Custom'], 'yes');
    assert.equal(headers['Content-Type'], 'application/json');
  });

  it('extra headers can override Content-Type', () => {
    const headers = ctx.authHeaders('', { 'Content-Type': '' });
    assert.equal(headers['Content-Type'], '');
  });
});
