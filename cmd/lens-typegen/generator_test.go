package main

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRepresentativeContract(t *testing.T) {
	t.Parallel()

	files, err := generate(config{
		dir:             ".",
		packagePattern:  "./testdata/fixture",
		rootType:        "FixtureDocument",
		additionalTypes: []string{"FixtureResponse"},
		versionConstant: "ContractVersion",
		palette:         fixturePalette(),
	})
	require.NoError(t, err)
	require.Equal(t, []string{"index.ts", "palette.ts", "schemas.ts", "types.ts"}, sortedFileNames(files))

	typesFile := files["types.ts"]
	require.Contains(t, typesFile, `export const CONTRACT_VERSION = "3.2.1"`)
	require.Contains(t, typesFile, `export type FixtureKind = "alpha" | "zeta"`)
	require.Contains(t, typesFile, `export type FixtureErrorCode = "NOT_FOUND" | "UNAVAILABLE"`)
	require.Contains(t, typesFile, "nested: Nested")
	require.Contains(t, typesFile, "lookup: Record<string, Nested>")
	require.Contains(t, typesFile, "optional?: string")
	require.Contains(t, typesFile, "maybe?: Nested")
	require.Contains(t, typesFile, "createdAt: string")
	require.Contains(t, typesFile, "export interface FixtureResponse")

	schemasFile := files["schemas.ts"]
	require.Contains(t, schemasFile, "const CONTRACT_MAJOR_VERSION = CONTRACT_VERSION.split('.', 1)[0]!")
	require.Contains(t, schemasFile, "return version.split('.', 1)[0]!")
	require.NotContains(t, schemasFile, "CONTRACT_VERSION.split('.', 1)[0] ?? CONTRACT_VERSION")
	require.NotContains(t, schemasFile, "version.split('.', 1)[0] ?? version")
	require.Contains(t, schemasFile, "z.record(z.string(), z.lazy(() => NestedSchema))")
	require.Contains(t, schemasFile, "optional: z.string().optional()")
	require.Contains(t, schemasFile, "createdAt: z.string().datetime({ offset: true })")
	require.Contains(t, schemasFile, "count: z.number().int()")
	require.Contains(t, schemasFile, "payload: z.unknown()")
	require.Contains(t, schemasFile, "export const FixtureResponseSchema")
	require.Contains(t, schemasFile, "export const FixtureDocumentSchema: z.ZodType<Contract.FixtureDocument> = z.lazy(() => z.object(")
	require.Contains(t, schemasFile, "export const FixtureKindSchema: z.ZodType<Contract.FixtureKind> = z.enum(")
	require.Contains(t, schemasFile, "export const FixtureResponseSchema: z.ZodType<Contract.FixtureResponse> = z.lazy(() => z.object(")
	require.Contains(t, schemasFile, "export const NestedSchema: z.ZodType<Contract.Nested> = z.object(")
	require.NotContains(t, schemasFile, "z.any()")

	require.Contains(t, files["index.ts"], "export * from './palette'")
}

// The runtime paints with the generated palette, so the generator owes it the
// declared order, the values themselves, and a form CSS can use — the Go
// literals are uppercase and the emitted ones must not be, or every run would
// look like drift against the committed file.
func TestGeneratePaletteCarriesGoValuesInOrder(t *testing.T) {
	t.Parallel()

	files, err := generate(config{
		dir:             ".",
		packagePattern:  "./testdata/fixture",
		rootType:        "FixtureDocument",
		versionConstant: "ContractVersion",
		palette:         fixturePalette(),
	})
	require.NoError(t, err)

	paletteFile := files["palette.ts"]
	require.Contains(t, paletteFile, "export const PALETTE_SERIES = [\n  '#2563eb',\n  '#0d9488',\n] as const")
	require.Contains(t, paletteFile, "export const PALETTE_NEUTRAL = '#94a3b8'")
	require.NotContains(t, paletteFile, "#2563EB")
}

// A palette that cannot be painted must stop generation rather than reach the
// runtime as an unusable string.
func TestGenerateRejectsUnusablePalette(t *testing.T) {
	t.Parallel()

	cases := map[string]paletteConfig{
		"no series":      {series: nil, neutral: "#94A3B8"},
		"malformed hex":  {series: []string{"#2563EB", "blue"}, neutral: "#94A3B8"},
		"absent neutral": {series: []string{"#2563EB"}, neutral: ""},
	}
	for name, palette := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := generate(config{
				dir:             ".",
				packagePattern:  "./testdata/fixture",
				rootType:        "FixtureDocument",
				versionConstant: "ContractVersion",
				palette:         palette,
			})
			require.Error(t, err)
		})
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()

	cfg := config{
		dir:             ".",
		packagePattern:  "./testdata/fixture",
		rootType:        "FixtureDocument",
		versionConstant: "ContractVersion",
		palette:         fixturePalette(),
	}
	first, err := generate(cfg)
	require.NoError(t, err)
	second, err := generate(cfg)
	require.NoError(t, err)
	require.Equal(t, first, second)

	outputDir := filepath.Join(t.TempDir(), "contract")
	require.NoError(t, writeGeneratedDirectory(outputDir, first))
	require.NoError(t, writeGeneratedDirectory(outputDir, second))
}

func fixturePalette() paletteConfig {
	return paletteConfig{series: []string{"#2563EB", "#0D9488"}, neutral: "#94A3B8"}
}

func sortedFileNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
