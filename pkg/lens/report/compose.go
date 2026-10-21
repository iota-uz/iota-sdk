package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"reflect"
	"strconv"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

// Compose lowers report placements into the existing execution graph.
// Sources bind their effective parameters before returning a spec.
func Compose(ctx context.Context, c Composition, host Host) (lens.DashboardSpec, []Resolved, error) {
	if err := c.Validate(); err != nil {
		return lens.DashboardSpec{}, nil, err
	}
	result := lens.DashboardSpec{ID: c.ID, Title: c.Title}
	resolved := []Resolved{}
	for _, row := range c.Rows {
		panels := []panel.Spec{}
		for _, placement := range row {
			var input *filtercontext.ContextInput
			if c.Context != nil {
				card := placement.Filters
				if current, exists := c.Context.Cards[placement.ID]; exists {
					card = current
				}
				layers := []filtercontext.ContextLayer{
					{Origin: filtercontext.ContextOrigin{Scope: "dashboard", ID: c.ID}, Filters: c.Context.Dashboard},
					{Origin: filtercontext.ContextOrigin{Scope: "card", ID: placement.ID}, Filters: card},
				}
				if placement.SectionID != "" {
					layers = append(layers, filtercontext.ContextLayer{Origin: filtercontext.ContextOrigin{Scope: "section", ID: placement.SectionID}, Filters: c.Context.Sections[placement.SectionID]})
				}
				layers = append(layers, c.Context.Groups[placement.ID]...)
				input = &filtercontext.ContextInput{Calendar: c.Context.Calendar, Layers: layers}
			}
			block, err := resolve(ctx, host, placement, c.Parameters, input)
			if errors.Is(err, ErrIncompatible) {
				block.Unavailable = document.GroupUnavailableReportIncompatible
			}
			if block.Unavailable != "" {
				container, datasets, unavailableErr := unavailablePlacement(block)
				if unavailableErr != nil {
					return lens.DashboardSpec{}, nil, unavailableErr
				}
				panels = append(panels, container)
				result.Datasets = append(result.Datasets, datasets...)
				resolved = append(resolved, block)
				continue
			}
			if err != nil {
				return lens.DashboardSpec{}, nil, err
			}
			scoped := namespace(block.Spec, placement.ID)
			if block.Context != nil {
				name := scopedID(placement.ID, "filter-context")
				data, contextErr := contextEvidence(placement.ID, *block.Context)
				if contextErr != nil {
					return lens.DashboardSpec{}, nil, contextErr
				}
				scoped.Datasets = append(scoped.Datasets, lens.DatasetSpec{Name: name, Title: "Report filter context", Kind: lens.DatasetKindStatic, Static: data, Export: exportmeta.Spec{Enabled: true, Lineage: true, SheetName: "Scope " + placement.ID, TableName: "ctx_" + strings.ReplaceAll(placement.ID, "-", "_"), FreezeHeader: true}})
				for i := range scoped.Rows {
					attachContextEvidence(scoped.Rows[i].Panels, name)
				}
				result.Export.EvidenceDatasets = append(result.Export.EvidenceDatasets, name)
			}
			children := []panel.Spec{}
			for i, sourceRow := range scoped.Rows {
				if sourceRow.Heading != "" {
					return lens.DashboardSpec{}, nil, invalid("report blocks use titled panels rather than document heading rows")
				}
				if len(sourceRow.Panels) > 0 {
					children = append(children, panel.Grid(scopedID(placement.ID, "row-"+strconv.Itoa(i)), "", sourceRow.Panels...).Span(12).Build())
				}
			}
			container := panel.Grid(placement.ID, placementTitle(block), children...).Span(placementSpan(placement)).Build()
			container.FilterContext = block.Context
			panels = append(panels, container)
			result.Datasets = append(result.Datasets, scoped.Datasets...)
			result.Variables = append(result.Variables, scoped.Variables...)
			result.Explorers = append(result.Explorers, scoped.Explorers...)
			resolved = append(resolved, block)
		}
		result.Rows = append(result.Rows, lens.RowSpec{Panels: panels})
	}
	type identity struct {
		Definition Definition       `json:"definition"`
		Source     SourceDescriptor `json:"source"`
		Parameters Parameters       `json:"parameters"`
	}
	identities := make([]identity, 0, len(resolved))
	for _, block := range resolved {
		identities = append(identities, identity{block.Definition, block.Source, block.Parameters})
	}
	payload, err := json.Marshal(struct {
		Composition Composition `json:"composition"`
		Reports     []identity  `json:"reports"`
	}{c, identities})
	if err != nil {
		return lens.DashboardSpec{}, nil, err
	}
	digest := sha256.Sum256(payload)
	result.Revision = hex.EncodeToString(digest[:])
	return result, resolved, nil
}

