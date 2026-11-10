// TENN core: the lexer, parser, node model and printer.
//
// This package is syntax only. The element model, layout, execution and persistence of the Tenniarb
// application are deliberately not here: a consumer of the format decides what commands mean.
export * from "./model.ts";
export { TennLexer, detectSymbolType } from "./lexer.ts";
export { TennParser, TennErrorContainer } from "./parser.ts";
export type { TennError, TennErrorCode } from "./parser.ts";
export { toStr, childsToStr } from "./printer.ts";

import { TennParser } from "./parser.ts";
import type { TennNode } from "./model.ts";

// parse is a shortcut for a new parser with the default lexer.
export function parse(source: string): { tree: TennNode; parser: TennParser } {
  const parser = new TennParser();
  return { tree: parser.parse(source), parser };
}
