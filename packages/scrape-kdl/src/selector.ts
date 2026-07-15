export type Combinator = "none" | "descendant" | "child" | "adjacent" | "sibling";

export interface Selector { readonly groups: readonly ComplexSelector[]; }
export interface ComplexSelector { readonly parts: readonly SelectorPart[]; }
export interface SelectorPart { readonly combinator: Combinator; readonly compound: CompoundSelector; }
export interface CompoundSelector {
  readonly typeName?: string;
  readonly universal: boolean;
  readonly id?: string;
  readonly classes: readonly string[];
  readonly attributes: readonly AttributeSelector[];
  readonly pseudos: readonly PseudoSelector[];
}
export interface AttributeSelector { readonly name: string; readonly operator: string; readonly value: string; }
export interface PseudoSelector { readonly name: string; readonly nth?: NthExpression; readonly negation?: CompoundSelector; }
export interface NthExpression { readonly a: number; readonly b: number; }

export function parseSelector(source: string): Selector {
  const parser = new SelectorParser(source);
  const selector = parser.parseSelectorList();
  parser.skipWhitespace();
  if (!parser.eof()) parser.fail("unexpected token");
  return selector;
}

class SelectorParser {
  index = 0;

  constructor(readonly source: string) {}

  parseSelectorList(): Selector {
    const groups: ComplexSelector[] = [];
    for (;;) {
      this.skipWhitespace();
      groups.push(this.parseComplex());
      this.skipWhitespace();
      if (this.eof() || this.peek() !== ",") break;
      this.index++;
      this.skipWhitespace();
      if (this.eof()) this.fail("selector list cannot end with comma");
    }
    if (groups.length === 0) this.fail("empty selector");
    return { groups };
  }

  parseComplex(): ComplexSelector {
    const parts: SelectorPart[] = [{ combinator: "none", compound: this.parseCompound() }];
    for (;;) {
      const hadWhitespace = this.skipWhitespace();
      if (this.eof() || this.peek() === "," || this.peek() === ")") break;
      let combinator: Combinator;
      if (this.peek() === ">") { combinator = "child"; this.index++; this.skipWhitespace(); }
      else if (this.peek() === "+") { combinator = "adjacent"; this.index++; this.skipWhitespace(); }
      else if (this.peek() === "~") { combinator = "sibling"; this.index++; this.skipWhitespace(); }
      else if (hadWhitespace) combinator = "descendant";
      else this.fail("expected combinator");
      parts.push({ combinator, compound: this.parseCompound() });
    }
    return { parts };
  }

  parseCompound(): CompoundSelector {
    let typeName: string | undefined;
    let universal = false;
    let id: string | undefined;
    const classes: string[] = [];
    const attributes: AttributeSelector[] = [];
    const pseudos: PseudoSelector[] = [];
    let consumed = false;
    if (!this.eof() && this.peek() === "*") { universal = true; this.index++; consumed = true; }
    else if (!this.eof() && isIdentifierStart(this.peek())) { typeName = this.parseIdentifier().toLowerCase(); consumed = true; }
    while (!this.eof()) {
      if (this.peek() === "#") {
        this.index++;
        const value = this.parseIdentifier();
        if (id !== undefined) this.fail("multiple ID selectors are unsupported");
        id = value; consumed = true;
      } else if (this.peek() === ".") {
        this.index++; classes.push(this.parseIdentifier()); consumed = true;
      } else if (this.peek() === "[") {
        attributes.push(this.parseAttribute()); consumed = true;
      } else if (this.peek() === ":") {
        pseudos.push(this.parsePseudo()); consumed = true;
      } else break;
    }
    if (!consumed) this.fail("expected compound selector");
    const result: CompoundSelector = { universal, classes, attributes, pseudos };
    return { ...result, ...(typeName === undefined ? {} : { typeName }), ...(id === undefined ? {} : { id }) };
  }

  parseAttribute(): AttributeSelector {
    this.index++;
    this.skipWhitespace();
    const name = this.parseIdentifier().toLowerCase();
    this.skipWhitespace();
    if (this.eof()) this.fail("unterminated attribute selector");
    if (this.peek() === "]") { this.index++; return { name, operator: "", value: "" }; }
    const operators = ["~=", "|=", "^=", "$=", "*=", "="];
    const operator = operators.find((candidate) => this.source.startsWith(candidate, this.index));
    if (operator === undefined) this.fail("invalid attribute operator");
    this.index += operator.length;
    this.skipWhitespace();
    const value = this.parseAttributeValue();
    const separated = this.skipWhitespace();
    if (separated && !this.eof() && /[iIsS]/u.test(this.peek())) {
      const flag = this.peek(); this.index++; this.skipWhitespace();
      if (!this.eof() && this.peek() === "]") this.fail(`attribute selector case-sensitivity flag ${JSON.stringify(flag)} is unsupported`);
    }
    if (this.eof() || this.peek() !== "]") this.fail("unterminated attribute selector");
    this.index++;
    return { name, operator, value };
  }

