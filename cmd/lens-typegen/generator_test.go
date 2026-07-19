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
	})
	require.NoError(t, err)
	require.Equal(t, []string{"index.ts", "schemas.ts", "types.ts"}, sortedFileNames(files))

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
}

func TestGenerateIsDeterministic(t *testing.T) {
	t.Parallel()

	cfg := config{
		dir:             ".",
		packagePattern:  "./testdata/fixture",
		rootType:        "FixtureDocument",
		versionConstant: "ContractVersion",
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

func sortedFileNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
