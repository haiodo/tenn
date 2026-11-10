# TENN core

TENN (**Tool-Extensible Node Notation**) is a text notation for trees of commands with literal
arguments. It has one grammar rule, no schema and no reserved words. A consumer gives the commands
meaning; the parser only builds the tree.

```tenn
element "Screen" { item A { pos 0 0 } }
```

The same text is a diagram for a drawing tool, a screen for a UI builder, a service description for a
configuration reader, or a call to an editor tool. Nothing in the syntax says which.

## 1. Grammar

```
file        := command*
command     := name arg*
arg         := value | block
block       := "{" command* "}"
value       := ident | int | float | string
             | expression | expression-block | markdown | image
```

- `name` is a bare word (`ident`). It is the first token of a command.
- A command ends at `;`, at a newline separator, at the closing `}` of its block, or at end of file.
- Commands are separated by `;` or by a newline. A newline only separates commands at file level and
  directly inside a `{ ... }` block; inside an expression or markdown literal it is ordinary text.
- A block may appear anywhere among the arguments, and a command may have more than one. In practice a
  block is last: `item A { pos 0 0 }`.
- There is no escaping of blocks: the body of `{ ... }` is data, not a computation.

A bare newline before a string literal or before `{` does **not** end the command, so a long command
may be broken across lines:

```tenn
element "Wide"
{
    item A { pos 0 0 }
}
```

is one `element` command with a block, not three commands.

## 2. Lexical layer

Source text is UTF-8. The reference implementations expose token positions as byte offsets (Go,
Swift) or UTF-16 code units (TypeScript); the canonical positions used by `conformance/` are
1-based lines and 0-based columns counted in Unicode scalar values.

### Whitespace and separators

Only space, tab, CR and LF separate tokens. Other Unicode whitespace is **not** a token boundary:
`a\u00A0b` is one identifier. (The accessors trim the full Swift `whitespacesAndNewlines` set from
*values*, which is a model-level convenience, not a lexical rule.)

`;` is an explicit command separator. A newline is an implicit separator under the rule above: it is
emitted as a separator only when the lexer's block depth is zero or the innermost open brace is a
command block.

### Comments

```tenn
a 1  // line comment, to the end of the line
/* block comment,
   no nesting */ b 2
```

Comments produce no tokens. A `/` that starts neither `//` nor `/*` is consumed as a separator, so
`a/b` lexes as two identifiers `a` and `b`.

### Identifiers and numbers

A bare word is a maximal run of characters that are not whitespace, `{`, `}`, `;`, `"`, `'`, `%`, `$`,
`@` or `/`. `#` is an ordinary word character; TENN has no `#` comments. Dots are allowed inside a
word (`my.lib.button`), and non-ASCII letters are allowed (`имя`, `😈`).

A word is classified as a number only if it consists of an optional leading `-`, decimal digits and at
most one interior `.`:

| Text      | Token kind | Note                                                        |
|-----------|------------|-------------------------------------------------------------|
| `42`, `-7`| `intLit`   |                                                             |
| `1.5`     | `floatLit` |                                                             |
| `.5`      | `symbol`   | a dot before the first digit disqualifies the word          |
| `-`       | `intLit`   | the lone `-` passes the classifier (quirk kept for parity)  |
| `+5`      | `symbol`   | `+` is not accepted as a sign                               |
| `1e5`     | `symbol`   | exponent forms are not number tokens                        |
| `1.2.3`   | `symbol`   | a second dot disqualifies the word                          |

