package main

import (
	"fmt"
	"go/constant"
	"go/types"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const generatedHeader = "// GENERATED — do not edit\n"

type config struct {
	dir                string
	packagePattern     string
	rootType           string
	additionalTypes    []string
	versionConstant    string
	palette            paletteConfig
	discriminatedTypes map[string]discriminatedTypeConfig
}

type discriminatedTypeConfig struct {
	field    string
	variants map[string][]string
}

// paletteConfig carries Go-owned colour *values*. Types can be reflected out of
// the contract package; values cannot, so the generator's caller imports the
// package that declares them and hands them over here. The runtime renders
// these, which is why they are generated rather than restated in TypeScript.
type paletteConfig struct {
	// series is the categorical palette in its declared order. Order is the
	// contract: index n means the same colour on both sides.
	series []string
	// neutral is the colour reserved for collapsed remainders.
	neutral string
}

type contractModel struct {
	pkg     *types.Package
	root    string
	version string
	types   map[string]*types.Named
	enums   map[string][]string
}

type jsonField struct {
	name           string
	typ            types.Type
	optional       bool
	zodConstraints string
}

func generate(cfg config) (map[string]string, error) {
	model, err := loadContract(cfg)
	if err != nil {
		return nil, err
	}
	typesFile, err := emitTypes(model, cfg.discriminatedTypes)
	if err != nil {
		return nil, err
	}
	schemasFile, err := emitSchemas(model, cfg.discriminatedTypes)
	if err != nil {
		return nil, err
	}
	paletteFile, err := emitPalette(cfg.palette)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"index.ts":   generatedHeader + "\nexport * from './palette'\nexport * from './schemas'\nexport * from './types'\n",
		"palette.ts": paletteFile,
		"schemas.ts": schemasFile,
		"types.ts":   typesFile,
	}, nil
}

// emitPalette writes the Go-owned palette values into the runtime module.
// Category-to-colour assignment is a separate React runtime responsibility.
func emitPalette(cfg paletteConfig) (string, error) {
	if len(cfg.series) == 0 {
		return "", fmt.Errorf("palette has no series colors")
	}
	var output strings.Builder
	output.WriteString(generatedHeader)
	output.WriteString(`
/**
 * The Lens categorical palette, in the order Go declares it (pkg/lens/color).
 *
 * Values only. Which colour a given category is painted with is decided by
 * ` + "`charts/palette.ts`" + `; nothing generated here owns assignment.
 */
export const PALETTE_SERIES = [
`)
	for _, value := range cfg.series {
		normalized, err := normalizeHexColor(value)
		if err != nil {
			return "", fmt.Errorf("palette series: %w", err)
		}
		output.WriteString("  '")
		output.WriteString(normalized)
		output.WriteString("',\n")
	}
	output.WriteString("] as const\n\n")
	neutral, err := normalizeHexColor(cfg.neutral)
	if err != nil {
		return "", fmt.Errorf("palette neutral: %w", err)
	}
	output.WriteString(`/**
 * The colour reserved for a collapsed remainder, so an aggregated tail reads as
 * de-emphasized rather than as one more category.
 */
export const PALETTE_NEUTRAL = '`)
	output.WriteString(neutral)
	output.WriteString("'\n")
	return output.String(), nil
}

// normalizeHexColor lowercases a #rrggbb value and rejects anything else. The
// case is cosmetic — CSS does not care — but a generated file that changes case
// between runs would fail the drift check, and a malformed colour would
// otherwise ship as a silently unpaintable string.
func normalizeHexColor(value string) (string, error) {
	if len(value) != 7 || value[0] != '#' {
		return "", fmt.Errorf("color %q is not a #rrggbb hex value", value)
	}
	for _, digit := range value[1:] {
		isHex := (digit >= '0' && digit <= '9') || (digit >= 'a' && digit <= 'f') || (digit >= 'A' && digit <= 'F')
		if !isHex {
			return "", fmt.Errorf("color %q is not a #rrggbb hex value", value)
		}
	}
	return strings.ToLower(value), nil
}

