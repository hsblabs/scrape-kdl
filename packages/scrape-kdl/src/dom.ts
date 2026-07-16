import { parse, serialize, type DefaultTreeAdapterTypes } from "parse5";
import {
  nthMatches,
  parseSelector,
  type AttributeSelector,
  type ComplexSelector,
  type CompoundSelector,
  type PseudoSelector,
  type Selector,
} from "./selector.js";

export type DocumentNode = DefaultTreeAdapterTypes.Document;
export type ElementNode = DefaultTreeAdapterTypes.Element;
type ParentNode = DefaultTreeAdapterTypes.ParentNode;
type Node = DefaultTreeAdapterTypes.Node;

interface SiblingMetadata {
  readonly elements: readonly ElementNode[];
  readonly indexes: ReadonlyMap<ElementNode, number>;
  readonly typeIndexes: ReadonlyMap<ElementNode, number>;
  readonly typeCounts: ReadonlyMap<string, number>;
}

const siblingMetadata = new WeakMap<ParentNode, SiblingMetadata>();

export function parseHTML(source: string): DocumentNode {
  return parse(source);
}
export function innerHTML(element: ElementNode): string {
  return serialize(element);
}
export function attribute(element: ElementNode, name: string): string | undefined {
  return element.attrs.find((item) => item.name === name)?.value;
}

export function textContent(node: ParentNode): string {
  let output = "";
  for (const child of node.childNodes) {
    if ("value" in child) output += child.value;
    else if (isParent(child)) output += textContent(child);
  }
  return output;
}

export function queryAll(root: ParentNode, source: string | Selector, limit = 0): ElementNode[] {
  const selector = typeof source === "string" ? parseSelector(source) : source;
  const output: ElementNode[] = [];
  const seen = new Set<ElementNode>();
  const walk = (parent: ParentNode): boolean => {
    for (const child of parent.childNodes) {
      if (!isElement(child)) continue;
      if (selector.groups.some((group) => matchesComplex(child, group, group.parts.length - 1)) && !seen.has(child)) {
        seen.add(child);
        output.push(child);
        if (limit > 0 && output.length >= limit) return true;
      }
      if (walk(child)) return true;
    }
    return false;
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
    for (let parent = parentElement(node); parent !== undefined; parent = parentElement(parent))
      if (matchesComplex(parent, selector, index - 1)) return true;
    return false;
  }
  if (combinator === "adjacent") return matchesComplex(previousElementSibling(node), selector, index - 1);
  if (combinator === "sibling") {
    for (let sibling = previousElementSibling(node); sibling !== undefined; sibling = previousElementSibling(sibling))
      if (matchesComplex(sibling, selector, index - 1)) return true;
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
  const metadata = metadataFor(node);
  const position = metadata.indexes.get(node) ?? 0;
  const reversePosition = metadata.elements.length - position + 1;
  const typePosition = metadata.typeIndexes.get(node) ?? 0;
  const typeCount = metadata.typeCounts.get(node.tagName.toLowerCase()) ?? 0;
  const reverseTypePosition = typeCount - typePosition + 1;
  if (pseudo.name === "first-child") return position === 1;
  if (pseudo.name === "last-child") return reversePosition === 1;
  if (pseudo.name === "only-child") return metadata.elements.length === 1;
  if (pseudo.name === "empty")
    return node.childNodes.every((child) => child.nodeName === "#comment" || ("value" in child && child.value === ""));
  if (pseudo.name === "first-of-type") return typePosition === 1;
  if (pseudo.name === "last-of-type") return reverseTypePosition === 1;
  if (pseudo.name === "only-of-type") return typeCount === 1;
  if (pseudo.name === "nth-child") return pseudo.nth !== undefined && nthMatches(pseudo.nth, position);
  if (pseudo.name === "nth-last-child") return pseudo.nth !== undefined && nthMatches(pseudo.nth, reversePosition);
  if (pseudo.name === "nth-of-type") return pseudo.nth !== undefined && nthMatches(pseudo.nth, typePosition);
  if (pseudo.name === "nth-last-of-type")
    return pseudo.nth !== undefined && nthMatches(pseudo.nth, reverseTypePosition);
  return pseudo.name === "not" && pseudo.negation !== undefined && !matchesCompound(node, pseudo.negation);
}

function parentElement(node: ElementNode): ElementNode | undefined {
  const parent = node.parentNode;
  return parent !== null && isElement(parent) ? parent : undefined;
}
function previousElementSibling(node: ElementNode): ElementNode | undefined {
  const metadata = metadataFor(node);
  const index = metadata.indexes.get(node) ?? 0;
  return index > 1 ? metadata.elements[index - 2] : undefined;
}

function metadataFor(node: ElementNode): SiblingMetadata {
  const parent = node.parentNode;
  if (parent === null)
    return {
      elements: [node],
      indexes: new Map([[node, 1]]),
      typeIndexes: new Map([[node, 1]]),
      typeCounts: new Map([[node.tagName.toLowerCase(), 1]]),
    };
  const cached = siblingMetadata.get(parent);
  if (cached !== undefined) return cached;
  const elements = parent.childNodes.filter(isElement);
  const indexes = new Map<ElementNode, number>();
  const typeIndexes = new Map<ElementNode, number>();
  const typeCounts = new Map<string, number>();
  elements.forEach((element, index) => {
    indexes.set(element, index + 1);
    const type = element.tagName.toLowerCase();
    const typeIndex = (typeCounts.get(type) ?? 0) + 1;
    typeCounts.set(type, typeIndex);
    typeIndexes.set(element, typeIndex);
  });
  const metadata = { elements, indexes, typeIndexes, typeCounts };
  siblingMetadata.set(parent, metadata);
  return metadata;
}
function isParent(node: Node): node is ParentNode {
  return "childNodes" in node;
}
function isElement(node: Node): node is ElementNode {
  return "tagName" in node;
}
