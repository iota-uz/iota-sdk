package serve

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

func wireFrame(ref document.FrameRef, result *lensruntime.PanelResult) (document.Frame, error) {
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil {
		return document.Frame{}, fmt.Errorf("frame %q has no primary frame", ref)
	}
	source := result.Frames.Primary()
	wire := document.Frame{Columns: make([]document.Column, 0, len(source.Fields)), Rows: make([][]any, source.RowCount)}
	for _, field := range source.Fields {
		kind, err := wireColumnType(field.Type)
		if err != nil {
			return document.Frame{}, fmt.Errorf("frame %q field %s: %w", ref, field.Name, err)
		}
		wire.Columns = append(wire.Columns, document.Column{Name: field.Name, Type: kind})
	}
	for rowIndex := 0; rowIndex < source.RowCount; rowIndex++ {
		wire.Rows[rowIndex] = make([]any, len(source.Fields))
		for columnIndex := range source.Fields {
			wire.Rows[rowIndex][columnIndex] = source.Fields[columnIndex].Values[rowIndex]
		}
	}
	return wire, nil
}

func wireColumnType(kind frame.FieldType) (document.ColumnType, error) {
	//nolint:exhaustive // Unknown field types are rejected by the meaningful default.
	switch kind {
	case frame.FieldTypeString, frame.FieldTypeLocalized:
		return document.ColumnString, nil
	case frame.FieldTypeNumber:
		return document.ColumnNumber, nil
	case frame.FieldTypeBoolean:
		return document.ColumnBool, nil
	case frame.FieldTypeTime:
		return document.ColumnTime, nil
	default:
		return "", fmt.Errorf("unsupported field type %q", kind)
	}
}

func runtimeResultFromSnapshot(spec lens.DashboardSpec, snapshot *document.Snapshot, request lensruntime.Request) (*lensruntime.DashboardResult, error) {
	spec = snapshotExportSpec(spec)
	result := &lensruntime.DashboardResult{
		Spec: spec, Variables: variableParams(snapshot.Params), Filters: filter.Build(spec.Variables, snapshot.Params),
		Datasets: make(map[string]*lensruntime.DatasetResult), Panels: make(map[string]*lensruntime.PanelResult),
		Locale: request.Locale, Timezone: request.Timezone, RequestPath: request.Path, Request: cloneValues(request.Request),
		StartedAt: snapshot.CreatedAt, SnapshotID: snapshot.ID,
	}
	for _, panelSpec := range lens.FlattenPanels(spec) {
		if panelSpec.Kind.IsContainer() {
			continue
		}
		ref := document.FrameRef("panel:" + panelSpec.ID)
		wire, ok := snapshot.Frames[ref]
		if !ok {
			continue
		}
		frames, err := runtimeFrameSet(string(ref), wire)
		if err != nil {
			return nil, err
		}
		addRuntimePanel(result, panelSpec, frames)
	}
	for _, explorerSpec := range spec.Explorers {
		for _, branch := range explorerSpec.Branches {
			for _, perspective := range branch.Perspectives {
				for _, node := range perspective.Nodes {
					if node.Panel == nil {
						continue
					}
					ref := document.FrameRef("explore:" + qualified(explorerSpec.ID, branch.Key, perspective.Key) + ":" + node.Key)
					wire, ok := snapshot.Frames[ref]
					if !ok {
						continue
					}
					frames, err := runtimeFrameSet(string(ref), wire)
					if err != nil {
						return nil, err
					}
					addRuntimePanel(result, *node.Panel, frames)
				}
			}
		}
	}
	return result, nil
}