  parseAttributeValue(): string {
    if (this.eof()) this.fail("missing attribute value");
    const quote = this.peek();
    if (quote === "'" || quote === "\"") {
      this.index++;
      let output = "";
      while (!this.eof() && this.peek() !== quote) {
        if (this.peek() === "\\") { this.index++; if (this.eof()) this.fail("unterminated escape"); }
        output += this.peek(); this.index++;
      }
      if (this.eof()) this.fail("unterminated quoted attribute value");
      this.index++;
      return output;
    }
    const start = this.index;
    while (!this.eof() && !/\s/u.test(this.peek()) && this.peek() !== "]") this.index++;
    if (start === this.index) this.fail("missing attribute value");
    return this.source.slice(start, this.index);
  }

  parsePseudo(): PseudoSelector {
    this.index++;
    if (!this.eof() && this.peek() === ":") this.fail("pseudo-elements are unsupported");
    const name = this.parseIdentifier().toLowerCase();
    if (["first-child", "last-child", "only-child", "empty", "first-of-type", "last-of-type", "only-of-type"].includes(name)) return { name };
    if (this.eof() || this.peek() !== "(") this.fail(`unsupported pseudo-class ${JSON.stringify(name)}`);
    this.index++;
    const start = this.index;
    let depth = 1;
    let quote = "";
    while (!this.eof() && depth > 0) {
      const character = this.peek();
      if (quote !== "") {
        if (character === "\\") { this.index = Math.min(this.source.length, this.index + 2); continue; }
        if (character === quote) quote = "";
        this.index++; continue;
      }
      if (character === "'" || character === "\"") { quote = character; this.index++; continue; }
      if (character === "(") depth++;
      else if (character === ")") { depth--; if (depth === 0) break; }
      this.index++;
    }
    if (this.eof() || depth !== 0) this.fail("unterminated pseudo-class");
    const argument = this.source.slice(start, this.index).trim();
    this.index++;
    if (["nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type"].includes(name)) {
      try { return { name, nth: parseNth(argument) }; }
      catch (error) { this.fail(`invalid ${name} expression: ${error instanceof Error ? error.message : String(error)}`); }
    }
    if (name === "not") {
      const nested = new SelectorParser(argument);
      let negation: CompoundSelector;
      try { negation = nested.parseCompound(); }
      catch (error) { this.fail(`invalid :not argument: ${error instanceof Error ? error.message : String(error)}`); }
      nested.skipWhitespace();
      if (!nested.eof()) this.fail(":not only accepts a compound selector");
      return { name, negation };
    }
    this.fail(`unsupported pseudo-class ${JSON.stringify(name)}`);
  }

  parseIdentifier(): string {
    if (this.eof() || !isIdentifierStart(this.peek())) this.fail("expected identifier");
    const start = this.index++;
    while (!this.eof() && isIdentifierContinue(this.peek())) this.index++;
    return this.source.slice(start, this.index);
  }

  skipWhitespace(): boolean { const start = this.index; while (!this.eof() && /\s/u.test(this.peek())) this.index++; return this.index > start; }
  eof(): boolean { return this.index >= this.source.length; }
  peek(): string { return this.source[this.index]!; }
  fail(message: string): never { throw new Error(`selector byte ${this.index}: ${message}`); }
}

function parseNth(input: string): NthExpression {
  const value = input.trim().toLowerCase().replaceAll(" ", "");
  if (value === "odd") return { a: 2, b: 1 };
  if (value === "even") return { a: 2, b: 0 };
  if (!value.includes("n")) {
    if (!/^[+-]?\d+$/u.test(value)) throw new Error(`invalid integer ${JSON.stringify(value)}`);
    return { a: 0, b: Number(value) };
  }
  const parts = value.split("n");
  if (parts.length !== 2) throw new Error("invalid An+B form");
  let a: number;
  if (parts[0] === "" || parts[0] === "+") a = 1;
  else if (parts[0] === "-") a = -1;
  else if (/^[+-]?\d+$/u.test(parts[0]!)) a = Number(parts[0]);
  else throw new Error(`invalid integer ${JSON.stringify(parts[0])}`);
  let b = 0;
  if (parts[1] !== "") {
    if (!/^[+-]?\d+$/u.test(parts[1]!)) throw new Error(`invalid integer ${JSON.stringify(parts[1])}`);
    b = Number(parts[1]);
  }
  return { a, b };
}

function isIdentifierStart(character: string): boolean { return character === "_" || character === "-" || /[a-zA-Z]/u.test(character); }
function isIdentifierContinue(character: string): boolean { return isIdentifierStart(character) || /\d/u.test(character); }

export function nthMatches(expression: NthExpression, position: number): boolean {
  if (position <= 0) return false;
  if (expression.a === 0) return position === expression.b;
  const difference = position - expression.b;
  return difference % expression.a === 0 && difference / expression.a >= 0;
}
