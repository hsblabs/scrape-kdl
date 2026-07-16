import { readFile, readdir } from "node:fs/promises";
import { extname, join, relative } from "node:path";

const root = process.cwd();
const sourceRoots = ["packages/scrape-kdl/src", "packages/scrape-kdl-playwright/src"];
const files = [];
for (const sourceRoot of sourceRoots) await collect(join(root, sourceRoot), files);

const problems = [];
for (const path of files.sort()) {
  if (extname(path) !== ".ts") continue;
  const displayPath = relative(root, path).replaceAll("\\", "/");
  const source = await readFile(path, "utf8");
  if (source.includes("\r")) problems.push(`${displayPath}: use LF line endings`);
  if (source.includes("\t")) problems.push(`${displayPath}: tabs are not allowed`);
  if (/@ts-(?:ignore|nocheck)/u.test(source))
    problems.push(`${displayPath}: TypeScript suppression directives are not allowed`);
  if (/\b(?:as|:) +any\b/u.test(source)) problems.push(`${displayPath}: use an explicit type instead of any`);
  if (/\bconsole\./u.test(source)) problems.push(`${displayPath}: package source must not write to the console`);
  source.split("\n").forEach((line, index) => {
    if (/\s+$/u.test(line)) problems.push(`${displayPath}:${index + 1}: trailing whitespace`);
  });
  for (const match of source.matchAll(/\bfrom +["'](\.[^"']+)["']/gu)) {
    if (!match[1].endsWith(".js"))
      problems.push(`${displayPath}: relative import must use its emitted .js extension: ${match[1]}`);
  }
}

if (problems.length > 0) {
  console.error(`TypeScript lint failed with ${problems.length} problem(s):\n- ${problems.join("\n- ")}`);
  process.exit(1);
}
console.log(`TypeScript lint: ${files.length} source files passed`);

async function collect(directory, output) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) await collect(path, output);
    else if (entry.isFile()) output.push(path);
  }
}
