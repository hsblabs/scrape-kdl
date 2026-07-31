package authoring

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func Write(document Document) ([]byte, error) {
	catalog, err := BuiltinCatalog(document.LanguageVersion)
	if err != nil {
		return nil, err
	}
	writer := documentWriter{catalog: catalog}
	if err := writer.writeDocument(document); err != nil {
		return nil, err
	}
	return []byte(writer.output.String()), nil
}

type documentWriter struct {
	catalog Catalog
	output  strings.Builder
}

func (writer *documentWriter) writeDocument(document Document) error {
	extractor := document.Extractor
	if err := writer.line(0, "extractor ", quoted(extractor.Name), " version=", quoted(extractor.Version), " language-version=", quoted(document.LanguageVersion), " {"); err != nil {
		return err
	}
	if err := writer.line(1, "source \"html\" {"); err != nil {
		return err
	}
	if extractor.Source.FetchMode != scrapekdl.FetchModeHTTP && extractor.Source.FetchMode != scrapekdl.FetchModeBrowser {
		return fmt.Errorf("authoring: unsupported fetch mode %q", extractor.Source.FetchMode)
	}
	if err := writer.line(2, "fetch mode=", quoted(string(extractor.Source.FetchMode)), " url=", quoted(extractor.Source.URLTemplate)); err != nil {
		return err
	}
	if extractor.Source.SessionPolicy != scrapekdl.SessionPolicyNone && extractor.Source.SessionPolicy != scrapekdl.SessionPolicyOptional && extractor.Source.SessionPolicy != scrapekdl.SessionPolicyRequired {
		return fmt.Errorf("authoring: unsupported session policy %q", extractor.Source.SessionPolicy)
	}
	if err := writer.line(2, "session policy=", quoted(string(extractor.Source.SessionPolicy))); err != nil {
		return err
	}
	if err := writer.line(1, "}"); err != nil {
		return err
	}
	for _, input := range extractor.Inputs {
		if input.Type != PrimitiveString && input.Type != PrimitiveBool && input.Type != PrimitiveInt && input.Type != PrimitiveFloat {
			return fmt.Errorf("authoring: input %q has unsupported primitive type %q", input.Name, input.Type)
		}
		if err := writer.line(1, "input ", quoted(input.Name), " type=", quoted(string(input.Type)), " required=", boolLiteral(input.Required)); err != nil {
			return err
		}
	}
	for _, member := range extractor.Members {
		if err := writer.writeMember(1, member); err != nil {
			return err
		}
	}
	return writer.line(0, "}")
}

func (writer *documentWriter) writeMember(depth int, member Member) error {
	switch typed := member.(type) {
	case Field:
		return writer.writeField(depth, typed)
	case *Field:
		if typed == nil {
			return errors.New("authoring: field member is nil")
		}
		return writer.writeField(depth, *typed)
	case Collection:
		return writer.writeCollection(depth, typed)
	case *Collection:
		if typed == nil {
			return errors.New("authoring: collection member is nil")
		}
		return writer.writeCollection(depth, *typed)
	default:
		return fmt.Errorf("authoring: unsupported member %T", member)
	}
}