Exponent forms still read as floats through the accessor layer (`GetFloat` accepts
`[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?` on a symbol's text). The lexer keeps them symbolic so that a
consumer can decide.

### Strings

Both `'...'` and `"..."` are string literals. A backslash escapes only the delimiter that opened the
literal: `"a\"b"` and `'a\'b'`. Any other backslash is literal text (`"a\nb"` contains a backslash and
an `n`). Strings may span lines. An unterminated string is an error and the rest of the input becomes
a string token with the error attached.

The printer always emits double quotes and escapes only `"`; a single-quoted literal therefore prints
as `"..."`, and its text is preserved (`'a\'b'` → `"a'b"`).

### Expressions, markdown and images

| Form        | Token kind        | Ends at                                       |
|-------------|-------------------|-----------------------------------------------|
| `$( ... )`  | `expression`      | the matching `)`, counting nested `(` / `)`   |
| `${ ... }`  | `expressionBlock` | the matching `}`, counting nested `{` / `}`   |
| `%{ ... }`  | `markdownLit`     | the matching `}`, counting nested `{` / `}`   |
| `@( ... )`  | `imageData`       | the first `)`                                 |

The content is opaque to the parser: an expression is a string that a consumer evaluates (Tenniarb
evaluates it through goja), markdown is a document body, an image is base64 data.

An **empty** literal (`$()`, `${}`, `%{}`, `@()`) produces no token at all: the construct disappears
from the tree. This is a property of the current implementations and is pinned by the conformance
suite.

## 3. The nine value kinds

An argument is one of nine values. The AST kind names are the Swift case names used by all three
implementations.

| Kind              | Syntax        | Stored text                                  |
|-------------------|---------------|----------------------------------------------|
| `Ident`           | `color red`   | the bare word (`red`)                        |
| `IntLit`          | `pos 10 20`   | the digits, sign included                    |
| `FloatLit`        | `width 1.5`   | the digits                                   |
| `StringLit`       | `"A b"`       | the content between the delimiters           |
| `CharLit`         | —             | modeled for parity; the lexer emits no such token |
| `Expression`      | `$(1 + 2)`    | the raw text inside `$( )`                   |
| `ExpressionBlock` | `${ var a = 1; a }` | the raw text inside `${ }`             |
| `MarkdownLit`     | `%{**bold**}` | the raw text inside `%{ }`                   |
| `Image`           | `@(AAAA)`     | the raw base64 text                          |

`Statements` is not a value: it is the root container of a parsed file. The structural kinds are
`Statements` (file root), `Command` (name plus arguments) and `BlockExpr` (a braced body). A block is
an argument, but not a value.

## 4. AST

The tree mirrors the grammar exactly; there is no semantic layer.

- **Statements** — root. `children` are commands.
- **Command** — a name and its arguments. The name is the first child, an `Ident`. Remaining children
  are values or `BlockExpr` nodes, in source order. A command records the byte span from its first
  token to the last (`End` in Go, `end` elsewhere), which editors use for source mapping.
- **BlockExpr** — the body of `{ ... }`. Its `token` is the `{` token; `children` are commands.
- **Value node** — one of the nine kinds, with a token carrying the literal text and its position.

Blocks are looked up by name inside `BlockExpr` nodes: the node keeps a map from the first child's
identifier to the command, so `block.getNamedElement("pos")` is O(1). The map is not refreshed by
`RemoveNamed` (kept for parity with the original Swift code).

Positions: every token carries `pos` (byte/code-unit offset of the start of its source span), `size`
(the span length), `line` and `col`. For literals the span is the **content** between the delimiters;
for a block it starts at `{`. `line`/`col` are the lexer's counters at the moment the token is
recorded, which for a multi-line literal is the position after the literal.

## 5. Printing

`ToStr` / `toStr` / `toStr` render a tree back to text:

- four spaces per nesting level; only `Command` nodes are indented;
- children of a `Command` are separated by one space, children of `Statements` or `BlockExpr` by a
  newline;
- a block prints as `{\n`, its commands, then `\n` + indent + `}`, or as `{\n` + indent + `}` when
  empty;
- strings print double-quoted with `"` escaped; numbers and identifiers print their literal;
- `$(...)`, `${...}`, `%{...}`, `@(...)` print with their original delimiters.

The `clean` mode (`ToStr(tree, 0, true)`) prints string literals unquoted. It exists for comparing
against persisted values and is not a valid TENN document.

Printing is not byte-identical to arbitrary input: comments and the choice of `;` versus newline are
lost, single-quoted strings become double-quoted, and indentation is normalized. For every tree that
parses without errors, printing and re-parsing yields an equivalent tree; `conformance/` pins the
exact canonical text.

## 6. Lineage

TENN borrows its shape from the **Tool Command Language** (Tcl):

- the same `name argument... { body }` form as a Tcl command;
- the body in braces is a **word**, not a computation: Tcl passes `{ ... }` through unevaluated, and so
  does TENN;
- where Tcl substitutes with `[...]` (and `$var`), TENN has `${ ... }` and `$( ... )` holding an
  expression that the consumer evaluates (Tenniarb uses goja to run JavaScript).

What TENN deliberately does **not** take from Tcl:

- no `[ ]` command substitution;
- no variables and no runtime scopes — nothing is bound while parsing;
- no "a command returns a value": a command is a node in a tree, not a call.

TENN is declarative data with embedded expressions, not a scripting language. That is what keeps one
parser sufficient for diagrams, documents, configuration and tool calls, and what makes a `.tenn` file
safe to read without executing it.

## 7. Errors

Parsing reports errors instead of throwing, and error recovery keeps the partial tree:

| Code | Meaning |
|------|---------|
| `invalidCommandStart` | a command does not start with an identifier |
| `unexpectedToken` | a required token is missing (e.g. `{`) |
| `wrongBlockTerminator` | a block or expression is not closed |
| `EndOfFileDuringStringRead` | a string is not closed |
| `parseError` | no more tokens while parsing a block |
| `unexpectedInput` | empty input in a context that requires a token |

The tree is still returned; consumers decide whether to reject the file. `conformance/` includes
error cases with their exact codes.

## 8. Profiles

The core says nothing about what commands mean. A profile defines a vocabulary and its semantics on
top of the same grammar:

- [`profiles/scene.md`](profiles/scene.md) — the Tenniarb scene profile: `element`, `item`, `link`,
  `use`, layout and styles. It is the profile the reference application implements.
