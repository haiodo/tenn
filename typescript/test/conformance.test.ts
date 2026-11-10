// Runs the shared conformance suite in typescript/: every case must produce exactly the canonical AST
// that go/conformance_test.go generated, and every error-free case must print exactly expected/*.tenn.
//
// The canonical AST is described in conformance/README.md. Positions there are 1-based lines and
// 0-based columns in Unicode scalar values, so the UTF-16 positions of this implementation are
// converted before comparing.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { TennParser, toStr } from "../src/index.ts";
import type { TennNode, TennToken } from "../src/index.ts";

const root = fileURLToPath(new URL("../../conformance/", import.meta.url));

interface Pos {
  file: string;
  line: number;
  col: number;
}

interface CanonNode {
  kind: string;
  name?: string;
  value?: string;
  pos: Pos;
  args?: CanonNode[];
  block?: CanonNode[];
  blockIndex?: number;
  blockPos?: Pos;
}

function position(file: string, source: string, token: TennToken | null): Pos {
  const end = token?.pos ?? 0;
  let line = 1;
  let lineStart = 0;
  for (let i = 0; i < end; i++) {
    if (source.charCodeAt(i) === 10) {
      line += 1;
      lineStart = i + 1;
    }
  }
  return { file, line, col: Array.from(source.slice(lineStart, end)).length };
}

function commands(node: TennNode, file: string, source: string): CanonNode[] {
  return (node.children ?? []).map((c) => command(c, file, source));
}

function value(node: TennNode, file: string, source: string): CanonNode {
  if (node.kind === "BlockExpr") {
    return { kind: "BlockExpr", pos: position(file, source, node.token), block: commands(node, file, source) };
  }
  return { kind: node.kind, value: node.getIdentText() ?? "", pos: position(file, source, node.token) };
}

function command(node: TennNode, file: string, source: string): CanonNode {
  const name = node.getIdent(0) ?? "";
  const out: CanonNode = { kind: "Command", name, pos: position(file, source, node.token) };
  const args: CanonNode[] = [];
  let block: CanonNode[] | undefined;
  let blockPos: Pos | undefined;
  let blockIndex = -1;
  for (const c of (node.children ?? []).slice(1)) {
    if (c.kind === "BlockExpr" && block === undefined) {
      blockIndex = args.length;
      block = commands(c, file, source);
      blockPos = position(file, source, c.token);
      continue;
    }
    args.push(value(c, file, source));
  }
  out.args = args;
  if (block !== undefined) {
    out.block = block;
    if (blockIndex < args.length) {
      out.blockIndex = blockIndex;
    }
    out.blockPos = blockPos;
  }
  return out;
}

function build(name: string, source: string): { tree: TennNode; canon: object; errors: boolean } {
  const parser = new TennParser();
  const tree = parser.parse(source);
  const file = `cases/${name}`;
  return {
    tree,
    errors: parser.errors.hasErrors(),
    canon: {
      kind: "Statements",
      file,
      children: commands(tree, file, source),
      errors: parser.errors.errors.map((e) => ({ code: e.errorCode, message: e.message })),
    },
  };
}

const cases = readdirSync(root + "cases")
  .filter((f) => f.endsWith(".tenn"))
  .sort();

assert.ok(cases.length >= 15, `want at least 15 conformance cases, found ${cases.length}`);

for (const name of cases) {
  test(`conformance ${name}`, () => {
    const source = readFileSync(root + "cases/" + name, "utf8");
    const { tree, canon, errors } = build(name, source);

    const wantJSON = readFileSync(root + "expected/" + name.replace(/\.tenn$/, ".json"), "utf8");
    assert.equal(JSON.stringify(canon, null, 2) + "\n", wantJSON);

    const tennPath = root + "expected/" + name.replace(/\.tenn$/, ".tenn");
    if (errors) {
      assert.throws(() => readFileSync(tennPath), /ENOENT/, `${name} must not have a printer golden`);
      return;
    }
    assert.equal(toStr(tree) + "\n", readFileSync(tennPath, "utf8"));
  });
}