func loadContract(cfg config) (*contractModel, error) {
	loaded, err := packages.Load(&packages.Config{
		Dir:  cfg.dir,
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo,
	}, cfg.packagePattern)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(loaded) > 0 {
		return nil, fmt.Errorf("package %s has load errors", cfg.packagePattern)
	}
	if len(loaded) != 1 {
		return nil, fmt.Errorf("package pattern %s matched %d packages", cfg.packagePattern, len(loaded))
	}
	pkg := loaded[0].Types
	rootObject, ok := pkg.Scope().Lookup(cfg.rootType).(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("root type %s not found", cfg.rootType)
	}
	root, ok := types.Unalias(rootObject.Type()).(*types.Named)
	if !ok {
		return nil, fmt.Errorf("root %s is not a named type", cfg.rootType)
	}
	if _, ok := root.Underlying().(*types.Struct); !ok {
		return nil, fmt.Errorf("root %s is not a struct", cfg.rootType)
	}
	versionObject, ok := pkg.Scope().Lookup(cfg.versionConstant).(*types.Const)
	if !ok || versionObject.Val().Kind() != constant.String {
		return nil, fmt.Errorf("version constant %s is not a string", cfg.versionConstant)
	}
	model := &contractModel{
		pkg:     pkg,
		root:    root.Obj().Name(),
		version: constant.StringVal(versionObject.Val()),
		types:   make(map[string]*types.Named),
		enums:   make(map[string][]string),
	}
	collectNamedTypes(model, root)
	for _, name := range cfg.additionalTypes {
		object, ok := pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			return nil, fmt.Errorf("additional type %s not found", name)
		}
		collectNamedTypes(model, object.Type())
	}
	collectEnums(model)
	return model, nil
}

func collectNamedTypes(model *contractModel, typ types.Type) {
	typ = types.Unalias(typ)
	switch typed := typ.(type) {
	case *types.Named:
		object := typed.Obj()
		if object.Pkg() != model.pkg {
			return
		}
		if _, exists := model.types[object.Name()]; exists {
			return
		}
		model.types[object.Name()] = typed
		collectNamedTypes(model, typed.Underlying())
	case *types.Pointer:
		collectNamedTypes(model, typed.Elem())
	case *types.Slice:
		collectNamedTypes(model, typed.Elem())
	case *types.Array:
		collectNamedTypes(model, typed.Elem())
	case *types.Map:
		collectNamedTypes(model, typed.Key())
		collectNamedTypes(model, typed.Elem())
	case *types.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			field := typed.Field(index)
			if field.Exported() {
				collectNamedTypes(model, field.Type())
			}
		}
	}
}

// collectEnums treats an exported string-underlying named type as an enum only
// when at least one constant of that type exists in the contract package itself;
// a type whose constants live elsewhere (or nowhere) degenerates to an open
// string with no value validation, silently.
func collectEnums(model *contractModel) {
	names := model.pkg.Scope().Names()
	for _, typeName := range names {
		typeObject, ok := model.pkg.Scope().Lookup(typeName).(*types.TypeName)
		if !ok || !typeObject.Exported() {
			continue
		}
		named, ok := types.Unalias(typeObject.Type()).(*types.Named)
		if !ok {
			continue
		}
		basic, ok := named.Underlying().(*types.Basic)
		if !ok || basic.Info()&types.IsString == 0 {
			continue
		}
		values := make([]string, 0)
		for _, name := range names {
			object, ok := model.pkg.Scope().Lookup(name).(*types.Const)
			if !ok || !types.Identical(types.Unalias(object.Type()), named) || object.Val().Kind() != constant.String {
				continue
			}
			values = append(values, constant.StringVal(object.Val()))
		}
		if len(values) > 0 {
			sort.Strings(values)
			model.types[typeName] = named
			model.enums[typeName] = values
		}
	}
}

