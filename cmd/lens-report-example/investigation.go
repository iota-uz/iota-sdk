package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

type branchRow struct {
	id, label      string
	profit, income float64
}

// Synthetic branch margins: region → branch → host records. Each drill level
// declares the report dimension its selection constrains, so the slice can be
// saved as a report; the margin is a ratio of host-computed parts.
var investigationRegions = []struct {
	id, label string
	branches  []branchRow
}{
	{"north", "North", []branchRow{{"chilanzar", "Chilanzar", 182_000, 1_040_000}, {"yunusabad", "Yunusabad", 96_000, 640_000}}},
	{"south", "South", []branchRow{{"karshi", "Karshi", 74_000, 520_000}, {"termez", "Termez", 31_000, 410_000}}},
}

func marginFrame(name string, rows []branchRow) (*frame.Frame, error) {
	builder := frame.NewBuilder(name).
		String("id", frame.RoleDimension).String("label", frame.RoleDimension).
		Number("margin", frame.RoleMetric).Number("profit", frame.RoleMetric).Number("revenue", frame.RoleMetric)
	for _, row := range rows {
		if err := builder.Append(frame.Row{"id": row.id, "label": row.label, "margin": 100 * row.profit / row.income, "profit": row.profit, "revenue": row.income}); err != nil {
			return nil, err
		}
	}
	return builder.Frame()
}

func marginPanel(id, title, dataset string) *panel.Builder {
	return panel.HorizontalBar(id, title, dataset).IDField("id").LabelField("label").ValueField("margin").
		Format(format.Percent(1)).
		Ratio(panel.Ratio{
			NumeratorField: "profit", NumeratorLabel: "Profit",
			DenominatorField: "revenue", DenominatorLabel: "Revenue",
			Description: "Profit divided by revenue of the same branches; region margins are recalculated from the sums, never averaged.",
		})
}

func buildInvestigation(context.Context, report.Definition, report.Parameters, filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
	regions := make([]branchRow, 0, len(investigationRegions))
	datasets := make([]*frame.Frame, 0, len(investigationRegions)+1)
	rootEdges := make([]explore.Edge, 0, len(investigationRegions))
	nodes := make([]explore.Node, 0, len(investigationRegions)+1)
	for _, region := range investigationRegions {
		var profit, income float64
		edges := make([]explore.Edge, 0, len(region.branches))
		for _, branch := range region.branches {
			profit += branch.profit
			income += branch.income
			edges = append(edges, explore.ToAction(branch.id, action.Navigate("/demo/records?branch="+branch.id)))
		}
		regions = append(regions, branchRow{region.id, region.label, profit, income})
		branches, err := marginFrame("branches-"+region.id, region.branches)
		if err != nil {
			return lens.DashboardSpec{}, err
		}
		datasets = append(datasets, branches)
		rootEdges = append(rootEdges, explore.ToNode(region.id, region.id))
		nodes = append(nodes, explore.PanelNode(region.id, region.label, marginPanel("branches-"+region.id, region.label, "branches-"+region.id).Build(), edges...).WithDimension("branch"))
	}
	root, err := marginFrame("regions", regions)
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	datasets = append([]*frame.Frame{root}, datasets...)
	nodes = append([]explore.Node{explore.PanelNode("regions", "Regions", marginPanel("regions-level", "Regions", "regions").Build(), rootEdges...).WithDimension("region")}, nodes...)
	explorer, err := explore.New("margin", "margin",
		explore.NewBranch("by-region", "By region", "composition",
			explore.NewPerspective("composition", "Composition", explore.SemanticsPartition, "regions", nodes...),
		),
	).Build()
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	specs := make([]lens.DatasetSpec, 0, len(datasets))
	for _, data := range datasets {
		set, setErr := frame.NewFrameSet(data)
		if setErr != nil {
			return lens.DashboardSpec{}, setErr
		}
		specs = append(specs, lens.DatasetSpec{Name: data.Name, Kind: lens.DatasetKindStatic, Static: set})
	}
	return lens.DashboardSpec{ID: "margin", Title: "Branch margin", Datasets: specs, Explorers: []explore.Spec{explorer}, Rows: []lens.RowSpec{
		{Panels: []panel.Spec{marginPanel("margin", "Margin by region", "regions").Build()}},
	}}, nil
}

