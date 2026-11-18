package report_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

// contractFixture is shared with web/lens/src/reports/contract.test.ts so Go and
// TypeScript read the same stored definitions, draft and version cases.
type contractFixture struct {
	Definitions []json.RawMessage `json:"definitions"`
	Composition json.RawMessage   `json:"composition"`
	Versions    []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Opened struct {
			Definition report.Definition       `json:"definition"`
			Source     report.SourceDescriptor `json:"source"`
		} `json:"opened"`
		Latest struct {
			Definition report.Definition       `json:"definition"`
			Source     report.SourceDescriptor `json:"source"`
		} `json:"latest"`
	} `json:"versions"`
}

func loadContractFixture(t *testing.T) contractFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "lens", "fixtures", "report-contract.json"))
	require.NoError(t, err)
	var fixture contractFixture
	require.NoError(t, json.Unmarshal(data, &fixture))
	return fixture
}

// storingHost is a host adapter that owns persistence. The SDK only sees Host.
type storingHost interface {
	report.Host
	put(kind, id string, data []byte) error
	get(kind, id string) ([]byte, error)
}

var errMissing = errors.New("missing")

// keyValueHost stores serialized records in one flat map, as a cache or a KV store would.
type keyValueHost struct {
	records map[string][]byte
	sources map[string]report.Source
}

func (h *keyValueHost) put(kind, id string, data []byte) error {
	h.records[kind+"/"+id] = append([]byte(nil), data...)
	return nil
}
func (h *keyValueHost) get(kind, id string) ([]byte, error) {
	data, ok := h.records[kind+"/"+id]
	if !ok {
		return nil, errMissing
	}
	return append([]byte(nil), data...), nil
}
func (h *keyValueHost) Report(_ context.Context, id string) (report.Definition, error) {
	data, err := h.get("definition", id)
	if err != nil {
		return report.Definition{}, serrors.New(serrors.NotFound, "report missing").WithCause(err)
	}
	return report.DecodeDefinition(data)
}
func (h *keyValueHost) Source(_ context.Context, id string) (report.Source, error) {
	return lookupSource(h.sources, id)
}

// directoryHost stores one file per record, as a document store would.
type directoryHost struct {
	root    string
	sources map[string]report.Source
}

func (h *directoryHost) put(kind, id string, data []byte) error {
	if err := os.MkdirAll(filepath.Join(h.root, kind), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(h.root, kind, id+".json"), data, 0o600)
}
func (h *directoryHost) get(kind, id string) ([]byte, error) {
	return os.ReadFile(filepath.Join(h.root, kind, id+".json"))
}
func (h *directoryHost) Report(_ context.Context, id string) (report.Definition, error) {
	data, err := h.get("definition", id)
	if err != nil {
		return report.Definition{}, serrors.New(serrors.NotFound, "report missing").WithCause(err)
	}
	return report.DecodeDefinition(data)
}
func (h *directoryHost) Source(_ context.Context, id string) (report.Source, error) {
	return lookupSource(h.sources, id)
}

func lookupSource(sources map[string]report.Source, id string) (report.Source, error) {
	source, ok := sources[id]
	if !ok {
		return nil, serrors.New(serrors.NotFound, "source missing")
	}
	return source, nil
}

func saveDefinition(t *testing.T, host storingHost, definition report.Definition) {
	t.Helper()
	data, err := json.Marshal(definition)
	require.NoError(t, err)
	require.NoError(t, host.put("definition", definition.ID, data))
}

func saveComposition(t *testing.T, host storingHost, composition report.Composition) {
	t.Helper()
	data, err := json.Marshal(composition)
	require.NoError(t, err)
	require.NoError(t, host.put("composition", composition.ID, data))
}

func restoreComposition(t *testing.T, host storingHost, id string) report.Composition {
	t.Helper()
	data, err := host.get("composition", id)
	require.NoError(t, err)
	composition, err := report.DecodeComposition(data)
	require.NoError(t, err)
	return composition
}