func emitTypes(model *contractModel, discriminatedTypes map[string]discriminatedTypeConfig) (string, error) {
	var output strings.Builder
	output.WriteString(generatedHeader)
	output.WriteString("\nexport const CONTRACT_VERSION = ")
	output.WriteString(strconv.Quote(model.version))
	output.WriteString("\n\n")
	for _, name := range sortedTypeNames(model) {
		named := model.types[name]
		if values := model.enums[name]; len(values) > 0 {
			output.WriteString("export type ")
			output.WriteString(name)
			output.WriteString(" = ")
			output.WriteString(quotedUnion(values))
			output.WriteString("\n\n")
			continue
		}
		if structure, ok := named.Underlying().(*types.Struct); ok {
			fields, err := exportedJSONFields(structure)
			if err != nil {
				return "", fmt.Errorf("emit %s: %w", name, err)
			}
			if discriminated, ok := discriminatedTypes[name]; ok {
				if err := emitDiscriminatedType(&output, name, fields, discriminated); err != nil {
					return "", err
				}
			} else {
				if err := emitInterface(&output, name, fields); err != nil {
					return "", fmt.Errorf("emit %s: %w", name, err)
				}
			}
			continue
		}
		value, err := emitTSType(named.Underlying(), false)
		if err != nil {
			return "", fmt.Errorf("emit %s: %w", name, err)
		}
		output.WriteString("export type ")
		output.WriteString(name)
		output.WriteString(" = ")
		output.WriteString(value)
		output.WriteString("\n\n")
	}
	return strings.TrimRight(output.String(), "\n") + "\n", nil
}

func emitInterface(output *strings.Builder, name string, fields []jsonField) error {
	output.WriteString("export interface ")
	output.WriteString(name)
	output.WriteString(" {\n")
	for _, field := range fields {
		typeName, err := emitTSType(field.typ, field.optional)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.name, err)
		}
		output.WriteString("  ")
		output.WriteString(field.name)
		if field.optional {
			output.WriteString("?")
		}
		output.WriteString(": ")
		output.WriteString(typeName)
		output.WriteString("\n")
	}
	output.WriteString("}\n\n")
	return nil
}

func emitDiscriminatedType(output *strings.Builder, name string, fields []jsonField, cfg discriminatedTypeConfig) error {
	fieldByName := make(map[string]jsonField, len(fields))
	specific := make(map[string]struct{})
	for _, field := range fields {
		fieldByName[field.name] = field
	}
	for _, names := range cfg.variants {
		for _, field := range names {
			specific[field] = struct{}{}
		}
	}
	if _, ok := fieldByName[cfg.field]; !ok {
		return fmt.Errorf("emit %s: discriminant %s not found", name, cfg.field)
	}
	base := make([]jsonField, 0, len(fields))
	for _, field := range fields {
		if field.name == cfg.field {
			continue
		}
		if _, ok := specific[field.name]; !ok {
			base = append(base, field)
		}
	}
	if err := emitInterface(output, name+"Base", base); err != nil {
		return err
	}
	variants := make([]string, 0, len(cfg.variants))
	for variant := range cfg.variants {
		variants = append(variants, variant)
	}
	sort.Strings(variants)
	fieldNames := make([]string, 0, len(specific))
	for fieldName := range specific {
		fieldNames = append(fieldNames, fieldName)
	}
	sort.Strings(fieldNames)
	output.WriteString("export type ")
	output.WriteString(name)
	output.WriteString(" =")
	for index, variant := range variants {
		if index > 0 {
			output.WriteString("\n  | ")
		} else {
			output.WriteString("\n  | ")
		}
		allowed := make(map[string]struct{}, len(cfg.variants[variant]))
		for _, field := range cfg.variants[variant] {
			allowed[field] = struct{}{}
		}
		output.WriteString(name)
		output.WriteString("Base & { ")
		output.WriteString(cfg.field)
		output.WriteString(": ")
		output.WriteString(strconv.Quote(variant))
		for _, fieldName := range fieldNames {
			output.WriteString("; ")
			output.WriteString(fieldName)
			if _, ok := allowed[fieldName]; !ok {
				output.WriteString("?: never")
				continue
			}
			field := fieldByName[fieldName]
			typeName, err := emitTSType(field.typ, field.optional)
			if err != nil {
				return fmt.Errorf("emit %s.%s: %w", name, fieldName, err)
			}
			if field.optional {
				output.WriteString("?")
			}
			output.WriteString(": ")
			output.WriteString(typeName)
		}
		output.WriteString(" }")
	}
	output.WriteString("\n\n")
	output.WriteString("export const ")
	output.WriteString(strings.ToUpper(name))
	output.WriteString("_KIND_CONFIG_FIELDS = {\n")
	for _, variant := range variants {
		output.WriteString("  ")
		output.WriteString(strconv.Quote(variant))
		output.WriteString(": [")
		fields := append([]string(nil), cfg.variants[variant]...)
		sort.Strings(fields)
		for index, field := range fields {
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteString(strconv.Quote(field))
		}
		output.WriteString("],\n")
	}
	output.WriteString("} as const satisfies Record<PanelKind, readonly string[]>\n\n")
	return nil
}

