export interface Position {
  readonly offset: number;
  readonly line: number;
  readonly column: number;
}

export interface Span {
  readonly file: string;
  readonly start: Position;
  readonly end: Position;
}

export type ValueKind = "string" | "int" | "float" | "bool" | "null";

export interface Value {
  readonly kind: ValueKind;
  readonly raw: string;
  readonly value: string | number | boolean | null;
  readonly span: Span;
}

export interface Property {
  readonly name: string;
  readonly value: Value;
  readonly span: Span;
}

export interface Node {
  readonly name: string;
  readonly arguments: readonly Value[];
  readonly properties: readonly Property[];
  readonly children: readonly Node[];
  readonly span: Span;
}

export interface ParseDiagnostic {
  readonly code: "E_KDL_SYNTAX";
  readonly severity: "error";
  readonly message: string;
  readonly span: Span;
  readonly path: "";
}

export interface Document {
  readonly nodes: readonly Node[];
  readonly span: Span;
}

type TokenKind =
  | "identifier" | "string" | "int" | "float" | "bool" | "null"
  | "lbrace" | "rbrace" | "equals" | "newline" | "semicolon" | "eof" | "invalid";

interface Token {
  readonly kind: TokenKind;
  readonly text: string;
  readonly value?: string | number | boolean | null;
  readonly span: Span;
}

class Lexer {
  readonly diagnostics: ParseDiagnostic[] = [];
  #index = 0;
  #line = 1;
  #column = 1;

  constructor(readonly path: string, readonly source: string) {}