// acceptanceSources are synthetic: a single-figure report and a compound block whose
// stat and table share one dataset and one control.
func acceptanceSources() map[string]report.Source {
	amount := report.SourceFunc{
		Metadata: report.SourceDescriptor{ID: "sales", Version: 1, Revision: "v1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}, Save: true, Export: true}},
		BuildSpec: func(_ context.Context, _ report.Definition, params report.Parameters) (lens.DashboardSpec, error) {
			var period float64
			if err := json.Unmarshal(params["period"], &period); err != nil {
				return lens.DashboardSpec{}, err
			}
			builder := frame.NewBuilder("values").Number("amount", frame.RoleMetric)
			if err := builder.Append(frame.Row{"amount": period}); err != nil {
				return lens.DashboardSpec{}, err
			}
			data, err := builder.FrameSet()
			if err != nil {
				return lens.DashboardSpec{}, err
			}
			return lens.DashboardSpec{ID: "amount", Title: "Amount",
				Datasets: []lens.DatasetSpec{{Name: "values", Kind: lens.DatasetKindStatic, Static: data}},
				Rows:     []lens.RowSpec{{Panels: []panel.Spec{panel.Stat("amount", "Amount", "values").ValueField("amount").Terminal().Build()}}},
			}, nil
		},
	}
	orders := report.SourceFunc{
		Metadata: report.SourceDescriptor{ID: "orders", Version: 1, Revision: "v1", Capabilities: report.Capabilities{Filters: []string{"region"}, Views: []string{"overview"}, Drill: true}},
		BuildSpec: func(_ context.Context, _ report.Definition, params report.Parameters) (lens.DashboardSpec, error) {
			var region string
			if err := json.Unmarshal(params["region"], &region); err != nil {
				return lens.DashboardSpec{}, err
			}
			builder := frame.NewBuilder("orders").String("region", frame.RoleDimension).Number("orders", frame.RoleMetric)
			for _, row := range []frame.Row{{"region": region, "orders": float64(3)}, {"region": region, "orders": float64(4)}} {
				if err := builder.Append(row); err != nil {
					return lens.DashboardSpec{}, err
				}
			}
			data, err := builder.FrameSet()
			if err != nil {
				return lens.DashboardSpec{}, err
			}
			return lens.DashboardSpec{ID: "overview", Title: "Overview",
				Variables: []lens.VariableSpec{{Name: "region", Label: "Region", Kind: lens.VariableText, RequestKeys: []string{"region"}, Default: region, Bound: true}},
				Datasets:  []lens.DatasetSpec{{Name: "orders", Kind: lens.DatasetKindStatic, Static: data}},
				Rows: []lens.RowSpec{{Panels: []panel.Spec{
					panel.Stat("total", "Orders", "orders").ValueField("orders").Span(4).Terminal().Build(),
					panel.Table("rows", "By region", "orders").IDField("region").Columns(panel.TableColumn{Field: "region", Label: "Region"}, panel.TableColumn{Field: "orders", Label: "Orders"}).Span(8).Terminal().Build(),
				}}},
			}, nil
		},
	}
	return map[string]report.Source{"sales": amount, "orders": orders}
}

func openBoard(t *testing.T, host storingHost, id string) (*document.DashboardDocument, *lensruntime.Result, lens.DashboardSpec) {
	t.Helper()
	spec, _, err := report.Compose(context.Background(), restoreComposition(t, host, id), host)
	require.NoError(t, err)
	// A fresh runtime with no snapshot store: the stored definitions alone must recompute the board.
	result, err := lensruntime.New(lensruntime.Options{}).Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "test"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "acceptance", GeneratedAt: time.Unix(1, 0), Locale: "en", InlineDepth: 10})
	require.NoError(t, err)
	require.NoError(t, doc.Validate())
	return doc, result, spec
}

func groupLabels(doc *document.DashboardDocument) map[string]string {
	labels := map[string]string{}
	for _, row := range doc.Layout.Rows {
		for _, item := range row.Panels {
			for _, group := range item.Groups {
				labels[group.ID] = group.Label
			}
		}
	}
	return labels
}

