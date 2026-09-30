"use strict";

// One npm package per prebuilt binary, named after Node's process.platform and
// process.arch so npm's os/cpu matching picks exactly the one this machine
// needs. platforms.json is the single list: this module, stage-platforms.js
// and the Go test that keeps package.json in step all read it.

const PLATFORMS = require("./platforms.json");

const SCOPE = "@z40";
const NAME = "magpie";

function packageName(platform) {
  return `${SCOPE}/${NAME}-${platform.node}`;
}

function binaryName(platform) {
  return platform.goos === "windows" ? `${NAME}.exe` : NAME;
}

// forHost returns the entry for this machine, or undefined when no prebuilt
// binary exists for it.
function forHost(nodePlatform = process.platform, nodeArch = process.arch) {
  return PLATFORMS.find((p) => p.node === `${nodePlatform}-${nodeArch}`);
}

module.exports = { PLATFORMS, SCOPE, NAME, packageName, binaryName, forHost };
