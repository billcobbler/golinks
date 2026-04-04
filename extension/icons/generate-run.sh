#!/bin/sh
# Installs the canvas dependency and runs generate.js.
# Executed inside the node:lts Docker container.
set -e
cd /ext/icons
npm install --silent canvas
node generate.js