// investigationExample emits a composed report whose drill can be saved.
func investigationExample() error {
	ctx := context.Background()
	calendar := filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2026-07-22T12:00:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Days: 30}}
	june := filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-06-01", End: "2026-06-30"}
	definition := report.Definition{ID: "branch-margin", Revision: "3", Title: "Branch margin", Source: report.SourceRef{ID: "margin", Version: 1}, View: "summary"}
	source := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "margin", Version: 1, Revision: "1", Capabilities: report.Capabilities{
		Filters: []string{"period"}, Views: []string{"summary"}, Dimensions: []string{"segment", "region", "branch"}, Drill: true, Save: true, Export: true,
	}}, BuildContextSpec: buildInvestigation}
	board := filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &june, Constraints: []filtercontext.ContextConstraint{{Dimension: "segment", Values: []string{"retail"}}}}}
	composition := report.Composition{ID: "finance-overview", Revision: "7", Title: "Finance overview", Context: &board, Rows: [][]report.Placement{{{ID: "margin", ReportID: definition.ID, Span: 12}}}}
	spec, _, err := report.Compose(ctx, composition, report.Catalog{Definitions: []report.Definition{definition}, Sources: []report.Source{source}})
	if err != nil {
		return err
	}
	result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, spec, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
	if err != nil {
		return err
	}
	doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "investigation", Locale: "en", GeneratedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC), InlineDepth: 10})
	if err != nil {
		return err
	}
	doc.ContextEditor = &filtercontext.ContextEditor{URL: "/", Value: board, Saved: board, ReadOnly: true,
		DimensionLabels: map[string]string{"segment": "Segment", "region": "Region", "branch": "Branch"},
		ValueLabels:     map[string]map[string]string{"segment": {"retail": "Retail"}}}
	doc.Endpoints.Query = "/demo/lens/query"
	if err = doc.Validate(); err != nil {
		return err
	}
	levels, err := investigationLevelFrames(ctx, doc)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	return encoder.Encode(struct {
		Definition  report.Definition           `json:"definition"`
		Composition report.Composition          `json:"composition"`
		Source      report.SourceDescriptor     `json:"source"`
		Document    *document.DashboardDocument `json:"document"`
		Levels      map[string]document.Frame   `json:"levels"`
	}{definition, composition, source.Metadata, doc, levels})
}

// investigationLevelFrames answers the story's query endpoint: each drill
// level's frame, built by the production document builder from the same
// node panels the level declares, keyed by the level's wire key.
func investigationLevelFrames(ctx context.Context, doc *document.DashboardDocument) (map[string]document.Frame, error) {
	spec, err := buildInvestigation(ctx, report.Definition{}, nil, filtercontext.EffectiveContext{})
	if err != nil {
		return nil, err
	}
	nodes := spec.Explorers[0].Branches[0].Perspectives[0].Nodes
	rows := make([]lens.RowSpec, 0, len(nodes))
	for _, node := range nodes {
		// The flat board only serializes the frames; its panels act nowhere.
		flatPanel := *node.Panel
		flatPanel.Terminal = true
		rows = append(rows, lens.RowSpec{Panels: []panel.Spec{flatPanel}})
	}
	flat := lens.DashboardSpec{ID: "levels", Title: "Levels", Datasets: spec.Datasets, Rows: rows}
	result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, flat, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
	if err != nil {
		return nil, err
	}
	builtDoc, err := document.Build(flat, result, document.BuildOptions{SnapshotID: "levels", Locale: "en", GeneratedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		return nil, err
	}
	levels := make(map[string]document.Frame, len(nodes))
	for _, node := range nodes {
		for _, wirePanel := range builtDoc.Panels {
			if wirePanel.ID != node.Panel.ID {
				continue
			}
			key := "margin::margin/by-region/composition/" + node.Key
			if _, ok := doc.Drill.Edges[document.NodeKey(key)]; !ok {
				return nil, fmt.Errorf("level %s is not in the document", key)
			}
			levels[key] = builtDoc.Frames[wirePanel.Frame]
		}
	}
	return levels, nil
}
