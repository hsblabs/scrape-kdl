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
export type ScalarValue = string | number | boolean | null;

export interface Value {
  readonly kind: ValueKind;
  readonly raw: string;
  readonly value: ScalarValue;
  readonly integerValue?: bigint;
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
  readonly code: "E_KDL_SYNTAX" | "E_TYPE_ANNOTATION_UNSUPPORTED";
  readonly severity: "error";
  readonly message: string;
  readonly span: Span;
  readonly path?: "";
}

export interface Document {
  readonly path: string;
  readonly nodes: readonly Node[];
  readonly span: Span;
}

type TokenKind =
  | "invalid"
  | "eof"
  | "newline"
  | "semicolon"
  | "slashdash"
  | "lbrace"
  | "rbrace"
  | "equals"
  | "lparen"
  | "rparen"
  | "identifier"
  | "string"
  | "int"
  | "float"
  | "bool"
  | "null";

interface Token {
  readonly kind: TokenKind;
  readonly text: string;
  readonly value?: ScalarValue;
  readonly integerValue?: bigint;
  readonly span: Span;
}

const INT64_MIN = -(1n << 63n);
const INT64_MAX = (1n << 63n) - 1n;

class Lexer {
  readonly diagnostics: ParseDiagnostic[] = [];
  #index = 0;
  #offset = 0;
  #line = 1;
  #column = 1;

  constructor(
    readonly path: string,
    readonly source: string,
  ) {}