func scopedID(scope, id string) string {
	if id == "" {
		return ""
	}
	return scope + "::" + id
}

func namespace(input lens.DashboardSpec, scope string) lens.DashboardSpec {
	out := clone(input)
	datasets, variables, explorers := map[string]string{}, map[string]string{}, map[string]string{}
	for _, dataset := range out.Datasets {
		datasets[dataset.Name] = scopedID(scope, dataset.Name)
	}
	for _, variable := range out.Variables {
		variables[variable.Name] = scopedID(scope, variable.Name)
	}
	for _, explorer := range out.Explorers {
		explorers[explorer.ID] = scopedID(scope, explorer.ID)
	}
	rename := func(value string, keys map[string]string) string {
		if name, ok := keys[value]; ok {
			return name
		}
		return value
	}
	export := func(spec *exportmeta.Spec) {
		for i, name := range spec.EvidenceDatasets {
			spec.EvidenceDatasets[i] = rename(name, datasets)
		}
	}
	transforms := func(items []transform.Spec) {
		for i := range items {
			if items[i].Join != nil {
				items[i].Join.Other = rename(items[i].Join.Other, datasets)
			}
			if items[i].Lookup != nil {
				items[i].Lookup.Other = rename(items[i].Lookup.Other, datasets)
			}
			if items[i].TopN != nil {
				items[i].TopN.RankByDataset = rename(items[i].TopN.RankByDataset, datasets)
			}
		}
	}
	value := func(source *action.ValueSource) {
		if source != nil && source.Kind == action.SourceVariable {
			source.Name = rename(source.Name, variables)
		}
	}
	rewriteAction := func(spec *action.Spec) {
		if spec == nil {
			return
		}
		value(spec.URLSource)
		value(spec.DrawerKey)
		for i := range spec.Params {
			value(&spec.Params[i].Source)
		}
		for key, source := range spec.Payload {
			value(&source)
			spec.Payload[key] = source
		}
		if spec.Drill != nil {
			value(&spec.Drill.Value)
		}
		if spec.Explore != nil {
			spec.Explore.ExplorerID = rename(spec.Explore.ExplorerID, explorers)
			value(&spec.Explore.Branch)
		}
	}
	var rewritePanel func(*panel.Spec)
	rewritePanel = func(item *panel.Spec) {
		item.ID = scopedID(scope, "panel:"+item.ID)
		item.Dataset = rename(item.Dataset, datasets)
		export(&item.Export)
		transforms(item.Transforms)
		rewriteAction(item.Action)
		for i := range item.Columns {
			rewriteAction(item.Columns[i].Action)
		}
		for i := range item.FlowStages {
			rewriteAction(item.FlowStages[i].Action)
		}
		for i := range item.HierarchyRows {
			rewriteAction(item.HierarchyRows[i].Action)
		}
		for i := range item.Children {
			rewritePanel(&item.Children[i])
		}
	}
	for i := range out.Datasets {
		dataset := &out.Datasets[i]
		dataset.Name = rename(dataset.Name, datasets)
		for j, name := range dataset.DependsOn {
			dataset.DependsOn[j] = rename(name, datasets)
		}
		dataset.TimeRangeVariable = rename(dataset.TimeRangeVariable, variables)
		export(&dataset.Export)
		transforms(dataset.Transforms)
		if dataset.Query != nil {
			for key, param := range dataset.Query.Params {
				param.Variable = rename(param.Variable, variables)
				dataset.Query.Params[key] = param
			}
		}
	}
	for i := range out.Variables {
		variable := &out.Variables[i]
		variable.Name = rename(variable.Name, variables)
		variable.CompareTo = rename(variable.CompareTo, variables)
		for j, key := range variable.RequestKeys {
			variable.RequestKeys[j] = scopedID(scope, key)
		}
	}
	for i := range out.Rows {
		for j := range out.Rows[i].Panels {
			rewritePanel(&out.Rows[i].Panels[j])
		}
	}
	for i := range out.Explorers {
		explorer := &out.Explorers[i]
		explorer.ID = rename(explorer.ID, explorers)
		explorer.HostPanelID = scopedID(scope, "panel:"+explorer.HostPanelID)
		for j := range explorer.Branches {
			for k := range explorer.Branches[j].Perspectives {
				perspective := &explorer.Branches[j].Perspectives[k]
				export(&perspective.Export)
				for n := range perspective.Nodes {
					node := &perspective.Nodes[n]
					if node.Panel != nil {
						rewritePanel(node.Panel)
					}
					if node.SourceData != nil {
						rewritePanel(&node.SourceData.Panel)
					}
					for e := range node.Edges {
						rewriteAction(node.Edges[e].Action)
					}
					if node.DynamicChildren != nil {
						value(&node.DynamicChildren.Key)
						value(&node.DynamicChildren.Label)
						value(node.DynamicChildren.Target)
						rewriteAction(node.DynamicChildren.Action)
					}
				}
			}
		}
	}
	return out
}

