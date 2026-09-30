#!/usr/bin/env node

"use strict";

// stage-platforms.js <version> <binaries-dir> <out-dir>
//
// Turns the release's raw binaries (magpie-v<version>-<goos>-<goarch>[.exe],
// as `make release-all` names them) into one publishable npm package per
// platform under <out-dir>/<node-platform>/. The main package lists these as
// optionalDependencies; npm installs only the one whose os/cpu match, so the
// binary arrives with `npm install` and no install script has to run.

const fs = require("fs");
const path = require("path");
const MAIN = require("./package.json");
const { PLATFORMS, NAME, packageName, binaryName } = require("./platforms");

function stagePlatform(platform, version, binDir, outDir) {
  const ext = platform.goos === "windows" ? ".exe" : "";
  const source = path.join(binDir, `${NAME}-v${version}-${platform.goos}-${platform.goarch}${ext}`);
  if (!fs.existsSync(source)) {
    throw new Error(`missing release binary ${source}`);
  }

  const [os, cpu] = platform.node.split("-");
  const dir = path.join(outDir, platform.node);
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(path.join(dir, "bin"), { recursive: true });

  const target = path.join(dir, "bin", binaryName(platform));
  fs.copyFileSync(source, target);
  fs.chmodSync(target, 0o755);

  const manifest = {
    name: packageName(platform),
    version,
    description: `The ${platform.node} binary for ${MAIN.name}. Install ${MAIN.name} instead.`,
    homepage: MAIN.homepage,
    repository: MAIN.repository,
    license: MAIN.license,
    author: MAIN.author,
    os: [os],
    cpu: [cpu],
    files: ["bin/"],
    publishConfig: { access: "public" },
  };
  fs.writeFileSync(path.join(dir, "package.json"), JSON.stringify(manifest, null, 2) + "\n");
  fs.writeFileSync(
    path.join(dir, "README.md"),
    `# ${manifest.name}\n\nThe prebuilt ${platform.node} binary for [${MAIN.name}](${MAIN.homepage}).\n` +
      `Do not install this directly: \`npm install -g ${MAIN.name}\` picks it for you.\n`
  );
  return dir;
}

function main() {
  const [version, binDir, outDir] = process.argv.slice(2);
  if (!version || !binDir || !outDir) {
    console.error("usage: stage-platforms.js <version> <binaries-dir> <out-dir>");
    process.exit(2);
  }
  if (version !== MAIN.version) {
    console.error(`stage-platforms: version ${version} does not match npm/package.json (${MAIN.version})`);
    process.exit(1);
  }
  for (const platform of PLATFORMS) {
    console.log(stagePlatform(platform, version, binDir, outDir));
  }
}

try {
  main();
} catch (err) {
  console.error(`stage-platforms: ${err.message}`);
  process.exit(1);
}
