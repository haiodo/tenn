# Conformance suite

The format has three implementations. They stay compatible only if they agree on the tree they build
from the same text. This directory is that agreement: hand-written cases, one canonical AST per case,
and the canonical printer output.

Nothing here is about any profile. The cases use only the core grammar: commands, blocks, separators,
comments, numbers, strings, expressions, markdown and images. A scene or a document profile is checked
by whoever implements it, on top of this suite.

## Layout

```
cases/<name>.tenn        input
expected/<name>.json     canonical AST for that input
expected/<name>.tenn     canonical printer output; absent when the case is an error
```

Both Go and TypeScript run the whole suite today. Swift has no test target yet (see below).

## The canonical AST

`expected/<name>.json` is the exact JSON the reference implementation produces. The comparison is
byte-for-byte, so the formatting is part of the contract:

- UTF-8, two-space indent, one trailing newline;
- keys in the order shown below; `"<"`, `">"` and `"&"` are **not** escaped (Go must encode with
  `SetEscapeHTML(false)`);
- object keys are only the ones listed; absent optional fields are omitted, not null.

### File

```json
{
  "kind": "Statements",
  "file": "cases/basic.tenn",
  "children": [ ... ],
  "errors": []
}
```

`errors` is `[{ "code": "...", "message": "..." }]` from the parser; it is `[]` when the case parses.
The tree is the partial tree the parser recovered, so an error case still has `children`.

### Command

```json
{
  "kind": "Command",
  "name": "item",
  "pos": { "file": "cases/basic.tenn", "line": 1, "col": 0 },
  "args": [ ... ]
}
```

| Field | Meaning |
|-------|---------|
| `name` | the first token of the command, its identifier |
| `pos` | position of that name token |
| `args` | arguments in source order; always present, may be `[]` |
| `block` | present when the command has a block: an array of commands |
| `blockIndex` | present only when the block is **not** the last argument; the 0-based index of the block in the full argument list |
| `blockPos` | present with `block`: position of `{` |

A consumer reconstructs the source order by taking `args` and inserting `block` at `blockIndex`
(or appending it when `blockIndex` is absent). A command with a second block keeps it in `args` as a
`BlockExpr` value.

### Value

```json
{
  "kind": "IntLit",
  "value": "10",
  "pos": { "file": "cases/basic.tenn", "line": 1, "col": 10 }
}
```

`kind` is one of `Ident`, `IntLit`, `FloatLit`, `StringLit`, `CharLit`, `Expression`,
`ExpressionBlock`, `MarkdownLit`, `Image`. `value` is the literal text exactly as the lexer stores it:
the content between the delimiters, with the escaped delimiter resolved (`"a\"b"` → `a"b`), and with any
other backslash left alone.

A `BlockExpr` value (a block that is not the command's first one) has no `value`:

```json
{ "kind": "BlockExpr", "pos": { ... }, "block": [ ... ] }
```

### Positions

`line` is 1-based, `col` is 0-based and counted in **Unicode scalar values** — not bytes and not UTF-16
code units, so all three implementations can compare without knowing each other's string model.

The position is the start of the token's source span:

- a command: its name;
- a block: its `{`;
- a literal: the content **after** the opening delimiter (`"abc"` starting at column 0 has its
  `StringLit` at column 1).

For a multi-line literal the span starts at the first content character, even though the lexer's own
`line`/`col` counters have moved past it by the time the token is recorded. The conformance format
deliberately records the start of the span, which is the useful one for source mapping.

## Printer golden

`expected/<name>.tenn` is `toStr(tree, 0, false)` with a single `\n` appended (the printer itself does
not terminate the line). It pins indentation, quoting and the normalized form of separators. Cases
that report a parse error have **no** `.tenn` file: an error-recovery tree is not meant to print back.
If a case starts failing to parse, its old `.tenn` has to be removed by hand.

## Running the suite

**Go** — the suite is part of `go test`:

```
cd go
go vet ./...
go test ./...            # includes TestConformance
go test -run TestConformance -v ./...
```

**TypeScript** — `test/conformance.test.ts` reads the same directory:

```
cd typescript
npm ci
npm test                 # node --test, includes the conformance tests
npm run typecheck
```

**Swift** — not wired yet. The `Tenn` package has no test target; adding one means an XCTest case that
parses `../conformance/cases/*.tenn` and compares the same JSON. The Swift node model already exposes
`toStr`, so the printer half is a straight copy of the TypeScript runner.

Both runners need the repository layout intact: the directories are found relative to `go/` and
`typescript/` (`../conformance`), not passed as configuration.

## Adding a case

1. Write `cases/<name>.tenn` by hand. Keep it small: one grammar feature per case, plus the edge cases
   worth pinning.
2. From `go/`, regenerate from the Go implementation:

   ```
   TENN_UPDATE_CONFORMANCE=1 go test -run TestConformance .
   ```

3. **Read the diff.** The generator records what the parser produced, not what the grammar ought to
   produce. Check the tree by eye: kinds, argument order, block placement, positions. If the output is
   wrong, fix the parser or the case — do not commit the golden as is.
4. Run both suites (`go test ./...` and `npm test`). The TypeScript run is the cross-check that the two
   implementations agreed on the canonical form.
5. Commit the case, the JSON and (for a clean parse) the `.tenn` together.

The `.json` files are generated from the **Go** implementation; Go is the reference for this suite.
TypeScript is an independent verifier, which is why a disagreement is a real bug in one of them.