func (writer *documentWriter) writeField(depth int, field Field) error {
	if field.Match != MatchOne && field.Match != MatchFirst {
		return fmt.Errorf("authoring: field %q has unsupported match mode %q", field.Name, field.Match)
	}
	if field.OnError != ErrorFail && field.OnError != ErrorNull && field.OnError != ErrorWarn {
		return fmt.Errorf("authoring: field %q has unsupported error policy %q", field.Name, field.OnError)
	}
	if err := writer.line(depth, "field ", quoted(field.Name), " type=", quoted(field.Type), " required=", boolLiteral(field.Required), " {"); err != nil {
		return err
	}
	if err := writer.line(depth+1, "select ", quoted(field.Selector), " match=", quoted(string(field.Match))); err != nil {
		return err
	}
	switch value := field.Value.(type) {
	case TextValue:
		if err := writer.line(depth+1, "value \"text\""); err != nil {
			return err
		}
	case HTMLValue:
		if err := writer.line(depth+1, "value \"html\""); err != nil {
			return err
		}
	case AttributeValue:
		if err := writer.line(depth+1, "value \"attr\" name=", quoted(value.Name)); err != nil {
			return err
		}
	case *TextValue:
		if value == nil {
			return fmt.Errorf("authoring: field %q has nil text value source", field.Name)
		}
		if err := writer.line(depth+1, "value \"text\""); err != nil {
			return err
		}
	case *HTMLValue:
		if value == nil {
			return fmt.Errorf("authoring: field %q has nil HTML value source", field.Name)
		}
		if err := writer.line(depth+1, "value \"html\""); err != nil {
			return err
		}
	case *AttributeValue:
		if value == nil {
			return fmt.Errorf("authoring: field %q has nil attribute value source", field.Name)
		}
		if err := writer.line(depth+1, "value \"attr\" name=", quoted(value.Name)); err != nil {
			return err
		}
	default:
		return fmt.Errorf("authoring: field %q has unsupported value source %T", field.Name, field.Value)
	}
	for _, call := range field.Transforms {
		if err := writer.writeBuiltinCall(depth+1, call); err != nil {
			return fmt.Errorf("authoring: field %q: %w", field.Name, err)
		}
	}
	if field.OnError != ErrorFail {
		if err := writer.line(depth+1, "on-error ", quoted(string(field.OnError))); err != nil {
			return err
		}
	}
	return writer.line(depth, "}")
}

func (writer *documentWriter) writeCollection(depth int, collection Collection) error {
	if collection.MinItems < 0 {
		return fmt.Errorf("authoring: collection %q has negative min-items", collection.Name)
	}
	if collection.MaxItems != nil && *collection.MaxItems < 0 {
		return fmt.Errorf("authoring: collection %q has negative max-items", collection.Name)
	}
	if collection.OnRowError != RowErrorFail && collection.OnRowError != RowErrorSkip {
		return fmt.Errorf("authoring: collection %q has unsupported row error policy %q", collection.Name, collection.OnRowError)
	}
	parts := []kdlPart{
		literal("collection "), quoted(collection.Name), literal(" required="), literal(boolLiteral(collection.Required)),
		literal(" min-items="), literal(strconv.Itoa(collection.MinItems)),
	}
	if collection.MaxItems != nil {
		parts = append(parts, literal(" max-items="), literal(strconv.Itoa(*collection.MaxItems)))
	}
	parts = append(parts, literal(" on-row-error="), quoted(string(collection.OnRowError)), literal(" {"))
	if err := writer.lineParts(depth, parts...); err != nil {
		return err
	}
	if err := writer.line(depth+1, "select ", quoted(collection.Selector)); err != nil {
		return err
	}
	for _, member := range collection.Members {
		if err := writer.writeMember(depth+1, member); err != nil {
			return err
		}
	}
	return writer.line(depth, "}")
}

func (writer *documentWriter) writeBuiltinCall(depth int, call BuiltinCall) error {
	definition, ok := writer.catalog.Lookup(call.Name)
	if !ok {
		return fmt.Errorf("unknown built-in %q for language version %s", call.Name, writer.catalog.LanguageVersion)
	}
	maximum := definition.PositionalArguments.Max
	if len(call.Positional) < definition.PositionalArguments.Min || maximum >= 0 && len(call.Positional) > maximum {
		return fmt.Errorf("built-in %q accepts %d..%s positional arguments, got %d", call.Name, definition.PositionalArguments.Min, positionalMaximum(maximum), len(call.Positional))
	}
	parts := []kdlPart{literal("apply "), quoted(call.Name)}
	for _, value := range call.Positional {
		parts = append(parts, literal(" "), scalarPart(value))
	}
	known := make(map[string]NamedArgument, len(definition.NamedArguments))
	for _, argument := range definition.NamedArguments {
		known[argument.Name] = argument
		value, exists := call.Named[argument.Name]
		if argument.Required && !exists {
			return fmt.Errorf("built-in %q requires named argument %q", call.Name, argument.Name)
		}
		if !exists {
			continue
		}
		if err := validateScalarConstraint(value, argument.Constraint); err != nil {
			return fmt.Errorf("built-in %q argument %q: %w", call.Name, argument.Name, err)
		}
		parts = append(parts, literal(" "+argument.Name+"="), scalarPart(value))
	}
	for name := range call.Named {
		if _, ok := known[name]; !ok {
			return fmt.Errorf("built-in %q does not accept named argument %q", call.Name, name)
		}
	}
	return writer.lineParts(depth, parts...)
}

