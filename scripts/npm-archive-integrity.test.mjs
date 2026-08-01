import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import test from "node:test";

const execFileAsync = promisify(execFile);

test("prints the npm sha512 integrity of an archive", async () => {
  const directory = await mkdtemp(join(tmpdir(), "npm-integrity-"));
  try {
    const archive = join(directory, "package.tgz");
    await writeFile(archive, "scrape-kdl\n");
    const { stdout } = await execFileAsync(process.execPath, [
      new URL("./npm-archive-integrity.mjs", import.meta.url).pathname,
      archive,
    ]);
    assert.equal(
      stdout,
      "sha512-XsffPbkhODHefLcBfkFvgaIqAWOvGaErJ6+zIGCthDC670wjSdqcmpdBaQe/XJ2naD4UCBhsTRiLOgyWvlQIMw==\n",
    );
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