func emitSchemas(model *contractModel, discriminatedTypes map[string]discriminatedTypeConfig) (string, error) {
	var output strings.Builder
	output.WriteString(generatedHeader)
	typeImports := []string{"CONTRACT_VERSION"}
	for name := range discriminatedTypes {
		typeImports = append(typeImports, strings.ToUpper(name)+"_KIND_CONFIG_FIELDS")
	}
	sort.Strings(typeImports)
	output.WriteString("\nimport { z } from 'zod'\nimport { ")
	output.WriteString(strings.Join(typeImports, ", "))
	output.WriteString(" } from './types'\nimport type * as Contract from './types'\n\n")
	output.WriteString("const CONTRACT_MAJOR_VERSION = CONTRACT_VERSION.split('.', 1)[0]!\n\n")
	output.WriteString("function contractMajor(version: string): string {\n  return version.split('.', 1)[0]!\n}\n\n")
	output.WriteString("export class ContractVersionMismatchError extends Error {\n")
	output.WriteString("  readonly code = 'CONTRACT_VERSION_MISMATCH'\n  readonly expectedMajor = CONTRACT_MAJOR_VERSION\n\n")
	output.WriteString("  constructor(readonly actualVersion: string) {\n")
	output.WriteString("    super(`Lens contract major version ${contractMajor(actualVersion)} is incompatible with expected major ${CONTRACT_MAJOR_VERSION}`)\n")
	output.WriteString("    this.name = 'ContractVersionMismatchError'\n  }\n}\n\n")
	output.WriteString("export const ContractVersionSchema: z.ZodType<string> = z.string().refine(\n")
	output.WriteString("  (version) => contractMajor(version) === CONTRACT_MAJOR_VERSION,\n")
	output.WriteString("  { message: `Expected Lens contract major version ${CONTRACT_MAJOR_VERSION}` },\n)\n\n")

	for _, name := range sortedTypeNames(model) {
		var expression string
		var err error
		if values := model.enums[name]; len(values) > 0 {
			expression = "z.enum([" + quotedList(values) + "])"
		} else {
			expression, err = emitZodType(model.types[name].Underlying(), false, 0)
		}
		if err != nil {
			return "", fmt.Errorf("emit %s schema: %w", name, err)
		}
		if name == model.root {
			structure := model.types[name].Underlying().(*types.Struct)
			expression, err = emitZodStruct(structure, 0, true)
			if err != nil {
				return "", fmt.Errorf("emit %s schema: %w", name, err)
			}
		}
		output.WriteString("export const ")
		output.WriteString(name)
		output.WriteString("Schema: z.ZodType<Contract.")
		output.WriteString(name)
		output.WriteString("> = ")
		if hasForwardSchemaReference(model.types[name].Underlying(), name) {
			output.WriteString("z.lazy(() => ")
			output.WriteString(expression)
			output.WriteString(")")
		} else {
			output.WriteString(expression)
		}
		if discriminated, ok := discriminatedTypes[name]; ok {
			constant := strings.ToUpper(name) + "_KIND_CONFIG_FIELDS"
			output.WriteString(".superRefine((value, ctx) => {\n")
			output.WriteString("  const allowed = new Set<string>(")
			output.WriteString(constant)
			output.WriteString("[value.")
			output.WriteString(discriminated.field)
			output.WriteString("])\n")
			output.WriteString("  const candidate = value as unknown as Record<string, unknown>\n")
			output.WriteString("  for (const field of Object.values(")
			output.WriteString(constant)
			output.WriteString(").flat()) {\n")
			output.WriteString("    if (!allowed.has(field) && candidate[field] !== undefined) {\n")
			output.WriteString("      ctx.addIssue({ code: z.ZodIssueCode.custom, path: [field], message: `${field} is not valid for ${value.")
			output.WriteString(discriminated.field)
			output.WriteString("}` })\n")
			output.WriteString("    }\n  }\n})")
			output.WriteString(" as z.ZodType<Contract.")
			output.WriteString(name)
			output.WriteString(">")
		}
		output.WriteString("\n\n")
	}

	output.WriteString("const DocumentVersionSchema = z.object({ version: z.string() }).passthrough()\n\n")
	output.WriteString(`function panelIsActionable(panel: Contract.Panel): boolean {
  return Boolean(
    panel.drillRoot || panel.actions.length > 0 || panel.columns?.some((column) => column.action) ||
    panel.metricFlow?.stages.some((stage) => stage.action) ||
    panel.metricHierarchy?.rows.some((row) => row.action) ||
    panel.metricRelationship?.source.action || panel.metricRelationship?.target.action
  )
}

function assertPanelInteractionContract(document: Contract.DashboardDocument): void {
  document.panels.forEach((panel, index) => {
    const actionable = panelIsActionable(panel)
    if (panel.terminal && actionable) {
      throw new z.ZodError([{ code: 'custom', path: ['panels', index], message: 'panel ' + panel.id + ' cannot be terminal and actionable' }])
    }
    if (!panel.terminal && !actionable) {
      throw new z.ZodError([{ code: 'custom', path: ['panels', index], message: 'panel ' + panel.id + ' must be actionable or explicitly terminal' }])
    }
  })
}

`)
	output.WriteString("export function parseDocument(input: unknown): Contract.")
	output.WriteString(model.root)
	output.WriteString(" {\n")
	output.WriteString("  const version = DocumentVersionSchema.safeParse(input)\n")
	output.WriteString("  if (version.success && contractMajor(version.data.version) !== CONTRACT_MAJOR_VERSION) {\n")
	output.WriteString("    throw new ContractVersionMismatchError(version.data.version)\n  }\n")
	output.WriteString("  const document = ")
	output.WriteString(model.root)
	output.WriteString("Schema.parse(input)\n")
	output.WriteString("  assertPanelInteractionContract(document)\n")
	output.WriteString("  return document\n}\n")
	return output.String(), nil
}