func validateScalarConstraint(value Scalar, constraint string) error {
	switch constraint {
	case "string":
		if value.kind != ScalarString {
			return errors.New("value must be string")
		}
	case "bool":
		if value.kind != ScalarBool {
			return errors.New("value must be bool")
		}
	case "int":
		if value.kind != ScalarInt {
			return errors.New("value must be int")
		}
	case "non-negative-int":
		if value.kind != ScalarInt || value.intValue < 0 {
			return errors.New("value must be a non-negative int")
		}
	case "number":
		if value.kind != ScalarInt && value.kind != ScalarFloat {
			return errors.New("value must be a number")
		}
	case "scalar":
		if value.kind == "" {
			return errors.New("value must be initialized")
		}
	default:
		return fmt.Errorf("unknown catalog constraint %q", constraint)
	}
	return nil
}

func positionalMaximum(maximum int) string {
	if maximum < 0 {
		return "unbounded"
	}
	return strconv.Itoa(maximum)
}

type kdlPart func(*strings.Builder) error

func literal(value string) kdlPart {
	return func(output *strings.Builder) error {
		output.WriteString(value)
		return nil
	}
}

func quoted(value string) kdlPart {
	return func(output *strings.Builder) error { return writeQuotedString(output, value) }
}

func scalarPart(value Scalar) kdlPart {
	return func(output *strings.Builder) error {
		switch value.kind {
		case ScalarString:
			return writeQuotedString(output, value.stringValue)
		case ScalarBool:
			output.WriteString(boolLiteral(value.boolValue))
		case ScalarInt:
			output.WriteString(strconv.FormatInt(value.intValue, 10))
		case ScalarFloat:
			if math.IsNaN(value.floatValue) || math.IsInf(value.floatValue, 0) {
				return errors.New("authoring: float scalar must be finite")
			}
			output.WriteString(strconv.FormatFloat(value.floatValue, 'g', -1, 64))
		case ScalarNull:
			output.WriteString("#null")
		default:
			return errors.New("authoring: scalar is uninitialized")
		}
		return nil
	}
}

func (writer *documentWriter) line(depth int, rawParts ...any) error {
	writer.output.WriteString(strings.Repeat("  ", depth))
	for _, raw := range rawParts {
		switch part := raw.(type) {
		case string:
			writer.output.WriteString(part)
		case kdlPart:
			if err := part(&writer.output); err != nil {
				return err
			}
		default:
			return fmt.Errorf("authoring: invalid writer part %T", raw)
		}
	}
	writer.output.WriteByte('\n')
	return nil
}

func (writer *documentWriter) lineParts(depth int, parts ...kdlPart) error {
	writer.output.WriteString(strings.Repeat("  ", depth))
	for _, part := range parts {
		if err := part(&writer.output); err != nil {
			return err
		}
	}
	writer.output.WriteByte('\n')
	return nil
}

func writeQuotedString(output *strings.Builder, value string) error {
	if !utf8.ValidString(value) {
		return errors.New("authoring: string is not valid UTF-8")
	}
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character < 0x20 || character == 0x7f {
				fmt.Fprintf(output, `\u{%x}`, character)
			} else {
				output.WriteRune(character)
			}
		}
	}
	output.WriteByte('"')
	return nil
}

func boolLiteral(value bool) string {
	if value {
		return "#true"
	}
	return "#false"
}