// clone retains concrete parameter types (dates, UUIDs, frames) without a JSON round trip.
func clone[T any](input T) T { return copyValue(reflect.ValueOf(input)).Interface().(T) }

func copyValue(value reflect.Value) reflect.Value {
	//nolint:exhaustive // Scalar and immutable value kinds retain their original representation.
	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(copyValue(value.Elem()))
		return out
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(copyValue(value.Elem()))
		return out
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(copyValue(value.Index(i)))
		}
		return out
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), copyValue(iter.Value()))
		}
		return out
	case reflect.Struct:
		out := reflect.New(value.Type()).Elem()
		out.Set(value)
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				out.Field(i).Set(copyValue(value.Field(i)))
			}
		}
		return out
	default:
		return value
	}
}

// The export reads the same frozen identity/range/restrictions as the wire group.
func contextEvidence(card string, effective filtercontext.EffectiveContext) (*frame.FrameSet, error) {
	builder := frame.NewBuilder("context").String("card", frame.RoleDimension).String("identity", frame.RoleDimension).String("kind", frame.RoleDimension).String("scope", frame.RoleDimension).String("origin", frame.RoleDimension).String("dimension", frame.RoleDimension).String("value", frame.RoleDimension)
	appendRow := func(kind, scope, origin, dimension, value string) error {
		return builder.Append(frame.Row{"card": card, "identity": effective.Identity, "kind": kind, "scope": scope, "origin": origin, "dimension": dimension, "value": value})
	}
	selection, err := json.Marshal(effective.Period.Selection)
	if err != nil {
		return nil, err
	}
	if err := appendRow("period", effective.Period.Origin.Scope, effective.Period.Origin.ID, "selection", string(selection)); err != nil {
		return nil, err
	}
	for _, entry := range []struct{ dimension, value string }{{"start", effective.Period.Start}, {"end", effective.Period.End}, {"timezone", effective.Calendar.Timezone}, {"asOf", effective.Calendar.AsOf}} {
		if err := appendRow("calendar", effective.Period.Origin.Scope, effective.Period.Origin.ID, entry.dimension, entry.value); err != nil {
			return nil, err
		}
	}
	for _, restriction := range effective.Restrictions {
		for _, value := range restriction.Values {
			if err := appendRow("restriction", restriction.Origin.Scope, restriction.Origin.ID, restriction.Dimension, value); err != nil {
				return nil, err
			}
		}
	}
	for _, dimension := range effective.Conflicts {
		if err := appendRow("conflict", "card", card, dimension, ""); err != nil {
			return nil, err
		}
	}
	for _, restriction := range effective.Unsupported {
		if err := appendRow("unsupported", restriction.Origin.Scope, restriction.Origin.ID, restriction.Dimension, strings.Join(restriction.Values, ", ")); err != nil {
			return nil, err
		}
	}
	return builder.FrameSet()
}
func attachContextEvidence(panels []panel.Spec, name string) {
	for i := range panels {
		panels[i].Export.EvidenceDatasets = append(panels[i].Export.EvidenceDatasets, name)
		attachContextEvidence(panels[i].Children, name)
	}
}

func placementTitle(block Resolved) string {
	if block.Placement.Title != nil {
		return *block.Placement.Title
	}
	return block.Definition.Title
}

func placementSpan(placement Placement) int {
	if placement.Span == 0 {
		return 12
	}
	return placement.Span
}

// unavailablePlacement keeps an unavailable placement's identity and width
// without executing its source. Its single member reads an empty static frame,
// so neither the runtime, print nor export has figures to show. A forbidden
// placement also drops its caption: the slot must not reveal what it hides.
func unavailablePlacement(block Resolved) (panel.Spec, []lens.DatasetSpec, error) {
	data, err := frame.NewBuilder("unavailable").String("reason", frame.RoleDimension).FrameSet()
	if err != nil {
		return panel.Spec{}, nil, err
	}
	id := block.Placement.ID
	name := scopedID(id, "unavailable")
	member := panel.Table(scopedID(id, "panel:unavailable"), "", name).IDField("reason").Columns(panel.TableColumn{Field: "reason", Label: "Reason"}).Terminal().Build()
	title := ""
	if block.Unavailable != document.GroupUnavailableReportForbidden {
		title = placementTitle(block)
	}
	container := panel.Grid(id, title, member).Span(placementSpan(block.Placement)).Build()
	container.Unavailable = string(block.Unavailable)
	return container, []lens.DatasetSpec{{Name: name, Kind: lens.DatasetKindStatic, Static: data}}, nil
}
