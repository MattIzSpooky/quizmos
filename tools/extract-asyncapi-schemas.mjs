#!/usr/bin/env node
// Extracts components.schemas from api/asyncapi.yaml into standalone JSON
// Schema files, so quicktype (which speaks JSON Schema, not AsyncAPI) can
// generate matching Go and TypeScript types from a single spec-first source.
import { readFileSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import yaml from "js-yaml";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const specPath = path.join(rootDir, "api", "asyncapi.yaml");
const outDir = path.join(rootDir, "api", "asyncapi", "generated-schemas");

const spec = yaml.load(readFileSync(specPath, "utf8"));
const schemas = spec?.components?.schemas;
if (!schemas || Object.keys(schemas).length === 0) {
  throw new Error(`No components.schemas found in ${specPath}`);
}

// Rewrite internal $refs (#/components/schemas/X) to sibling-file refs
// (X.schema.json#) so cross-references resolve once each schema is split
// into its own file.
function rewriteRefs(node) {
  if (Array.isArray(node)) {
    node.forEach(rewriteRefs);
    return;
  }
  if (node && typeof node === "object") {
    if (typeof node.$ref === "string") {
      const prefix = "#/components/schemas/";
      if (node.$ref.startsWith(prefix)) {
        const name = node.$ref.slice(prefix.length);
        node.$ref = `${name}.schema.json#`;
      }
    }
    for (const value of Object.values(node)) {
      rewriteRefs(value);
    }
  }
}

rmSync(outDir, { recursive: true, force: true });
mkdirSync(outDir, { recursive: true });

for (const [name, schema] of Object.entries(schemas)) {
  const copy = structuredClone(schema);
  rewriteRefs(copy);
  const doc = {
    $schema: "http://json-schema.org/draft-07/schema#",
    title: name,
    ...copy,
  };
  writeFileSync(
    path.join(outDir, `${name}.schema.json`),
    JSON.stringify(doc, null, 2) + "\n",
  );
}

console.log(`Wrote ${Object.keys(schemas).length} schema file(s) to ${path.relative(rootDir, outDir)}`);
