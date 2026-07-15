package dom

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Combinator uint8

const (
	NoCombinator Combinator = iota
	DescendantCombinator
	ChildCombinator
	AdjacentCombinator
	GeneralSiblingCombinator
)

type Selector struct {
	Groups []ComplexSelector
}

type ComplexSelector struct {
	Parts []SelectorPart
}

type SelectorPart struct {
	Combinator Combinator
	Compound   CompoundSelector
}

type CompoundSelector struct {
	TypeName   string
	Universal  bool
	ID         string
	Classes    []string
	Attributes []AttributeSelector
	Pseudos    []PseudoSelector
}

type AttributeSelector struct {
	Name     string
	Operator string
	Value    string
}

type PseudoSelector struct {
	Name     string
	Nth      *NthExpression
	Negation *CompoundSelector
}

type NthExpression struct {
	A int
	B int
}

func ParseSelector(input string) (Selector, error) {
	parser := selectorParser{s: input}
	selector, err := parser.parseSelectorList()
	if err != nil {
		return Selector{}, err
	}
	parser.skipWhitespace()
	if !parser.eof() {
		return Selector{}, parser.errorf("unexpected token")
	}
	return selector, nil
}

func QueryAll(root *Node, selector Selector) []*Node {
	return QueryLimit(root, selector, 0)
}

