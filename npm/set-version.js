#!/usr/bin/env node

"use strict";

// set-version.js <version>
//
// Bumps every place a release version lives: npm/package.json's version, the
// pinned optionalDependencies on the platform packages, and the Makefile's
// VERSION. The platform packages must be pinned exactly — a range could pair
// this wrapper with a binary from another release.

const fs = require("fs");
const path = require("path");
const { PLATFORMS, packageName } = require("./platforms");

const version = (process.argv[2] || "").replace(/^v/, "");
if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.]+)?$/.test(version)) {
  console.error("usage: set-version.js <version>   e.g. 1.0.5");
  process.exit(2);
}

const pkgPath = path.join(__dirname, "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
const optionalDependencies = Object.fromEntries(PLATFORMS.map((p) => [packageName(p), version]));
fs.writeFileSync(pkgPath, JSON.stringify({ ...pkg, version, optionalDependencies }, null, 2) + "\n");

const makefilePath = path.join(__dirname, "..", "Makefile");
const makefile = fs.readFileSync(makefilePath, "utf8");
const bumped = makefile.replace(/^VERSION := v\S+$/m, `VERSION := v${version}`);
if (bumped === makefile && !makefile.includes(`VERSION := v${version}\n`)) {
  console.error("set-version: no `VERSION := v...` line in the Makefile");
  process.exit(1);
}
fs.writeFileSync(makefilePath, bumped);

console.log(`set-version: npm/package.json and Makefile now at ${version}`);