func snapshotExportSpec(spec lens.DashboardSpec) lens.DashboardSpec {
	// Evidence pages stay live and are never snapshot frames, so snapshot exports
	// include only the aggregate frames that were visible to the runtime.
	spec.Export.EvidenceDatasets = nil
	spec.Export.IncludeUpstream = false
	spec.Datasets = append([]lens.DatasetSpec(nil), spec.Datasets...)
	for index := range spec.Datasets {
		spec.Datasets[index].Export.EvidenceDatasets = nil
		spec.Datasets[index].Export.IncludeUpstream = false
	}
	spec.Rows = append([]lens.RowSpec(nil), spec.Rows...)
	for rowIndex := range spec.Rows {
		spec.Rows[rowIndex].Panels = append([]panel.Spec(nil), spec.Rows[rowIndex].Panels...)
		for panelIndex := range spec.Rows[rowIndex].Panels {
			spec.Rows[rowIndex].Panels[panelIndex] = stripPanelEvidence(spec.Rows[rowIndex].Panels[panelIndex])
		}
	}
	spec.Explorers = append([]explore.Spec(nil), spec.Explorers...)
	for explorerIndex := range spec.Explorers {
		spec.Explorers[explorerIndex].Branches = append([]explore.Branch(nil), spec.Explorers[explorerIndex].Branches...)
		for branchIndex := range spec.Explorers[explorerIndex].Branches {
			branch := &spec.Explorers[explorerIndex].Branches[branchIndex]
			branch.Perspectives = append([]explore.Perspective(nil), branch.Perspectives...)
			for perspectiveIndex := range branch.Perspectives {
				perspective := &branch.Perspectives[perspectiveIndex]
				perspective.Nodes = append([]explore.Node(nil), perspective.Nodes...)
				for nodeIndex := range perspective.Nodes {
					if perspective.Nodes[nodeIndex].Panel != nil {
						cleaned := stripPanelEvidence(*perspective.Nodes[nodeIndex].Panel)
						perspective.Nodes[nodeIndex].Panel = &cleaned
					}
				}
			}
		}
	}
	return spec
}

func stripPanelEvidence(spec panel.Spec) panel.Spec {
	spec.Export.EvidenceDatasets = nil
	spec.Export.IncludeUpstream = false
	spec.Children = append([]panel.Spec(nil), spec.Children...)
	for index := range spec.Children {
		spec.Children[index] = stripPanelEvidence(spec.Children[index])
	}
	return spec
}

func addRuntimePanel(result *lensruntime.DashboardResult, spec panel.Spec, frames *frame.FrameSet) {
	result.Panels[spec.ID] = &lensruntime.PanelResult{
		Panel: spec, Frames: frames, Locale: result.Locale, Timezone: result.Timezone, Variables: cloneParams(result.Variables),
		RequestPath: result.RequestPath, Request: cloneValues(result.Request),
	}
	if _, ok := result.Datasets[spec.Dataset]; !ok {
		result.Datasets[spec.Dataset] = &lensruntime.DatasetResult{Frames: frames}
	}
}

func runtimeFrameSet(name string, wire document.Frame) (*frame.FrameSet, error) {
	fields := make([]frame.Field, len(wire.Columns))
	for columnIndex, column := range wire.Columns {
		kind, err := runtimeFieldType(column.Type)
		if err != nil {
			return nil, fmt.Errorf("frame %q column %s: %w", name, column.Name, err)
		}
		values := make([]any, len(wire.Rows))
		for rowIndex, row := range wire.Rows {
			if columnIndex >= len(row) {
				return nil, fmt.Errorf("frame %q row %d has too few columns", name, rowIndex)
			}
			values[rowIndex] = row[columnIndex]
		}
		fields[columnIndex] = frame.Field{Name: column.Name, Type: kind, Values: values}
	}
	primary, err := frame.New(name, fields...)
	if err != nil {
		return nil, err
	}
	return frame.NewFrameSet(primary)
}

func runtimeFieldType(kind document.ColumnType) (frame.FieldType, error) {
	switch kind {
	case document.ColumnString:
		return frame.FieldTypeString, nil
	case document.ColumnNumber:
		return frame.FieldTypeNumber, nil
	case document.ColumnBool:
		return frame.FieldTypeBoolean, nil
	case document.ColumnTime:
		return frame.FieldTypeTime, nil
	default:
		return frame.FieldTypeUnknown, fmt.Errorf("unsupported column type %q", kind)
	}
}
