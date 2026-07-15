import { parse, serialize, type DefaultTreeAdapterTypes } from "parse5";
import { nthMatches, parseSelector, type AttributeSelector, type ComplexSelector, type CompoundSelector, type PseudoSelector, type Selector } from "./selector.js";

export type DocumentNode = DefaultTreeAdapterTypes.Document;
export type ElementNode = DefaultTreeAdapterTypes.Element;
type ParentNode = DefaultTreeAdapterTypes.ParentNode;
type Node = DefaultTreeAdapterTypes.Node;

export function parseHTML(source: string): DocumentNode { return parse(source); }
export function innerHTML(element: ElementNode): string { return serialize(element); }
export function attribute(element: ElementNode, name: string): string | undefined { return element.attrs.find((item) => item.name === name)?.value; }

export function textContent(node: ParentNode): string {
  let output = "";
  for (const child of node.childNodes) {
    if ("value" in child) output += child.value;
    else if (isParent(child)) output += textContent(child);
  }
  return output;
}

export function queryAll(root: ParentNode, source: string | Selector): ElementNode[] {
  const selector = typeof source === "string" ? parseSelector(source) : source;
  const output: ElementNode[] = [];
  const seen = new Set<ElementNode>();
  const walk = (parent: ParentNode): void => {
    for (const child of parent.childNodes) {
      if (!isElement(child)) continue;
      if (selector.groups.some((group) => matchesComplex(child, group, group.parts.length - 1)) && !seen.has(child)) { seen.add(child); output.push(child); }
      walk(child);
    }
  };
  walk(root);
  return output;
}

function matchesComplex(node: ElementNode | undefined, selector: ComplexSelector, index: number): boolean {
  if (index < 0 || node === undefined || !matchesCompound(node, selector.parts[index]!.compound)) return false;
  if (index === 0) return true;
  const combinator = selector.parts[index]!.combinator;
  if (combinator === "child") return matchesComplex(parentElement(node), selector, index - 1);
  if (combinator === "descendant") {
    for (let parent = parentElement(node); parent !== undefined; parent = parentElement(parent)) if (matchesComplex(parent, selector, index - 1)) return true;
    return false;
  }
  if (combinator === "adjacent") return matchesComplex(previousElementSibling(node), selector, index - 1);
  if (combinator === "sibling") {
    for (let sibling = previousElementSibling(node); sibling !== undefined; sibling = previousElementSibling(sibling)) if (matchesComplex(sibling, selector, index - 1)) return true;
  }
  return false;
}

function matchesCompound(node: ElementNode, compound: CompoundSelector): boolean {
  if (compound.typeName !== undefined && node.tagName.toLowerCase() !== compound.typeName) return false;
  if (compound.id !== undefined && attribute(node, "id") !== compound.id) return false;
  const classes = (attribute(node, "class") ?? "").split(/\s+/u).filter(Boolean);
  if (compound.classes.some((item) => !classes.includes(item))) return false;
  if (compound.attributes.some((item) => !matchesAttribute(node, item))) return false;
  return compound.pseudos.every((item) => matchesPseudo(node, item));
}

function matchesAttribute(node: ElementNode, selector: AttributeSelector): boolean {
  const actual = attribute(node, selector.name);
  if (selector.operator === "") return actual !== undefined;
  if (actual === undefined) return false;
  if (selector.operator === "=") return actual === selector.value;
  if (selector.operator === "~=") return actual.split(/\s+/u).includes(selector.value);
  if (selector.operator === "|=") return actual === selector.value || actual.startsWith(`${selector.value}-`);
  if (selector.operator === "^=") return actual.startsWith(selector.value);
  if (selector.operator === "$=") return actual.endsWith(selector.value);
  return selector.operator === "*=" && actual.includes(selector.value);
}

function matchesPseudo(node: ElementNode, pseudo: PseudoSelector): boolean {
  const siblings = elementSiblings(node);
  const position = siblings.indexOf(node) + 1;
  const reversePosition = siblings.length - siblings.indexOf(node);
  const typeSiblings = siblings.filter((item) => item.tagName.toLowerCase() === node.tagName.toLowerCase());
  const typePosition = typeSiblings.indexOf(node) + 1;
  const reverseTypePosition = typeSiblings.length - typeSiblings.indexOf(node);
  if (pseudo.name === "first-child") return position === 1;
  if (pseudo.name === "last-child") return reversePosition === 1;
  if (pseudo.name === "only-child") return siblings.length === 1;
  if (pseudo.name === "empty") return node.childNodes.every((child) => child.nodeName === "#comment" || "value" in child && child.value === "");
  if (pseudo.name === "first-of-type") return typePosition === 1;
  if (pseudo.name === "last-of-type") return reverseTypePosition === 1;
  if (pseudo.name === "only-of-type") return typeSiblings.length === 1;
  if (pseudo.name === "nth-child") return pseudo.nth !== undefined && nthMatches(pseudo.nth, position);
  if (pseudo.name === "nth-last-child") return pseudo.nth !== undefined && nthMatches(pseudo.nth, reversePosition);
  if (pseudo.name === "nth-of-type") return pseudo.nth !== undefined && nthMatches(pseudo.nth, typePosition);
  if (pseudo.name === "nth-last-of-type") return pseudo.nth !== undefined && nthMatches(pseudo.nth, reverseTypePosition);
  return pseudo.name === "not" && pseudo.negation !== undefined && !matchesCompound(node, pseudo.negation);
}

function parentElement(node: ElementNode): ElementNode | undefined { const parent = node.parentNode; return parent !== null && isElement(parent) ? parent : undefined; }
function elementSiblings(node: ElementNode): ElementNode[] { return node.parentNode?.childNodes.filter(isElement) ?? []; }
function previousElementSibling(node: ElementNode): ElementNode | undefined { const siblings = elementSiblings(node); const index = siblings.indexOf(node); return index > 0 ? siblings[index - 1] : undefined; }
function isParent(node: Node): node is ParentNode { return "childNodes" in node; }
function isElement(node: Node): node is ElementNode { return "tagName" in node; }