func hasForwardSchemaReference(typ types.Type, schemaName string) bool {
	typ = types.Unalias(typ)
	switch typed := typ.(type) {
	case *types.Named:
		return !isTime(typed) && typed.Obj().Name() > schemaName
	case *types.Pointer:
		return hasForwardSchemaReference(typed.Elem(), schemaName)
	case *types.Slice:
		return hasForwardSchemaReference(typed.Elem(), schemaName)
	case *types.Array:
		return hasForwardSchemaReference(typed.Elem(), schemaName)
	case *types.Map:
		return hasForwardSchemaReference(typed.Key(), schemaName) || hasForwardSchemaReference(typed.Elem(), schemaName)
	case *types.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			field := typed.Field(index)
			if !field.Exported() {
				continue
			}
			_, _, skip := parseJSONTag(field.Name(), typed.Tag(index))
			if !skip && hasForwardSchemaReference(field.Type(), schemaName) {
				return true
			}
		}
	}
	return false
}

func emitTSType(typ types.Type, omitNilPointer bool) (string, error) {
	typ = types.Unalias(typ)
	switch typed := typ.(type) {
	case *types.Named:
		if isTime(typed) {
			return "string", nil
		}
		return typed.Obj().Name(), nil
	case *types.Pointer:
		value, err := emitTSType(typed.Elem(), false)
		if err != nil {
			return "", err
		}
		if omitNilPointer {
			return value, nil
		}
		return value + " | null", nil
	case *types.Slice:
		value, err := emitTSType(typed.Elem(), false)
		if err != nil {
			return "", err
		}
		return "Array<" + value + ">", nil
	case *types.Array:
		value, err := emitTSType(typed.Elem(), false)
		if err != nil {
			return "", err
		}
		return "Array<" + value + ">", nil
	case *types.Map:
		key, err := emitTSType(typed.Key(), false)
		if err != nil {
			return "", err
		}
		value, err := emitTSType(typed.Elem(), false)
		if err != nil {
			return "", err
		}
		return "Record<" + key + ", " + value + ">", nil
	case *types.Basic:
		return basicTSType(typed)
	case *types.Interface:
		if typed.Empty() {
			return "unknown", nil
		}
		return "", fmt.Errorf("non-empty interface %s is not supported", typed.String())
	default:
		return "", fmt.Errorf("unsupported Go type %s", typ.String())
	}
}

