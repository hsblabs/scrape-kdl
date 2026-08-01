#!/usr/bin/env node

import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";

const [archive, ...extra] = process.argv.slice(2);
if (!archive || extra.length > 0) {
  console.error("usage: npm-archive-integrity.mjs <archive.tgz>");
  process.exit(2);
}

const data = await readFile(archive);
const digest = createHash("sha512").update(data).digest("base64");
process.stdout.write(`sha512-${digest}\n`);
