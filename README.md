# TENN — Tool-Extensible Node Notation

[Русская версия](README.ru.md)

TENN is a text notation for trees of commands with literal arguments. It is meant to be a **compact
protocol**: small enough that a person writes it by hand, regular enough that a tool reads it without
a schema, and expressive enough to carry a diagram, a document, a configuration, or a call to an MCP
tool.

One grammar rule:

```
command := name argument* block?
block   := { command* }
```

No schema. No reserved words. The meaning comes from the consumer: `element "Screen" { item A { pos 0 0 } }`
is a diagram, `service api { port 8080 }` is a configuration, `set "item A" title "Hello"` is a tool
call. One parser serves all of them.

```
element "Screen" {
    item A { pos 0 0 }
    item B { pos 320 0 }
    link A B
}
```

## Why

Most exchange formats force a choice: either a rigid schema (fast to validate, painful to extend) or
free-form JSON (easy to extend, verbose and unreadable when nested). TENN takes the Tcl shape — a name
followed by arguments and a braced body — and stops there.

- **It extends by adding commands.** There is nothing to reserve, no registry to update. A tool that
  understands `ports` ignores `owners`, and a file written for a newer tool still parses in an older one.
- **It is compact.** No quotes around names, no commas, no brackets: a screen outline or a mind map is
  a few hundred bytes, not a few thousand.
- **It is diff-friendly.** One command per line, indentation is presentation, `git diff` shows what
  changed.
- **It is safe to read.** Nothing executes while parsing. Expressions live inside `${...}` and are
  evaluated by the consumer, not by the parser.
- **It has one implementation to port.** The whole syntax fits in a lexer, a recursive-descent parser
  and a printer — a few hundred lines per language, which is why Go, TypeScript and Swift
  implementations coexist here.

For an MCP tool, TENN is a natural request/response body: `edit path "x.tenn"`, `set prop value`,
`run check`, with a `{}` block for a list of operations. For documents, a command names a section and a
`%{...}` literal carries its markdown body. The same file can hold both.

## Nine value kinds

Arguments are one of nine values, and that is the entire data model:

| Kind | Example | Meaning |
|------|---------|---------|
| `Ident` | `color red` | a bare word |
| `IntLit` | `pos 10 20` | integer |
| `FloatLit` | `width 1.5` | float |
| `StringLit` | `title "A b"` | quoted text |
| `CharLit` | — | modeled for parity, not produced by the lexer |
| `Expression` | `$(1 + 2)` | an expression for the consumer |
| `ExpressionBlock` | `${ var a = 1; a }` | a statement body for the consumer |
| `MarkdownLit` | `%{**bold**}` | a markdown body |
| `Image` | `@(iVBORw0K...)` | base64 image data |

`;` and newlines separate commands; `//` and `/* ... */` are comments. Everything else — braces,
dollars, percent, at-signs — only delimits one of the forms above.

## Lineage

TENN is a direct descendant of the **Tool Command Language**. It keeps Tcl's command shape, its braced
body as an unevaluated word, and its taste for building a small language out of a single rule. It
replaces Tcl's `[ ... ]` substitution with `${ ... }` / `$( ... )`, and drops variables, runtime scopes
and return values: TENN is declarative data with embedded expressions, not a scripting language.

The full comparison is in [spec/core.md, section 6](spec/core.md#lineage).

## Repository layout

```
go.mod             the Go module github.com/haiodo/tenn (package tenn lives in go/)
Package.swift      the SwiftPM manifest, at the root so that a URL dependency finds it
spec/              the format: grammar, AST, printing, profiles
  core.md            language core
  profiles/          vocabularies built on the same grammar (scene.md)
go/                Go package tenn: import github.com/haiodo/tenn/go
typescript/        npm package @haiodo/tenn
swift/             Swift sources of the package Tenn (path of the target in Package.swift)
conformance/       shared cases and canonical AST for all implementations
```

The two manifests live at the root on purpose: SwiftPM looks for `Package.swift` at the root of a
package fetched by URL, and Go wants the module path to match the repository. One repository, three
toolchains, one tag `v0.1.0` for Go and Swift; the npm package carries its own `version`.

## Use it

**Go** — the module path is **case-sensitive**; use it exactly as written:

```
go get github.com/haiodo/tenn/go
```

```go
import tenn "github.com/haiodo/tenn/go"

tree, parser := tenn.Parse(`element "Screen" { item A { pos 0 0 } }`)
_ = parser.Errors
```

**TypeScript / Node 24** — the package lives in `typescript/`, runs on `.ts` directly, and is not
published to the npm registry. Install it from a clone, or link it:

```
npm install ../tenn/typescript
```

```ts
import { parse, toStr } from "@haiodo/tenn";

const { tree, parser } = parse('element "Screen" { item A { pos 0 0 } }');
toStr(tree);
```

**Swift** — SwiftPM package `Tenn`, macOS 13+:

```swift
.package(url: "https://github.com/haiodo/tenn.git", from: "0.1.0")
```

## Conformance

The three implementations must produce the same tree for the same text. [`conformance/`](conformance/README.md)
holds the shared cases, the canonical AST in `expected/*.json`, and the canonical printer output in
`expected/*.tenn`. Every implementation runs the same suite; adding a case means adding one `.tenn`
file and regenerating the expected output from a reference implementation.

## Spec

- [spec/core.md](spec/core.md) — grammar, lexical rules, value kinds, AST, printing, lineage
- [spec/profiles/scene.md](spec/profiles/scene.md) — the Tenniarb scene profile

## License

Eclipse Public License 2.0. See [LICENSE](LICENSE).
