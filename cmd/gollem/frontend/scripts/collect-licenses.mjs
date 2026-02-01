#!/usr/bin/env node
// Collects LICENSE information from all production dependencies using
// pnpm licenses list and bundles them into a single text file.

import { execSync } from "child_process";
import { readFileSync, writeFileSync, existsSync, mkdirSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const rootDir = join(__dirname, "..");
const outDir = join(rootDir, "public");
const outFile = join(outDir, "licenses.txt");

const licenseFileNames = [
  "LICENSE",
  "LICENSE.md",
  "LICENSE.txt",
  "LICENCE",
  "LICENCE.md",
  "LICENCE.txt",
  "license",
  "license.md",
  "license.txt",
];

function findLicenseText(pkgDir) {
  for (const name of licenseFileNames) {
    const p = join(pkgDir, name);
    if (existsSync(p)) {
      return readFileSync(p, "utf-8").trim();
    }
  }
  return null;
}

// Run pnpm licenses list for production deps
const raw = execSync("pnpm licenses list --prod --json", {
  cwd: rootDir,
  encoding: "utf-8",
});
const byLicense = JSON.parse(raw);

// Flatten into a sorted list
const entries = [];
for (const [, pkgs] of Object.entries(byLicense)) {
  for (const pkg of pkgs) {
    const pkgDir = pkg.paths?.[0];
    entries.push({
      name: pkg.name,
      version: (pkg.versions || []).join(", "),
      license: pkg.license || "unknown",
      author: pkg.author || "",
      homepage: pkg.homepage || "",
      licenseText: pkgDir ? findLicenseText(pkgDir) : null,
    });
  }
}
entries.sort((a, b) => a.name.localeCompare(b.name));

const separator = "=".repeat(72);
const lines = [
  "THIRD-PARTY LICENSES",
  "",
  `This file contains the licenses of all ${entries.length} third-party packages`,
  "used in the gollem trace viewer frontend.",
  "",
  separator,
];

for (const dep of entries) {
  lines.push("");
  lines.push(`Package: ${dep.name}`);
  lines.push(`Version: ${dep.version}`);
  lines.push(`License: ${dep.license}`);
  if (dep.author) lines.push(`Author:  ${dep.author}`);
  if (dep.homepage) lines.push(`URL:     ${dep.homepage}`);
  lines.push("");
  if (dep.licenseText) {
    lines.push(dep.licenseText);
  } else {
    lines.push(`(License file not found in package — licensed under ${dep.license})`);
  }
  lines.push("");
  lines.push(separator);
}

if (!existsSync(outDir)) {
  mkdirSync(outDir, { recursive: true });
}
writeFileSync(outFile, lines.join("\n") + "\n");
console.log(`Collected ${entries.length} licenses -> ${outFile}`);
