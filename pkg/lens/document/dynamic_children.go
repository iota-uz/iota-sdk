package document

import (
	"fmt"
	"strings"
)

func ResolveDynamicChildren(frame *Frame, level Level) error {
	if frame == nil || level.DynamicChildren == nil {
		return nil
	}
	declaration := level.DynamicChildren
	owner := "dynamic level"
	if len(level.Path) > 0 {
		owner = string(level.Path[len(level.Path)-1])
	}
	if err := validateDynamicChildFields(owner, *declaration, *frame); err != nil {
		return err
	}
	indexes := make(map[string]int, len(frame.Columns))
	for index, column := range frame.Columns {
		indexes[column.Name] = index
	}
	read := func(source Source, row []any) (any, error) {
		switch source.Kind {
		case ValueSourceField:
			index, ok := indexes[source.Name]
			if !ok {
				return nil, fmt.Errorf("dynamic children reference missing field %q", source.Name)
			}
			value := row[index]
			if value == nil {
				value = source.Fallback
			}
			return value, nil
		case ValueSourceLiteral:
			return source.Value, nil
		case ValueSourceVariable:
			return nil, fmt.Errorf("dynamic children do not support %q sources", source.Kind)
		default:
			return nil, fmt.Errorf("dynamic children do not support %q sources", source.Kind)
		}
	}
	children := cloneNodes(level.Children)
	seen := make(map[NodeKey]struct{}, len(level.Children)+len(frame.Rows))
	for _, child := range level.Children {
		seen[child.Key] = struct{}{}
	}
	for rowIndex, row := range frame.Rows {
		keyValue, err := read(declaration.Key, row)
		if err != nil {
			return err
		}
		key, ok := keyValue.(string)
		if !ok || strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) {
			return fmt.Errorf("dynamic child row %d key must be a nonblank string", rowIndex)
		}
		nodeKey := NodeKey(key)
		if _, duplicate := seen[nodeKey]; duplicate {
			return fmt.Errorf("dynamic children have duplicate key %q", key)
		}
		seen[nodeKey] = struct{}{}
		labelValue, err := read(declaration.Label, row)
		if err != nil {
			return err
		}
		label, ok := labelValue.(string)
		if !ok || strings.TrimSpace(label) == "" {
			return fmt.Errorf("dynamic child row %d label must be a nonblank string", rowIndex)
		}
		child := Node{Key: nodeKey, Path: appendPath(level.Path, nodeKey), Label: label}
		if declaration.Target != nil {
			targetValue, readErr := read(*declaration.Target, row)
			if readErr != nil {
				return readErr
			}
			target, targetOK := targetValue.(string)
			if !targetOK || strings.TrimSpace(target) == "" || target != strings.TrimSpace(target) {
				return fmt.Errorf("dynamic child row %d target must be a nonblank string", rowIndex)
			}
			child.Target = dynamicTarget(level, target)
		}
		if declaration.Action != nil {
			action := *declaration.Action
			child.Action = &action
		}
		children = append(children, child)
	}
	frame.Children = children
	return nil
}

func dynamicTarget(level Level, target string) NodeKey {
	if strings.Contains(target, "/") || len(level.Path) == 0 {
		return NodeKey(target)
	}
	owner := string(level.Path[len(level.Path)-1])
	if index := strings.LastIndexByte(owner, '/'); index >= 0 {
		return NodeKey(owner[:index+1] + target)
	}
	return NodeKey(target)
}

func ValidateResolvedChildren(level Level, frame Frame, edges map[NodeKey]Level) error {
	seen := make(map[NodeKey]struct{}, len(frame.Children))
	for _, child := range frame.Children {
		if err := validNodeKey("dynamic drill child", child.Key); err != nil {
			return err
		}
		if _, duplicate := seen[child.Key]; duplicate {
			return fmt.Errorf("dynamic children have duplicate key %q", child.Key)
		}
		seen[child.Key] = struct{}{}
		if err := validateChildPath(level.Path[len(level.Path)-1], level.Path, child); err != nil {
			return err
		}
		if (child.Target == "") == (child.Action == nil) {
			return fmt.Errorf("dynamic child %q requires exactly one of target or action", child.Key)
		}
		if child.Target != "" {
			if _, ok := edges[child.Target]; !ok {
				return fmt.Errorf("dynamic child %q references missing target %q", child.Key, child.Target)
			}
		}
		if child.Action != nil {
			if err := validateAction(string(child.Key), *child.Action); err != nil {
				return err
			}
			if err := validateActionFields(string(child.Key), *child.Action, frame); err != nil {
				return err
			}
		}
	}
	return nil
}
