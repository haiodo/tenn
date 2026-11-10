package tenn

// The conformance suite is the contract between the Go, TypeScript and Swift implementations: every
// implementation parses conformance/cases/*.tenn and must produce exactly the canonical AST in
// conformance/expected/*.json, and every error-free case must print exactly expected/*.tenn.
//
// Run with TENN_UPDATE_CONFORMANCE=1 to regenerate the expected files from this implementation. That is
// how a new case is added: write cases/<name>.tenn, regenerate, then read the diff to make sure the tree
// is what the grammar says, not just what the code happens to produce.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

const conformanceDir = "../conformance"

// canonPos is the position of the start of a token's source span: 1-based line, 0-based column in
// Unicode scalar values. Literal spans start after the opening delimiter, a block span starts at '{'.
type canonPos struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

type canonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type canonNode struct {
	Kind  string       `json:"kind"`
	Name  string       `json:"name,omitempty"`
	Value *string      `json:"value,omitempty"`
	Pos   canonPos     `json:"pos"`
	Args  *[]canonNode `json:"args,omitempty"`

	// Block is the first block argument of a Command, or the body of a BlockExpr value.
	Block *[]canonNode `json:"block,omitempty"`
	// BlockIndex is the index of Block in the Argument list, present only when it is not the last one.
	BlockIndex *int      `json:"blockIndex,omitempty"`
	BlockPos   *canonPos `json:"blockPos,omitempty"`
}

type canonFile struct {
	Kind     string       `json:"kind"`
	File     string       `json:"file"`
	Children []canonNode  `json:"children"`
	Errors   []canonError `json:"errors"`
}

func canonPosition(file, source string, pos int) canonPos {
	if pos < 0 {
		pos = 0
	}
	if pos > len(source) {
		pos = len(source)
	}
	line, lineStart := 1, 0
	for i := 0; i < pos; i++ {
		if source[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}
	return canonPos{File: file, Line: line, Col: utf8.RuneCountInString(source[lineStart:pos])}
}

func canonTokenPosition(file, source string, tok *Token) canonPos {
	if tok == nil {
		return canonPosition(file, source, 0)
	}
	return canonPosition(file, source, tok.Pos)
}

func canonCommands(n *Node, file, source string) []canonNode {
	out := []canonNode{}
	for _, c := range n.Children {
		out = append(out, canonCommand(c, file, source))
	}
	return out
}

func canonValue(n *Node, file, source string) canonNode {
	if n.Kind == KindBlockExpr {
		block := canonCommands(n, file, source)
		return canonNode{
			Kind:  "BlockExpr",
			Pos:   canonTokenPosition(file, source, n.Token),
			Block: &block,
		}
	}
	text, _ := n.GetIdentText()
	return canonNode{
		Kind:  string(n.Kind),
		Value: &text,
		Pos:   canonTokenPosition(file, source, n.Token),
	}
}

func canonCommand(n *Node, file, source string) canonNode {
	name, _ := n.GetIdent(0)
	out := canonNode{
		Kind: "Command",
		Name: name,
		Pos:  canonTokenPosition(file, source, n.Token),
	}

	args := []canonNode{}
	blockSeen := false
	blockIndex := -1
	for _, c := range n.Children[1:] {
		if c.Kind == KindBlockExpr && !blockSeen {
			blockSeen = true
			blockIndex = len(args)
			block := canonCommands(c, file, source)
			out.Block = &block
			pos := canonTokenPosition(file, source, c.Token)
			out.BlockPos = &pos
			continue
		}
		args = append(args, canonValue(c, file, source))
	}
	out.Args = &args
	if out.Block != nil && blockIndex < len(args) {
		out.BlockIndex = &blockIndex
	}
	return out
}

func buildCanonical(file, source string, tree *Node, parser *Parser) canonFile {
	out := canonFile{
		Kind:     "Statements",
		File:     file,
		Children: canonCommands(tree, file, source),
		Errors:   []canonError{},
	}
	for _, e := range parser.Errors.Errors {
		out.Errors = append(out.Errors, canonError{Code: string(e.ErrorCode), Message: e.Message})
	}
	return out
}

// TestConformance checks every case against its canonical AST and printer output.
func TestConformance(t *testing.T) {
	casesDir := filepath.Join(conformanceDir, "cases")
	expectedDir := filepath.Join(conformanceDir, "expected")
	update := os.Getenv("TENN_UPDATE_CONFORMANCE") == "1"

	paths, err := filepath.Glob(filepath.Join(casesDir, "*.tenn"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 15 {
		t.Fatalf("want at least 15 conformance cases, found %d in %s", len(paths), casesDir)
	}

	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(strings.TrimSuffix(name, ".tenn"), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			source := string(data)

			tree, parser := Parse(source)
			canon := buildCanonical("cases/"+name, source, tree, parser)
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false) // keep "<", ">" and "&" literal: the JSON is a cross-language contract
			enc.SetIndent("", "  ")
			if err := enc.Encode(canon); err != nil {
				t.Fatal(err)
			}
			gotJSON := buf.Bytes()

			jsonPath := filepath.Join(expectedDir, strings.TrimSuffix(name, ".tenn")+".json")
			if update {
				if err := os.WriteFile(jsonPath, gotJSON, 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				wantJSON, err := os.ReadFile(jsonPath)
				if err != nil {
					t.Fatalf("missing expected AST: %v", err)
				}
				if !bytes.Equal(gotJSON, wantJSON) {
					t.Fatalf("canonical AST differs from %s\n%s", jsonPath, firstLineDiff(string(wantJSON), string(gotJSON)))
				}
			}

			// Error recovery trees are not meant to print back, so only clean parses have printer output.
			tennPath := filepath.Join(expectedDir, strings.TrimSuffix(name, ".tenn")+".tenn")
			if parser.Errors.HasErrors() {
				if !update {
					if _, err := os.Stat(tennPath); err == nil {
						t.Fatalf("%s must not have a printer golden: the case does not parse", tennPath)
					}
				}
				return
			}
			gotTenn := ToStr(tree, 0, false) + "\n"
			if update {
				if err := os.WriteFile(tennPath, []byte(gotTenn), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				wantTenn, err := os.ReadFile(tennPath)
				if err != nil {
					t.Fatalf("missing printer golden: %v", err)
				}
				if gotTenn != string(wantTenn) {
					t.Fatalf("printer output differs from %s\n%s", tennPath, firstLineDiff(string(wantTenn), gotTenn))
				}
			}
		})
	}
}

func firstLineDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n  want: %q\n  got:  %q", i+1, w, g)
		}
	}
	return "outputs are equal"
}