  next(): Token {
    for (;;) {
      if (this.#eof()) return this.#token("eof", "", this.#position());
      const start = this.#position();
      const character = this.#peek();
      switch (character) {
        case " ":
        case "\t":
        case "\r":
          this.#advance();
          continue;
        case "\n":
          this.#advance();
          return this.#token("newline", "\n", start);
        case ";":
          return this.#punctuation("semicolon", start);
        case "{":
          return this.#punctuation("lbrace", start);
        case "}":
          return this.#punctuation("rbrace", start);
        case "=":
          return this.#punctuation("equals", start);
        case "(":
          return this.#punctuation("lparen", start);
        case ")":
          return this.#punctuation("rparen", start);
        case "/":
          if (this.#peek(1) === "/") {
            this.#skipLineComment();
            continue;
          }
          if (this.#peek(1) === "*") {
            this.#skipBlockComment(start);
            continue;
          }
          if (this.#peek(1) === "-") {
            this.#advance();
            this.#advance();
            return this.#token("slashdash", "/-", start);
          }
          break;
        case '"':
          return this.#scanQuotedString(start);
        case "#":
          return this.#scanHashToken(start);
      }
      if (isNumberStart(character, this.#peek(1))) return this.#scanNumber(start);
      if (isIdentifierStart(character)) return this.#scanIdentifier(start);
      const bad = this.#advance();
      const span = this.#span(start);
      this.#diagnostic("E_KDL_SYNTAX", `unexpected character ${quoteRune(bad)}`, span);
      return { kind: "invalid", text: bad, span };
    }
  }

  #punctuation(kind: TokenKind, start: Position): Token {
    const text = this.#advance();
    return this.#token(kind, text, start);
  }

  #skipLineComment(): void {
    while (!this.#eof() && this.#peek() !== "\n") this.#advance();
  }

  #skipBlockComment(start: Position): void {
    this.#advance();
    this.#advance();
    let depth = 1;
    while (!this.#eof()) {
      if (this.#peek() === "/" && this.#peek(1) === "*") {
        this.#advance();
        this.#advance();
        depth++;
        continue;
      }
      if (this.#peek() === "*" && this.#peek(1) === "/") {
        this.#advance();
        this.#advance();
        depth--;
        if (depth === 0) return;
        continue;
      }
      this.#advance();
    }
    this.#diagnostic("E_KDL_SYNTAX", "unterminated block comment", this.#span(start));
  }

  #scanIdentifier(start: Position): Token {
    const begin = this.#index;
    while (!this.#eof() && isIdentifierContinue(this.#peek())) this.#advance();
    const text = this.source.slice(begin, this.#index);
    return this.#token("identifier", text, start, text);
  }

  #scanQuotedString(start: Position): Token {
    const begin = this.#index;
    this.#advance();
    let value = "";
    while (!this.#eof()) {
      const character = this.#advance();
      if (character === '"') return this.#token("string", this.source.slice(begin, this.#index), start, value);
      if (character === "\\") {
        if (this.#eof()) break;
        const escaped = this.#advance();
        const escapes: Readonly<Record<string, string>> = {
          n: "\n",
          r: "\r",
          t: "\t",
          b: "\b",
          f: "\f",
          "\\": "\\",
          '"': '"',
        };
        const replacement = escapes[escaped];
        if (replacement !== undefined) {
          value += replacement;
          continue;
        }
        if (escaped === "u") {
          if (this.#peek() !== "{") {
            const span = this.#span(start);
            this.#diagnostic("E_KDL_SYNTAX", "KDL 2 Unicode escapes must use \\u{...}", span);
            return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
          }
          this.#advance();
          const hexStart = this.#index;
          while (!this.#eof() && this.#peek() !== "}") this.#advance();
          if (this.#eof()) {
            const span = this.#span(start);
            this.#diagnostic("E_KDL_SYNTAX", "unterminated Unicode escape", span);
            return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
          }
          const hexadecimal = this.source.slice(hexStart, this.#index);
          this.#advance();
          const codePoint = /^[0-9a-fA-F]+$/u.test(hexadecimal) ? Number.parseInt(hexadecimal, 16) : Number.NaN;
          if (
            !Number.isInteger(codePoint) ||
            codePoint < 0 ||
            codePoint > 0x10ffff ||
            (codePoint >= 0xd800 && codePoint <= 0xdfff)
          ) {
            const span = this.#span(start);
            this.#diagnostic("E_KDL_SYNTAX", "invalid Unicode escape", span);
            return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
          }
          value += String.fromCodePoint(codePoint);
          continue;
        }
        const span = this.#span(start);
        this.#diagnostic("E_KDL_SYNTAX", `invalid string escape \\${escaped}`, span);
        return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
      }
      if (character === "\n" || character === "\r") {
        const span = this.#span(start);
        this.#diagnostic("E_KDL_SYNTAX", "newline in quoted string", span);
        return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
      }
      value += character;
    }
    const span = this.#span(start);
    this.#diagnostic("E_KDL_SYNTAX", "unterminated quoted string", span);
    return { kind: "invalid", text: this.source.slice(begin, this.#index), span };
  }

  #scanHashToken(start: Position): Token {
    const begin = this.#index;
    for (const [raw, kind, value] of [
      ["#true", "bool", true],
      ["#false", "bool", false],
      ["#null", "null", null],
    ] as const) {
      if (this.source.startsWith(raw, this.#index) && isBoundary(this.#peek(raw.length))) {
        for (let index = 0; index < raw.length; index++) this.#advance();
        return this.#token(kind, raw, start, value);
      }
    }
    let hashes = 0;
    while (this.#peek(hashes) === "#") hashes++;
    if (this.#peek(hashes) !== '"') {
      this.#advance();
      const span = this.#span(start);
      this.#diagnostic("E_KDL_SYNTAX", "invalid hash-prefixed token", span);
      return { kind: "invalid", text: "#", span };
    }
    for (let index = 0; index < hashes; index++) this.#advance();
    const quoteCount = this.#peek() === '"' && this.#peek(1) === '"' && this.#peek(2) === '"' ? 3 : 1;
    for (let index = 0; index < quoteCount; index++) this.#advance();
    const contentStart = this.#index;
    while (!this.#eof()) {
      const quoteMatches =
        quoteCount === 3
          ? this.#peek() === '"' && this.#peek(1) === '"' && this.#peek(2) === '"'
          : this.#peek() === '"';
      if (quoteMatches && this.#hashesMatch(quoteCount, hashes)) {
        let content = this.source.slice(contentStart, this.#index);
        for (let index = 0; index < quoteCount + hashes; index++) this.#advance();
        if (quoteCount === 3) content = normalizeRawMultiline(content);
        return this.#token("string", this.source.slice(begin, this.#index), start, content);
      }
      this.#advance();
    }
    const span = this.#span(start);
    this.#diagnostic("E_KDL_SYNTAX", "unterminated raw string", span);
    return { kind: "invalid", text: "", span };
  }

  #hashesMatch(quoteCount: number, hashes: number): boolean {
    for (let index = 0; index < hashes; index++) if (this.#peek(quoteCount + index) !== "#") return false;
    return true;
  }

  #scanNumber(start: Position): Token {
    const begin = this.#index;
    if (this.#peek() === "+" || this.#peek() === "-") this.#advance();
    if (this.#peek() === "0") {
      const prefix = this.#peek(1).toLowerCase();
      const base = prefix === "x" ? 16 : prefix === "o" ? 8 : prefix === "b" ? 2 : 0;
      if (base !== 0) {
        this.#advance();
        this.#advance();
        let digits = 0;
        while (isBaseDigit(this.#peek(), base) || this.#peek() === "_") {
          if (this.#peek() !== "_") digits++;
          this.#advance();
        }
        const raw = this.source.slice(begin, this.#index);
        if (digits === 0) return this.#invalidNumber("integer base prefix must be followed by a digit", raw, start);
        const value = parseInteger(raw, base);
        if (value === undefined) return this.#invalidNumber("invalid integer literal", raw, start);
        return this.#integerToken(raw, start, value);
      }
    }
    while (isDigit(this.#peek()) || this.#peek() === "_") this.#advance();
    let kind: "int" | "float" = "int";
    if (this.#peek() === ".") {
      kind = "float";
      this.#advance();
      while (isDigit(this.#peek()) || this.#peek() === "_") this.#advance();
    }
    if (this.#peek().toLowerCase() === "e") {
      kind = "float";
      this.#advance();
      if (this.#peek() === "+" || this.#peek() === "-") this.#advance();
      while (isDigit(this.#peek()) || this.#peek() === "_") this.#advance();
    }
    const raw = this.source.slice(begin, this.#index);
    if (kind === "int") {
      const value = parseInteger(raw, 10);
      if (value === undefined) return this.#invalidNumber("invalid integer literal", raw, start);
      return this.#integerToken(raw, start, value);
    }
    const value = Number(raw.replaceAll("_", ""));
    if (!Number.isFinite(value)) return this.#invalidNumber("invalid float literal", raw, start);
    return this.#token(kind, raw, start, value);
  }

  #invalidNumber(message: string, raw: string, start: Position): Token {
    const span = this.#span(start);
    this.#diagnostic("E_KDL_SYNTAX", message, span);
    return { kind: "invalid", text: raw, span };
  }

  #integerToken(raw: string, start: Position, value: bigint): Token {
    return { kind: "int", text: raw, value: Number(value), integerValue: value, span: this.#span(start) };
  }

  #eof(): boolean {
    return this.#index >= this.source.length;
  }
  #peek(distance = 0): string {
    return this.source[this.#index + distance] ?? "";
  }

  #advance(): string {
    if (this.#eof()) return "";
    const codePoint = this.source.codePointAt(this.#index)!;
    const character = String.fromCodePoint(codePoint);
    this.#index += character.length;
    this.#offset += codePoint <= 0x7f ? 1 : codePoint <= 0x7ff ? 2 : codePoint <= 0xffff ? 3 : 4;
    if (character === "\n") {
      this.#line++;
      this.#column = 1;
    } else this.#column++;
    return character;
  }

  #position(): Position {
    return { offset: this.#offset, line: this.#line, column: this.#column };
  }
  #span(start: Position): Span {
    return { file: this.path, start, end: this.#position() };
  }

  #token(kind: TokenKind, text: string, start: Position, value?: ScalarValue): Token {
    const token: Token = { kind, text, span: this.#span(start) };
    return value === undefined ? token : { ...token, value };
  }

  #diagnostic(code: ParseDiagnostic["code"], message: string, span: Span): void {
    this.diagnostics.push({ code, severity: "error", message, span });
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
      if (this.#current.kind === "rbrace") {
        this.#error("E_KDL_SYNTAX", "unexpected closing brace", this.#current.span);
        this.#advance();
        continue;
      }
      if (this.#current.kind === "slashdash") {
        this.#skipSlashdashedNode();
        continue;
      }
      const node = this.#parseNode();
      if (node !== undefined) nodes.push(node);
    }
    this.diagnostics.push(...this.lexer.diagnostics);
    return { path: this.lexer.path, nodes, span: { file: this.lexer.path, start, end: this.#current.span.end } };
  }

  #parseNode(): Node | undefined {
    if (this.#current.kind === "lparen") {
      const span = this.#current.span;
      this.#skipTypeAnnotation();
      this.#error("E_TYPE_ANNOTATION_UNSUPPORTED", "KDL type annotations are not supported", span);
    }
    if (this.#current.kind !== "identifier" && this.#current.kind !== "string") {
      this.#error("E_KDL_SYNTAX", "expected node name", this.#current.span);
      this.#recoverNode();
      return undefined;
    }
    const start = this.#current.span;
    const name = this.#current.kind === "string" ? String(this.#current.value) : this.#current.text;
    this.#advance();
    const arguments_: Value[] = [];
    const properties: Property[] = [];
    let children: Node[] = [];
    for (;;) {
      const kind = this.#current.kind as TokenKind;
      switch (kind) {
        case "eof":
        case "newline":
        case "semicolon":
        case "rbrace": {
          const end = arguments_.at(-1)?.span ?? properties.at(-1)?.span ?? start;
          if (kind === "newline" || kind === "semicolon") this.#advance();
          return { name, arguments: arguments_, properties, children, span: mergeSpan(start, end) };
        }
        case "lbrace": {
          const open = this.#current.span;
          this.#advance();
          children = this.#parseChildren();
          if ((this.#current.kind as TokenKind) === "rbrace") {
            const close = this.#current.span;
            this.#advance();
            const following = this.#current.kind as TokenKind;
            if (following === "newline" || following === "semicolon") this.#advance();
            return { name, arguments: arguments_, properties, children, span: mergeSpan(start, close) };
          }
          this.#error("E_KDL_SYNTAX", "unterminated child block", open);
          return { name, arguments: arguments_, properties, children, span: mergeSpan(start, open) };
        }
        case "lparen": {
          const span = this.#current.span;
          this.#skipTypeAnnotation();
          this.#error("E_TYPE_ANNOTATION_UNSUPPORTED", "KDL type annotations are not supported", span);
          break;
        }
        case "slashdash":
          this.#skipSlashdashedComponent();
          break;
        default: {
          if (kind === "identifier" && this.#lookahead.kind === "equals") {
            const propertyStart = this.#current.span;
            const propertyName = this.#current.text;
            this.#advance();
            this.#advance();
            const value = this.#parseValue();
            if (value !== undefined)
              properties.push({ name: propertyName, value, span: mergeSpan(propertyStart, value.span) });
            break;
          }
          const value = this.#parseValue();
          if (value !== undefined) arguments_.push(value);
          else {
            this.#error(
              "E_KDL_SYNTAX",
              `unexpected token ${JSON.stringify(this.#current.text)} in node`,
              this.#current.span,
            );
            this.#advance();
          }
        }
      }
    }
  }

  #parseChildren(): Node[] {
    const nodes: Node[] = [];
    while (this.#current.kind !== "eof" && this.#current.kind !== "rbrace") {
      if (this.#current.kind === "newline" || this.#current.kind === "semicolon") {
        this.#advance();
        continue;
      }
      if (this.#current.kind === "slashdash") {
        this.#skipSlashdashedNode();
        continue;
      }
      const node = this.#parseNode();
      if (node !== undefined) nodes.push(node);
    }
    return nodes;
  }

  #skipSlashdashedNode(): void {
    const marker = this.#current.span;
    this.#advance();
    this.#skipSlashdashLineSpace();
    if (this.#current.kind === "slashdash") {
      this.#error("E_KDL_SYNTAX", "a slashdash cannot directly suppress another slashdash", this.#current.span);
      this.#advance();
      return;
    }
    if (this.#current.kind === "lparen") {
      this.#skipTypeAnnotation();
      this.#skipSlashdashLineSpace();
    }
    if (this.#current.kind !== "identifier" && this.#current.kind !== "string") {
      this.#error("E_KDL_SYNTAX", "slashdash before a node must be followed by a node", marker);
      return;
    }
    this.#parseNode();
  }

  #skipSlashdashedComponent(): void {
    const marker = this.#current.span;
    this.#advance();
    this.#skipSlashdashLineSpace();
    if (this.#current.kind === "slashdash") {
      this.#error("E_KDL_SYNTAX", "a slashdash cannot directly suppress another slashdash", this.#current.span);
      this.#advance();
      return;
    }
    if (this.#current.kind === "lparen") {
      this.#skipTypeAnnotation();
      this.#skipSlashdashLineSpace();
    }
    if (this.#current.kind === "lbrace") {
      this.#skipChildBlock();
      return;
    }
    if ((this.#current.kind === "identifier" || this.#current.kind === "string") && this.#lookahead.kind === "equals") {
      this.#advance();
      this.#advance();
      if ((this.#current.kind as TokenKind) === "lparen") this.#skipTypeAnnotation();
      if (this.#parseValue() === undefined)
        this.#error("E_KDL_SYNTAX", "slashdashed property is missing a value", marker);
      return;
    }
    if (this.#parseValue() === undefined) {
      this.#error(
        "E_KDL_SYNTAX",
        "slashdash inside a node must suppress an argument, property, or children block",
        marker,
      );
    }
  }

  #skipSlashdashLineSpace(): void {
    while (this.#current.kind === "newline") this.#advance();
  }

  #skipChildBlock(): void {
    const open = this.#current.span;
    this.#advance();
    this.#parseChildren();
    if (this.#current.kind !== "rbrace") {
      this.#error("E_KDL_SYNTAX", "unterminated slashdashed child block", open);
      return;
    }
    this.#advance();
  }

  #parseValue(): Value | undefined {
    const token = this.#current;
    if (!(["string", "int", "float", "bool", "null"] as const).includes(token.kind as ValueKind)) return undefined;
    this.#advance();
    const value: Value = {
      kind: token.kind as ValueKind,
      raw: token.text,
      value: token.value ?? null,
      span: token.span,
    };
    return token.integerValue === undefined ? value : { ...value, integerValue: token.integerValue };
  }

  #skipTypeAnnotation(): void {
    let depth = 0;
    while (this.#current.kind !== "eof") {
      if (this.#current.kind === "lparen") depth++;
      if (this.#current.kind === "rparen") {
        depth--;
        this.#advance();
        if (depth <= 0) return;
        continue;
      }
      this.#advance();
    }
  }

  #recoverNode(): void {
    while (!["eof", "newline", "semicolon", "rbrace"].includes(this.#current.kind)) this.#advance();
    if (this.#current.kind === "newline" || this.#current.kind === "semicolon") this.#advance();
  }

  #advance(): void {
    this.#current = this.#lookahead;
    this.#lookahead = this.lexer.next();
  }

  #error(code: ParseDiagnostic["code"], message: string, span: Span): void {
    this.diagnostics.push({ code, severity: "error", message, span });
  }
}

function mergeSpan(start: Span, end: Span): Span {
  return { file: start.file, start: start.start, end: end.end };
}

function normalizeRawMultiline(value: string): string {
  if (value.startsWith("\r\n")) value = value.slice(2);
  else if (value.startsWith("\n")) value = value.slice(1);
  if (value.endsWith("\r\n")) value = value.slice(0, -2);
  else if (value.endsWith("\n")) value = value.slice(0, -1);
  return value;
}

function parseInteger(raw: string, base: number): bigint | undefined {
  const clean = raw.replaceAll("_", "");
  const negative = clean.startsWith("-");
  const unsigned = clean.startsWith("-") || clean.startsWith("+") ? clean.slice(1) : clean;
  const digits = base === 10 ? unsigned : unsigned.slice(2);
  try {
    let value = BigInt(base === 10 ? digits : `${base === 16 ? "0x" : base === 8 ? "0o" : "0b"}${digits}`);
    if (negative) value = -value;
    if (value < INT64_MIN || value > INT64_MAX) return undefined;
    return value;
  } catch {
    return undefined;
  }
}

function isBaseDigit(character: string, base: number): boolean {
  if (base === 2) return character === "0" || character === "1";
  if (base === 8) return character >= "0" && character <= "7";
  return isDigit(character) || (character >= "a" && character <= "f") || (character >= "A" && character <= "F");
}

function isIdentifierStart(character: string): boolean {
  return (
    character === "_" ||
    character === "-" ||
    (character >= "A" && character <= "Z") ||
    (character >= "a" && character <= "z")
  );
}

function isIdentifierContinue(character: string): boolean {
  return isIdentifierStart(character) || isDigit(character) || character === "." || character === "/";
}

function isDigit(character: string): boolean {
  return character >= "0" && character <= "9";
}
function isNumberStart(character: string, next: string): boolean {
  return isDigit(character) || ((character === "+" || character === "-") && isDigit(next));
}
function isBoundary(character: string): boolean {
  return character === "" || [" ", "\t", "\r", "\n", ";", "}", "{"].includes(character);
}

function quoteRune(character: string): string {
  if (character === "'") return "'\\''";
  if (character === "\\") return "'\\\\'";
  if (character === "\n") return "'\\n'";
  if (character === "\r") return "'\\r'";
  if (character === "\t") return "'\\t'";
  const codePoint = character.codePointAt(0)!;
  if (codePoint < 0x20 || codePoint === 0x7f) return `'\\x${codePoint.toString(16).padStart(2, "0")}'`;
  if (/\p{C}/u.test(character)) {
    const width = codePoint <= 0xffff ? 4 : 8;
    return `'\\${codePoint <= 0xffff ? "u" : "U"}${codePoint.toString(16).padStart(width, "0")}'`;
  }
  return `'${character}'`;
}

export function parse(
  path: string,
  source: string,
): { readonly document: Document; readonly diagnostics: readonly ParseDiagnostic[] } {
  const parser = new Parser(new Lexer(path, source));
  const document = parser.parse();
  const diagnostics = [...parser.diagnostics].sort(
    (left, right) =>
      compareCodePoints(left.span.file, right.span.file) ||
      left.span.start.offset - right.span.start.offset ||
      compareCodePoints(left.code, right.code),
  );
  return { document, diagnostics };
}

function compareCodePoints(left: string, right: string): number {
  const leftPoints = Array.from(left, (character) => character.codePointAt(0)!);
  const rightPoints = Array.from(right, (character) => character.codePointAt(0)!);
  const length = Math.min(leftPoints.length, rightPoints.length);
  for (let index = 0; index < length; index++) {
    if (leftPoints[index] !== rightPoints[index]) return leftPoints[index]! - rightPoints[index]!;
  }
  return leftPoints.length - rightPoints.length;
}