func emitZodType(typ types.Type, omitNilPointer bool, indent int) (string, error) {
	typ = types.Unalias(typ)
	switch typed := typ.(type) {
	case *types.Named:
		if isTime(typed) {
			return "z.string().datetime({ offset: true })", nil
		}
		return "z.lazy(() => " + typed.Obj().Name() + "Schema)", nil
	case *types.Pointer:
		value, err := emitZodType(typed.Elem(), false, indent)
		if err != nil {
			return "", err
		}
		if omitNilPointer {
			return value, nil
		}
		return value + ".nullable()", nil
	case *types.Slice, *types.Array:
		var elem types.Type
		if slice, ok := typed.(*types.Slice); ok {
			elem = slice.Elem()
		} else {
			elem = typed.(*types.Array).Elem()
		}
		value, err := emitZodType(elem, false, indent)
		if err != nil {
			return "", err
		}
		return "z.array(" + value + ")", nil
	case *types.Map:
		key, err := emitZodType(typed.Key(), false, indent)
		if err != nil {
			return "", err
		}
		value, err := emitZodType(typed.Elem(), false, indent)
		if err != nil {
			return "", err
		}
		return "z.record(" + key + ", " + value + ")", nil
	case *types.Struct:
		return emitZodStruct(typed, indent, false)
	case *types.Basic:
		return basicZodType(typed)
	case *types.Interface:
		if typed.Empty() {
			return "z.unknown()", nil
		}
		return "", fmt.Errorf("non-empty interface %s is not supported", typed.String())
	default:
		return "", fmt.Errorf("unsupported Go type %s", typ.String())
	}
}

func emitZodStruct(structure *types.Struct, indent int, contractRoot bool) (string, error) {
	fields, err := exportedJSONFields(structure)
	if err != nil {
		return "", err
	}
	padding := strings.Repeat("  ", indent)
	fieldPadding := strings.Repeat("  ", indent+1)
	var output strings.Builder
	output.WriteString("z.object({\n")
	for _, field := range fields {
		var expression string
		if contractRoot && field.name == "version" {
			expression = "ContractVersionSchema"
		} else {
			expression, err = emitZodType(field.typ, field.optional, indent+1)
			if err != nil {
				return "", fmt.Errorf("field %s: %w", field.name, err)
			}
		}
		expression += field.zodConstraints
		if field.optional {
			expression += ".optional()"
		}
		output.WriteString(fieldPadding)
		output.WriteString(field.name)
		output.WriteString(": ")
		output.WriteString(expression)
		output.WriteString(",\n")
	}
	output.WriteString(padding)
	output.WriteString("}).strict()")
	return output.String(), nil
}

func exportedJSONFields(structure *types.Struct) ([]jsonField, error) {
	fields := make([]jsonField, 0, structure.NumFields())
	for index := 0; index < structure.NumFields(); index++ {
		field := structure.Field(index)
		if !field.Exported() {
			continue
		}
		if field.Anonymous() {
			return nil, fmt.Errorf("anonymous field %s is not supported", field.Name())
		}
		name, optional, skip := parseJSONTag(field.Name(), structure.Tag(index))
		if skip {
			continue
		}
		constraints, err := parseLensValidationTag(structure.Tag(index), field.Type())
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", field.Name(), err)
		}
		fields = append(fields, jsonField{name: name, typ: field.Type(), optional: optional, zodConstraints: constraints})
	}
	return fields, nil
}