  next(): Token {
    this.#skipHorizontalSpaceAndComments();
    const start = this.#position();
    if (this.#index >= this.source.length) return this.#token("eof", "", start);
    const character = this.source[this.#index]!;
    if (character === "\n" || character === "\r") {
      this.#consumeNewline();
      return this.#token("newline", character, start);
    }
    const punctuation: Partial<Record<string, TokenKind>> = {
      "{": "lbrace", "}": "rbrace", "=": "equals", ";": "semicolon",
    };
    const punctuationKind = punctuation[character];
    if (punctuationKind !== undefined) {
      this.#advanceCharacter();
      return this.#token(punctuationKind, character, start);
    }
    if (character === '"') return this.#quotedString(start);
    if (this.source.startsWith("#true", this.#index)) return this.#keyword("bool", "#true", true, start);
    if (this.source.startsWith("#false", this.#index)) return this.#keyword("bool", "#false", false, start);
    if (this.source.startsWith("#null", this.#index)) return this.#keyword("null", "#null", null, start);
    return this.#bareToken(start);
  }

  #skipHorizontalSpaceAndComments(): void {
    for (;;) {
      while (this.#index < this.source.length && /[\t \u00a0]/u.test(this.source[this.#index]!)) this.#advanceCharacter();
      if (this.source.startsWith("//", this.#index)) {
        while (this.#index < this.source.length && !/[\r\n]/u.test(this.source[this.#index]!)) this.#advanceCharacter();
        continue;
      }
      if (!this.source.startsWith("/*", this.#index)) return;
      const start = this.#position();
      let depth = 0;
      while (this.#index < this.source.length) {
        if (this.source.startsWith("/*", this.#index)) {
          depth++;
          this.#advanceCharacter();
          this.#advanceCharacter();
          continue;
        }
        if (this.source.startsWith("*/", this.#index)) {
          depth--;
          this.#advanceCharacter();
          this.#advanceCharacter();
          if (depth === 0) break;
          continue;
        }
        if (/[\r\n]/u.test(this.source[this.#index]!)) this.#consumeNewline();
        else this.#advanceCharacter();
      }
      if (depth !== 0) this.#diagnostic("unterminated block comment", start);
    }
  }

  #quotedString(start: Position): Token {
    const rawStart = this.#index;
    this.#advanceCharacter();
    let value = "";
    let terminated = false;
    while (this.#index < this.source.length) {
      const character = this.source[this.#index]!;
      if (character === '"') {
        this.#advanceCharacter();
        terminated = true;
        break;
      }
      if (character === "\\") {
        this.#advanceCharacter();
        if (this.#index >= this.source.length) break;
        const escaped = this.source[this.#index]!;
        const escapes: Record<string, string> = { '"': '"', "\\": "\\", "n": "\n", "r": "\r", "t": "\t", "b": "\b", "f": "\f" };
        if (escapes[escaped] === undefined) {
          this.#diagnostic(`unsupported string escape \\${escaped}`, start);
          value += escaped;
        } else {
          value += escapes[escaped];
        }
        this.#advanceCharacter();
        continue;
      }
      if (/[\r\n]/u.test(character)) {
        this.#diagnostic("newline in quoted string", start);
        this.#consumeNewline();
        continue;
      }
      value += character;
      this.#advanceCharacter();
    }
    if (!terminated) this.#diagnostic("unterminated quoted string", start);
    return this.#token("string", this.source.slice(rawStart, this.#index), start, value);
  }

  #keyword(kind: "bool" | "null", raw: string, value: boolean | null, start: Position): Token {
    for (let index = 0; index < raw.length; index++) this.#advanceCharacter();
    return this.#token(kind, raw, start, value);
  }

  #bareToken(start: Position): Token {
    const rawStart = this.#index;
    while (this.#index < this.source.length && !/[\s{}=;"()]/u.test(this.source[this.#index]!)) this.#advanceCharacter();
    if (rawStart === this.#index) {
      const invalid = this.source[this.#index]!;
      this.#advanceCharacter();
      this.#diagnostic(`unexpected character ${JSON.stringify(invalid)}`, start);
      return this.#token("invalid", invalid, start);
    }
    const raw = this.source.slice(rawStart, this.#index);
    if (/^-?(?:0|[1-9][0-9]*)$/u.test(raw)) return this.#token("int", raw, start, Number(raw));
    if (/^-?(?:0|[1-9][0-9]*)\.[0-9]+(?:[eE][+-]?[0-9]+)?$/u.test(raw) || /^-?(?:0|[1-9][0-9]*)[eE][+-]?[0-9]+$/u.test(raw)) {
      return this.#token("float", raw, start, Number(raw));
    }
    return this.#token("identifier", raw, start, raw);
  }

  #consumeNewline(): void {
    if (this.source.startsWith("\r\n", this.#index)) this.#index += 2;
    else this.#index++;
    this.#line++;
    this.#column = 1;
  }

  #advanceCharacter(): void {
    const codePoint = this.source.codePointAt(this.#index);
    if (codePoint === undefined) return;
    const character = String.fromCodePoint(codePoint);
    this.#index += character.length;
    this.#column++;
  }

  #position(): Position {
    return { offset: new TextEncoder().encode(this.source.slice(0, this.#index)).length, line: this.#line, column: this.#column };
  }

  #token(kind: TokenKind, text: string, start: Position, value?: string | number | boolean | null): Token {
    const token: Token = { kind, text, span: { file: this.path, start, end: this.#position() } };
    return value === undefined ? token : { ...token, value };
  }

  #diagnostic(message: string, start: Position): void {
    this.diagnostics.push({ code: "E_KDL_SYNTAX", severity: "error", message, path: "", span: { file: this.path, start, end: this.#position() } });
  }
}

class Parser {
  readonly diagnostics: ParseDiagnostic[] = [];
  #current: Token;
  #lookahead: Token;

  constructor(readonly lexer: Lexer) {
    this.#current = lexer.next();
    this.#lookahead = lexer.next();
  }

  parse(): Document {
    const start = this.#current.span.start;
    const nodes: Node[] = [];
    while (this.#current.kind !== "eof") {
      if (this.#current.kind === "newline" || this.#current.kind === "semicolon") {
        this.#advance();
        continue;
      }
      const node = this.#parseNode();
      if (node !== undefined) nodes.push(node);
    }
    this.diagnostics.push(...this.lexer.diagnostics);
    return { nodes, span: { file: this.lexer.path, start, end: this.#current.span.end } };
  }

  #parseNode(): Node | undefined {
    if (this.#current.kind !== "identifier" && this.#current.kind !== "string") {
      this.#error("expected node name", this.#current.span);
      this.#recoverNode();
      return undefined;
    }
    const start = this.#current.span;
    const name = String(this.#current.value ?? this.#current.text);
    this.#advance();
    const arguments_: Value[] = [];
    const properties: Property[] = [];
    let children: Node[] = [];
    for (;;) {
      const kind = this.#current.kind as TokenKind;
      if (["eof", "newline", "semicolon", "rbrace"].includes(kind)) {
        const end = arguments_.at(-1)?.span ?? properties.at(-1)?.span ?? start;
        if (kind === "newline" || kind === "semicolon") this.#advance();
        return { name, arguments: arguments_, properties, children, span: mergeSpan(start, end) };
      }
      if (kind === "lbrace") {
        const open = this.#current.span;
        this.#advance();
        children = this.#parseChildren();
        const closingKind = this.#current.kind as TokenKind;
        if (closingKind !== "rbrace") {
          this.#error("unterminated child block", open);
          return { name, arguments: arguments_, properties, children, span: mergeSpan(start, open) };
        }
        const close = this.#current.span;
        this.#advance();
        const followingKind = this.#current.kind as TokenKind;
        if (followingKind === "newline" || followingKind === "semicolon") this.#advance();
        return { name, arguments: arguments_, properties, children, span: mergeSpan(start, close) };
      }
      if (this.#current.kind === "identifier" && this.#lookahead.kind === "equals") {
        const propertyStart = this.#current.span;
        const propertyName = this.#current.text;
        this.#advance();
        this.#advance();
        const value = this.#parseValue();
        if (value !== undefined) properties.push({ name: propertyName, value, span: mergeSpan(propertyStart, value.span) });
        continue;
      }
      const value = this.#parseValue();
      if (value !== undefined) {
        arguments_.push(value);
        continue;
      }
      this.#error(`unexpected token ${JSON.stringify(this.#current.text)} in node`, this.#current.span);
      this.#advance();
    }
  }

  #parseChildren(): Node[] {
    const nodes: Node[] = [];
    while (this.#current.kind !== "eof" && this.#current.kind !== "rbrace") {
      if (this.#current.kind === "newline" || this.#current.kind === "semicolon") {
        this.#advance();
        continue;
      }
      const node = this.#parseNode();
      if (node !== undefined) nodes.push(node);
    }
    return nodes;
  }

  #parseValue(): Value | undefined {
    const token = this.#current;
    if (!["string", "int", "float", "bool", "null"].includes(token.kind)) return undefined;
    this.#advance();
    return { kind: token.kind as ValueKind, raw: token.text, value: token.value ?? null, span: token.span };
  }

  #recoverNode(): void {
    while (!["eof", "newline", "semicolon", "rbrace"].includes(this.#current.kind)) this.#advance();
    if (this.#current.kind === "newline" || this.#current.kind === "semicolon") this.#advance();
  }

  #advance(): void {
    this.#current = this.#lookahead;
    this.#lookahead = this.lexer.next();
  }

  #error(message: string, span: Span): void {
    this.diagnostics.push({ code: "E_KDL_SYNTAX", severity: "error", message, span, path: "" });
  }
}

function mergeSpan(start: Span, end: Span): Span {
  return { file: start.file, start: start.start, end: end.end };
}

export function parse(path: string, source: string): { readonly document: Document; readonly diagnostics: readonly ParseDiagnostic[] } {
  const parser = new Parser(new Lexer(path, source));
  const document = parser.parse();
  return { document, diagnostics: parser.diagnostics };
}