// False green: a single unsaved object placed once passes any identity check. Every
// assertion below runs on records that went through both adapters' storage and back.
func TestReusableReportContractAcrossTwoHostAdapters(t *testing.T) {
	t.Parallel()
	fixture := loadContractFixture(t)
	adapters := map[string]func(t *testing.T) storingHost{
		"key-value": func(*testing.T) storingHost {
			return &keyValueHost{records: map[string][]byte{}, sources: acceptanceSources()}
		},
		"directory": func(t *testing.T) storingHost {
			return &directoryHost{root: t.TempDir(), sources: acceptanceSources()}
		},
	}
	restored := map[string][]byte{}
	for name, open := range adapters {
		t.Run(name, func(t *testing.T) {
			host := open(t)
			for _, raw := range fixture.Definitions {
				definition, err := report.DecodeDefinition(raw)
				require.NoError(t, err)
				saveDefinition(t, host, definition)
			}
			composition, err := report.DecodeComposition(fixture.Composition)
			require.NoError(t, err)
			saveComposition(t, host, composition)

			// Round trip: the restored composition is exactly what was stored.
			back := restoreComposition(t, host, "board")
			require.Equal(t, composition, back)
			canonical, err := json.Marshal(back)
			require.NoError(t, err)
			restored[name] = canonical

			// One report R in placements A and B; A owns its caption, width and period.
			doc, result, spec := openBoard(t, host, "board")
			labels := groupLabels(doc)
			require.Equal(t, "June revenue", labels["a"])
			require.Equal(t, "Revenue", labels["b"])
			require.Equal(t, 6.0, result.Panels["a::panel:amount"].Frames.Primary().Fields[0].Values[0])
			require.Equal(t, 10.0, result.Panels["b::panel:amount"].Frames.Primary().Fields[0].Values[0])
			stored, err := host.Report(context.Background(), "revenue")
			require.NoError(t, err)
			require.JSONEq(t, `10`, string(stored.Parameters["period"]), "A's period never writes into R")

			// The compound block stays one named group whose panels keep sharing its dataset and control.
			require.Equal(t, "Sales overview", labels["overview"])
			members := 0
			for _, row := range doc.Layout.Rows {
				for _, item := range row.Panels {
					if len(item.Groups) > 0 && item.Groups[0].ID == "overview" {
						members++
					}
				}
			}
			require.Equal(t, 2, members, "stat and table stay inside the overview block")
			datasets := map[string]bool{}
			for _, dataset := range spec.Datasets {
				datasets[dataset.Name] = true
			}
			require.True(t, datasets["overview::orders"])
			variables := []string{}
			for _, variable := range spec.Variables {
				variables = append(variables, variable.Name)
			}
			require.Contains(t, variables, "overview::region")
			require.Equal(t, 2, result.Panels["overview::panel:total"].Frames.Primary().RowCount)
			require.Equal(t, 2, result.Panels["overview::panel:rows"].Frames.Primary().RowCount)

			// Renaming R reaches B but never replaces A's own caption.
			stored.Title, stored.Revision = "Net revenue", "r2"
			saveDefinition(t, host, stored)
			renamed, _, _ := openBoard(t, host, "board")
			labels = groupLabels(renamed)
			require.Equal(t, "June revenue", labels["a"])
			require.Equal(t, "Net revenue", labels["b"])

			// Duplicating a placement keeps the reference; copying the definition makes a new report.
			duplicate, err := report.DuplicatePlacement(back.Rows[0][0], "a-copy")
			require.NoError(t, err)
			require.Equal(t, "revenue", duplicate.ReportID)
			independent, err := report.CopyDefinition(stored, "revenue-copy", "c1")
			require.NoError(t, err)
			saveDefinition(t, host, independent)
			stored.Title, stored.Revision = "Gross revenue", "r3"
			saveDefinition(t, host, stored)
			copied, err := host.Report(context.Background(), "revenue-copy")
			require.NoError(t, err)
			require.Equal(t, "Net revenue", copied.Title, "an independent copy does not follow R")
			require.NotEqual(t, stored.ID, copied.ID)

			// Unsupported inputs and snapshot references are rejected, never ignored.
			_, err = report.Resolve(context.Background(), host, report.Placement{ID: "x", ReportID: "revenue", Parameters: report.Parameters{"region": json.RawMessage(`"north"`)}}, nil)
			require.True(t, serrors.HasCode(err, serrors.FailedPrecondition))
			_, err = report.DecodeComposition([]byte(`{"id":"board","revision":"1","title":"Board","snapshotId":"expired","rows":[]}`))
			require.True(t, serrors.HasCode(err, serrors.Invalid))
			_, err = report.DecodeComposition([]byte(`{"id":"board","revision":"1","title":"Board","rows":[[{"id":"a","reportId":"revenue","snapshotId":"expired"}]]}`))
			require.True(t, serrors.HasCode(err, serrors.Invalid))
		})
	}
	require.Len(t, restored, len(adapters))
	require.JSONEq(t, string(restored["key-value"]), string(restored["directory"]), "both adapters restore identical compositions")
}

// False green: hand-written expectations that drift from the TypeScript mirror. The same
// fixture drives compareReportVersion in web/lens/src/reports/contract.test.ts.
func TestCompareVersionMatchesSharedFixture(t *testing.T) {
	t.Parallel()
	fixture := loadContractFixture(t)
	require.NotEmpty(t, fixture.Versions)
	for _, version := range fixture.Versions {
		status := report.CompareVersion(version.Opened.Definition, version.Opened.Source, version.Latest.Definition, version.Latest.Source)
		require.Equal(t, report.VersionStatus(version.Status), status, version.Name)
	}
}