// QueryLimit returns matching descendants in document order and stops after
// limit matches. A non-positive limit preserves QueryAll behavior.
func QueryLimit(root *Node, selector Selector, limit int) []*Node {
	if root == nil {
		return nil
	}
	seen := map[*Node]bool{}
	result := make([]*Node, 0)
	var walk func(*Node) bool
	walk = func(node *Node) bool {
		for _, child := range node.Children {
			if child.Type == ElementNode {
				for _, group := range selector.Groups {
					if matchesComplex(child, group, len(group.Parts)-1) {
						if !seen[child] {
							seen[child] = true
							result = append(result, child)
							if limit > 0 && len(result) >= limit {
								return true
							}
						}
						break
					}
				}
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	walk(root)
	return result
}

func matchesComplex(node *Node, selector ComplexSelector, index int) bool {
	if index < 0 || node == nil || node.Type != ElementNode {
		return false
	}
	part := selector.Parts[index]
	if !matchesCompound(node, part.Compound) {
		return false
	}
	if index == 0 {
		return true
	}
	switch part.Combinator {
	case ChildCombinator:
		parent := node.Parent
		if parent != nil && parent.Type == DocumentNode {
			return false
		}
		return matchesComplex(parent, selector, index-1)
	case DescendantCombinator:
		for parent := node.Parent; parent != nil && parent.Type != DocumentNode; parent = parent.Parent {
			if matchesComplex(parent, selector, index-1) {
				return true
			}
		}
		return false
	case AdjacentCombinator:
		return matchesComplex(node.PreviousElementSibling(), selector, index-1)
	case GeneralSiblingCombinator:
		for sibling := node.PreviousElementSibling(); sibling != nil; sibling = sibling.PreviousElementSibling() {
			if matchesComplex(sibling, selector, index-1) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func matchesCompound(node *Node, compound CompoundSelector) bool {
	if compound.TypeName != "" && !strings.EqualFold(node.Tag, compound.TypeName) {
		return false
	}
	if compound.ID != "" {
		id, ok := node.Attr("id")
		if !ok || id != compound.ID {
			return false
		}
	}
	if len(compound.Classes) > 0 {
		classValue, _ := node.Attr("class")
		classes := strings.Fields(classValue)
		for _, wanted := range compound.Classes {
			found := false
			for _, class := range classes {
				if class == wanted {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	for _, attribute := range compound.Attributes {
		actual, ok := node.Attr(attribute.Name)
		if !matchAttribute(actual, ok, attribute) {
			return false
		}
	}
	for _, pseudo := range compound.Pseudos {
		if !matchPseudo(node, pseudo) {
			return false
		}
	}
	return true
}

func matchAttribute(actual string, present bool, selector AttributeSelector) bool {
	if selector.Operator == "" {
		return present
	}
	if !present {
		return false
	}
	switch selector.Operator {
	case "=":
		return actual == selector.Value
	case "~=":
		for _, part := range strings.Fields(actual) {
			if part == selector.Value {
				return true
			}
		}
		return false
	case "|=":
		return actual == selector.Value || strings.HasPrefix(actual, selector.Value+"-")
	case "^=":
		return strings.HasPrefix(actual, selector.Value)
	case "$=":
		return strings.HasSuffix(actual, selector.Value)
	case "*=":
		return strings.Contains(actual, selector.Value)
	default:
		return false
	}
}

func matchPseudo(node *Node, pseudo PseudoSelector) bool {
	switch pseudo.Name {
	case "first-child":
		return node.elementIndex == 1
	case "last-child":
		return node.Parent != nil && node.elementIndex == node.Parent.elementCount
	case "only-child":
		return node.Parent != nil && node.Parent.elementCount == 1
	case "empty":
		return node.IsEmpty()
	case "first-of-type":
		return node.typeIndex == 1
	case "last-of-type":
		return reverseTypePosition(node) == 1
	case "only-of-type":
		return node.Parent != nil && node.Parent.typeCounts[node.Tag] == 1
	case "nth-child":
		return pseudo.Nth != nil && pseudo.Nth.matches(node.elementIndex)
	case "nth-last-child":
		return pseudo.Nth != nil && node.Parent != nil && pseudo.Nth.matches(node.Parent.elementCount-node.elementIndex+1)
	case "nth-of-type":
		return pseudo.Nth != nil && pseudo.Nth.matches(node.typeIndex)
	case "nth-last-of-type":
		return pseudo.Nth != nil && pseudo.Nth.matches(reverseTypePosition(node))
	case "not":
		return pseudo.Negation != nil && !matchesCompound(node, *pseudo.Negation)
	default:
		return false
	}
}

func reverseTypePosition(node *Node) int {
	if node.Parent == nil {
		return 0
	}
	return node.Parent.typeCounts[node.Tag] - node.typeIndex + 1
}

func (expression NthExpression) matches(position int) bool {
	if position <= 0 {
		return false
	}
	if expression.A == 0 {
		return position == expression.B
	}
	difference := position - expression.B
	if difference%expression.A != 0 {
		return false
	}
	return difference/expression.A >= 0
}

type selectorParser struct {
	s string
	i int
}

func (p *selectorParser) parseSelectorList() (Selector, error) {
	selector := Selector{}
	for {
		p.skipWhitespace()
		complex, err := p.parseComplex()
		if err != nil {
			return Selector{}, err
		}
		selector.Groups = append(selector.Groups, complex)
		p.skipWhitespace()
		if p.eof() || p.peek() != ',' {
			break
		}
		p.i++
		p.skipWhitespace()
		if p.eof() {
			return Selector{}, p.errorf("selector list cannot end with comma")
		}
	}
	if len(selector.Groups) == 0 {
		return Selector{}, p.errorf("empty selector")
	}
	return selector, nil
}

func (p *selectorParser) parseComplex() (ComplexSelector, error) {
	complex := ComplexSelector{}
	compound, err := p.parseCompound()
	if err != nil {
		return ComplexSelector{}, err
	}
	complex.Parts = append(complex.Parts, SelectorPart{Compound: compound})
	for {
		hadWhitespace := p.skipWhitespace()
		if p.eof() || p.peek() == ',' || p.peek() == ')' {
			break
		}
		combinator := NoCombinator
		switch p.peek() {
		case '>':
			combinator = ChildCombinator
			p.i++
			p.skipWhitespace()
		case '+':
			combinator = AdjacentCombinator
			p.i++
			p.skipWhitespace()
		case '~':
			combinator = GeneralSiblingCombinator
			p.i++
			p.skipWhitespace()
		default:
			if hadWhitespace {
				combinator = DescendantCombinator
			} else {
				return ComplexSelector{}, p.errorf("expected combinator")
			}
		}
		compound, err := p.parseCompound()
		if err != nil {
			return ComplexSelector{}, err
		}
		complex.Parts = append(complex.Parts, SelectorPart{Combinator: combinator, Compound: compound})
	}
	return complex, nil
}

func (p *selectorParser) parseCompound() (CompoundSelector, error) {
	compound := CompoundSelector{}
	consumed := false
	if !p.eof() && p.peek() == '*' {
		compound.Universal = true
		p.i++
		consumed = true
	} else if !p.eof() && isIdentStart(p.peek()) {
		identifier, err := p.parseIdentifier()
		if err != nil {
			return CompoundSelector{}, err
		}
		compound.TypeName = strings.ToLower(identifier)
		consumed = true
	}
	for !p.eof() {
		switch p.peek() {
		case '#':
			p.i++
			identifier, err := p.parseIdentifier()
			if err != nil {
				return CompoundSelector{}, err
			}
			if compound.ID != "" {
				return CompoundSelector{}, p.errorf("multiple ID selectors are unsupported")
			}
			compound.ID = identifier
			consumed = true
		case '.':
			p.i++
			identifier, err := p.parseIdentifier()
			if err != nil {
				return CompoundSelector{}, err
			}
			compound.Classes = append(compound.Classes, identifier)
			consumed = true
		case '[':
			attribute, err := p.parseAttribute()
			if err != nil {
				return CompoundSelector{}, err
			}
			compound.Attributes = append(compound.Attributes, attribute)
			consumed = true
		case ':':
			pseudo, err := p.parsePseudo()
			if err != nil {
				return CompoundSelector{}, err
			}
			compound.Pseudos = append(compound.Pseudos, pseudo)
			consumed = true
		default:
			if !consumed {
				return CompoundSelector{}, p.errorf("expected compound selector")
			}
			return compound, nil
		}
	}
	if !consumed {
		return CompoundSelector{}, p.errorf("expected compound selector")
	}
	return compound, nil
}

func (p *selectorParser) parseAttribute() (AttributeSelector, error) {
	p.i++
	p.skipWhitespace()
	name, err := p.parseIdentifier()
	if err != nil {
		return AttributeSelector{}, err
	}
	name = strings.ToLower(name)
	p.skipWhitespace()
	if p.eof() {
		return AttributeSelector{}, p.errorf("unterminated attribute selector")
	}
	if p.peek() == ']' {
		p.i++
		return AttributeSelector{Name: name}, nil
	}
	operator := ""
	for _, candidate := range []string{"~=", "|=", "^=", "$=", "*=", "="} {
		if strings.HasPrefix(p.s[p.i:], candidate) {
			operator = candidate
			p.i += len(candidate)
			break
		}
	}
	if operator == "" {
		return AttributeSelector{}, p.errorf("invalid attribute operator")
	}
	p.skipWhitespace()
	value, err := p.parseAttributeValue()
	if err != nil {
		return AttributeSelector{}, err
	}
	separated := p.skipWhitespace()
	if separated && !p.eof() && (p.peek() == 'i' || p.peek() == 'I' || p.peek() == 's' || p.peek() == 'S') {
		flag := p.peek()
		p.i++
		p.skipWhitespace()
		if !p.eof() && p.peek() == ']' {
			return AttributeSelector{}, p.errorf("attribute selector case-sensitivity flag %q is unsupported", string(flag))
		}
	}
	if p.eof() || p.peek() != ']' {
		return AttributeSelector{}, p.errorf("unterminated attribute selector")
	}
	p.i++
	return AttributeSelector{Name: name, Operator: operator, Value: value}, nil
}

func (p *selectorParser) parseAttributeValue() (string, error) {
	if p.eof() {
		return "", p.errorf("missing attribute value")
	}
	if quote := p.peek(); quote == '\'' || quote == '"' {
		p.i++
		var b strings.Builder
		for !p.eof() && p.peek() != quote {
			if p.peek() == '\\' {
				p.i++
				if p.eof() {
					return "", p.errorf("unterminated escape")
				}
			}
			b.WriteByte(p.peek())
			p.i++
		}
		if p.eof() {
			return "", p.errorf("unterminated quoted attribute value")
		}
		p.i++
		return b.String(), nil
	}
	start := p.i
	for !p.eof() && !unicode.IsSpace(rune(p.peek())) && p.peek() != ']' {
		p.i++
	}
	if start == p.i {
		return "", p.errorf("missing attribute value")
	}
	return p.s[start:p.i], nil
}

func (p *selectorParser) parsePseudo() (PseudoSelector, error) {
	p.i++
	if !p.eof() && p.peek() == ':' {
		return PseudoSelector{}, p.errorf("pseudo-elements are unsupported")
	}
	name, err := p.parseIdentifier()
	if err != nil {
		return PseudoSelector{}, err
	}
	name = strings.ToLower(name)
	simple := map[string]bool{
		"first-child": true, "last-child": true, "only-child": true, "empty": true,
		"first-of-type": true, "last-of-type": true, "only-of-type": true,
	}
	if simple[name] {
		return PseudoSelector{Name: name}, nil
	}
	if p.eof() || p.peek() != '(' {
		return PseudoSelector{}, p.errorf("unsupported pseudo-class %q", name)
	}
	p.i++
	start := p.i
	depth := 1
	quote := byte(0)
	for !p.eof() && depth > 0 {
		character := p.peek()
		if quote != 0 {
			if character == '\\' {
				p.i += 2
				continue
			}
			if character == quote {
				quote = 0
			}
			p.i++
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			p.i++
			continue
		}
		if character == '(' {
			depth++
		} else if character == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
		p.i++
	}
	if p.eof() || depth != 0 {
		return PseudoSelector{}, p.errorf("unterminated pseudo-class")
	}
	argument := strings.TrimSpace(p.s[start:p.i])
	p.i++
	switch name {
	case "nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type":
		nth, err := parseNth(argument)
		if err != nil {
			return PseudoSelector{}, p.errorf("invalid %s expression: %v", name, err)
		}
		return PseudoSelector{Name: name, Nth: &nth}, nil
	case "not":
		nested := selectorParser{s: argument}
		compound, err := nested.parseCompound()
		if err != nil {
			return PseudoSelector{}, p.errorf("invalid :not argument: %v", err)
		}
		nested.skipWhitespace()
		if !nested.eof() {
			return PseudoSelector{}, p.errorf(":not only accepts a compound selector")
		}
		return PseudoSelector{Name: name, Negation: &compound}, nil
	default:
		return PseudoSelector{}, p.errorf("unsupported pseudo-class %q", name)
	}
}

func parseNth(input string) (NthExpression, error) {
	value := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input), " ", ""))
	switch value {
	case "odd":
		return NthExpression{A: 2, B: 1}, nil
	case "even":
		return NthExpression{A: 2, B: 0}, nil
	}
	if !strings.Contains(value, "n") {
		integer, err := strconv.Atoi(value)
		if err != nil {
			return NthExpression{}, err
		}
		return NthExpression{B: integer}, nil
	}
	parts := strings.Split(value, "n")
	if len(parts) != 2 {
		return NthExpression{}, fmt.Errorf("invalid An+B form")
	}
	a := 0
	switch parts[0] {
	case "", "+":
		a = 1
	case "-":
		a = -1
	default:
		parsed, err := strconv.Atoi(parts[0])
		if err != nil {
			return NthExpression{}, err
		}
		a = parsed
	}
	b := 0
	if parts[1] != "" {
		parsed, err := strconv.Atoi(parts[1])
		if err != nil {
			return NthExpression{}, err
		}
		b = parsed
	}
	return NthExpression{A: a, B: b}, nil
}

func (p *selectorParser) parseIdentifier() (string, error) {
	if p.eof() || !isIdentStart(p.peek()) {
		return "", p.errorf("expected identifier")
	}
	start := p.i
	p.i++
	for !p.eof() && isIdentContinue(p.peek()) {
		p.i++
	}
	return p.s[start:p.i], nil
}

func isIdentStart(character byte) bool {
	return character == '_' || character == '-' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func isIdentContinue(character byte) bool {
	return isIdentStart(character) || character >= '0' && character <= '9'
}

func (p *selectorParser) skipWhitespace() bool {
	start := p.i
	for !p.eof() && unicode.IsSpace(rune(p.peek())) {
		p.i++
	}
	return p.i > start
}

func (p *selectorParser) eof() bool  { return p.i >= len(p.s) }
func (p *selectorParser) peek() byte { return p.s[p.i] }
func (p *selectorParser) errorf(format string, args ...any) error {
	return fmt.Errorf("selector byte %d: %s", p.i, fmt.Sprintf(format, args...))
}
