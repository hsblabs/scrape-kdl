import { parseArgs } from "node:util";
import { resolve } from "node:path";
import { checkNpmPackages } from "./check-npm-package.mjs";

const help = `Build inspected npm release archives without publishing them.

Usage:
  node scripts/build-npm-release.mjs --version VERSION [--output DIRECTORY]

Options:
  --version VERSION   Semantic Versioning package version, without a leading v
  --output DIRECTORY  Artifact directory (default: dist)
  -h, --help          Show this help
`;

try {
  const { values } = parseArgs({
    options: {
      version: { type: "string" },
      output: { type: "string", default: "dist" },
      help: { type: "boolean", short: "h", default: false },
    },
    strict: true,
  });
  if (values.help) {
    process.stdout.write(help);
  } else {
    if (values.version === undefined) throw new Error("--version is required; run with --help for usage");
    await checkNpmPackages({
      releaseVersion: values.version,
      outputDirectory: resolve(values.output),
    });
  }
} catch (error) {
  process.stderr.write(`build-npm-release: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exitCode = 1;
}