func parseLensValidationTag(tag string, typ types.Type) (string, error) {
	raw := strings.TrimSpace(reflect.StructTag(tag).Get("lens"))
	if raw == "" {
		return "", nil
	}
	base := types.Unalias(typ)
	if pointer, ok := base.(*types.Pointer); ok {
		base = types.Unalias(pointer.Elem())
	}
	basic, ok := base.(*types.Basic)
	if !ok || basic.Info()&types.IsNumeric == 0 {
		return "", fmt.Errorf("lens numeric constraints require a numeric field")
	}
	var output strings.Builder
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 || (parts[0] != "min" && parts[0] != "max") {
			return "", fmt.Errorf("unsupported lens constraint %q", item)
		}
		if seen[parts[0]] {
			return "", fmt.Errorf("duplicate lens constraint %q", parts[0])
		}
		seen[parts[0]] = true
		value := strings.TrimSpace(parts[1])
		numeric, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(numeric) || math.IsInf(numeric, 0) {
			return "", fmt.Errorf("invalid lens constraint %s=%q", parts[0], value)
		}
		output.WriteString(".")
		output.WriteString(parts[0])
		output.WriteString("(")
		output.WriteString(value)
		output.WriteString(")")
	}
	return output.String(), nil
}

func parseJSONTag(fieldName, tag string) (string, bool, bool) {
	value, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return fieldName, false, false
	}
	parts := strings.Split(value, ",")
	if parts[0] == "-" {
		return "", false, true
	}
	name := parts[0]
	if name == "" {
		name = fieldName
	}
	optional := false
	for _, option := range parts[1:] {
		if option == "omitempty" || option == "omitzero" {
			optional = true
		}
	}
	return name, optional, false
}

func basicTSType(basic *types.Basic) (string, error) {
	if basic.Kind() == types.UntypedNil {
		return "null", nil
	}
	if basic.Info()&types.IsString != 0 {
		return "string", nil
	}
	if basic.Info()&types.IsBoolean != 0 {
		return "boolean", nil
	}
	if basic.Info()&types.IsNumeric != 0 {
		return "number", nil
	}
	if basic.Kind() == types.UnsafePointer {
		return "", fmt.Errorf("unsafe.Pointer is not supported")
	}
	return "unknown", nil
}

func basicZodType(basic *types.Basic) (string, error) {
	if basic.Kind() == types.UntypedNil {
		return "z.null()", nil
	}
	if basic.Info()&types.IsString != 0 {
		return "z.string()", nil
	}
	if basic.Info()&types.IsBoolean != 0 {
		return "z.boolean()", nil
	}
	if basic.Info()&types.IsInteger != 0 {
		return "z.number().int()", nil
	}
	if basic.Info()&types.IsFloat != 0 {
		return "z.number()", nil
	}
	if basic.Kind() == types.UnsafePointer {
		return "", fmt.Errorf("unsafe.Pointer is not supported")
	}
	return "z.unknown()", nil
}

func isTime(named *types.Named) bool {
	object := named.Obj()
	return object.Pkg() != nil && object.Pkg().Path() == "time" && object.Name() == "Time"
}

func sortedTypeNames(model *contractModel) []string {
	names := make([]string, 0, len(model.types))
	for name := range model.types {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func quotedUnion(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}
	return strings.Join(quoted, " | ")
}

func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}
	return strings.Join(quoted, ", ")
}

func writeGeneratedDirectory(outputDir string, files map[string]string) error {
	clean := filepath.Clean(outputDir)
	if clean == "." || clean == string(filepath.Separator) || filepath.Base(clean) != "contract" {
		return fmt.Errorf("refusing to replace unsafe output directory %s", outputDir)
	}
	if err := os.RemoveAll(clean); err != nil {
		return err
	}
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if filepath.Base(name) != name {
			return fmt.Errorf("generated filename %s is not a base name", name)
		}
		if err := os.WriteFile(filepath.Join(clean, name), []byte(files[name]), 0o644); err != nil {
			return err
		}
	}
	return nil
}
