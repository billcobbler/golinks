#!/usr/bin/env node
/**
 * generate.js — creates icon16.png, icon48.png, icon128.png
 *
 * Requires: npm install canvas
 * Via Docker: see Makefile `extension-icons` target
 *
 * Usage:
 *   node extension/icons/generate.js
 */
'use strict';

const fs   = require('node:fs');
const path = require('node:path');
const { createCanvas } = require('canvas');

const outDir = path.resolve(__dirname);
fs.mkdirSync(outDir, { recursive: true });

// Brand blue
const BG    = '#2563eb';
const FG    = '#ffffff';

function generateIcon(size) {
  const canvas = createCanvas(size, size);
  const ctx    = canvas.getContext('2d');

  const radius = size * 0.2;

  // Rounded rectangle background
  ctx.beginPath();
  ctx.moveTo(radius, 0);
  ctx.lineTo(size - radius, 0);
  ctx.quadraticCurveTo(size, 0, size, radius);
  ctx.lineTo(size, size - radius);
  ctx.quadraticCurveTo(size, size, size - radius, size);
  ctx.lineTo(radius, size);
  ctx.quadraticCurveTo(0, size, 0, size - radius);
  ctx.lineTo(0, radius);
  ctx.quadraticCurveTo(0, 0, radius, 0);
  ctx.closePath();
  ctx.fillStyle = BG;
  ctx.fill();

  // "go/" text — scale font to icon size
  ctx.fillStyle  = FG;
  ctx.textAlign  = 'center';
  ctx.textBaseline = 'middle';

  if (size >= 48) {
    const fontSize = Math.round(size * 0.38);
    ctx.font = `bold ${fontSize}px sans-serif`;
    ctx.fillText('go/', size / 2, size / 2);
  } else {
    // At 16px just draw a bold "G" — "go/" won't be legible
    const fontSize = Math.round(size * 0.58);
    ctx.font = `bold ${fontSize}px sans-serif`;
    ctx.fillText('G', size / 2, size / 2);
  }

  return canvas.toBuffer('image/png');
}

for (const size of [16, 48, 128]) {
  const file = path.join(outDir, `icon${size}.png`);
  fs.writeFileSync(file, generateIcon(size));
  console.log(`  created  ${path.relative(process.cwd(), file)}`);
}

console.log('Icons generated.');
