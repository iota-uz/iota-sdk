package explore

import "fmt"

type Status string

const (
	StatusIdle    Status = "idle"
	StatusLoading Status = "loading"
	StatusReady   Status = "ready"
	StatusError   Status = "error"
)

type State struct {
	ExplorerID         string                `json:"explorerId"`
	BranchKey          string                `json:"branchKey,omitempty"`
	PerspectiveKey     string                `json:"perspectiveKey,omitempty"`
	PathByPerspective  map[string][]string   `json:"pathByPerspective,omitempty"`
	StepsByPerspective map[string][]PathStep `json:"stepsByPerspective,omitempty"`
	HiddenByView       map[string][]string   `json:"hiddenByView,omitempty"`
	Status             Status                `json:"status"`
	RequestVersion     int                   `json:"requestVersion"`
	Error              string                `json:"error,omitempty"`
}

type EventKind string

const (
	EventOpen              EventKind = "open"
	EventSelectPerspective EventKind = "select_perspective"
	EventDrill             EventKind = "drill"
	EventBack              EventKind = "back"
	EventHome              EventKind = "home"
	EventRequestStarted    EventKind = "request_started"
	EventRequestSucceeded  EventKind = "request_succeeded"
	EventRequestFailed     EventKind = "request_failed"
	EventSetHidden         EventKind = "set_hidden"
)

type Event struct {
	Kind           EventKind
	BranchKey      string
	PerspectiveKey string
	NodeKey        string
	PointKey       string
	Hidden         []string
	RequestVersion int
	Error          string
}

func InitialState(spec Spec) State {
	return State{
		ExplorerID:         spec.ID,
		PathByPerspective:  make(map[string][]string),
		StepsByPerspective: make(map[string][]PathStep),
		HiddenByView:       make(map[string][]string),
		Status:             StatusIdle,
	}
}

type Selection struct {
	ExplorerID     string
	BranchKey      string
	PerspectiveKey string
	Path           []string
	Steps          []PathStep
	NodeKey        string
}

func ActiveSelection(spec Spec, state State) (Selection, error) {
	perspective, steps, err := activeSteps(spec, state)
	if err != nil {
		return Selection{}, err
	}
	return Selection{
		ExplorerID:     spec.ID,
		BranchKey:      state.BranchKey,
		PerspectiveKey: perspective.Key,
		Path:           nodePath(steps),
		Steps:          steps,
		NodeKey:        steps[len(steps)-1].NodeKey,
	}, nil
}

func Reduce(spec Spec, current State, event Event) (State, error) {
	if err := spec.Validate(); err != nil {
		return State{}, err
	}
	state := cloneState(current)
	if state.ExplorerID == "" {
		state = InitialState(spec)
	}
	if state.ExplorerID != spec.ID {
		return State{}, fmt.Errorf("state belongs to explorer %q, not %q", state.ExplorerID, spec.ID)
	}

	switch event.Kind {
	case EventOpen:
		branch, ok := spec.Branch(event.BranchKey)
		if !ok {
			return State{}, fmt.Errorf("explorer %s has no branch %q", spec.ID, event.BranchKey)
		}
		perspectiveKey := event.PerspectiveKey
		if perspectiveKey == "" {
			perspectiveKey = branch.DefaultPerspective
		}
		perspective, ok := branch.Perspective(perspectiveKey)
		if !ok {
			return State{}, fmt.Errorf("explorer %s branch %s has no perspective %q", spec.ID, branch.Key, perspectiveKey)
		}
		state.BranchKey = branch.Key
		state.PerspectiveKey = perspective.Key
		view := viewKey(branch.Key, perspective.Key)
		if len(state.PathByPerspective[view]) == 0 {
			state.PathByPerspective[view] = []string{perspective.RootNode}
		}
		if len(state.StepsByPerspective[view]) == 0 {
			state.StepsByPerspective[view] = []PathStep{{NodeKey: perspective.RootNode}}
		}
		state.Status = StatusReady
		state.Error = ""
	case EventSelectPerspective:
		branch, ok := spec.Branch(state.BranchKey)
		if !ok {
			return State{}, fmt.Errorf("cannot select a perspective without an open branch")
		}
		perspective, ok := branch.Perspective(event.PerspectiveKey)
		if !ok {
			return State{}, fmt.Errorf("explorer %s branch %s has no perspective %q", spec.ID, branch.Key, event.PerspectiveKey)
		}
		state.PerspectiveKey = perspective.Key
		view := viewKey(branch.Key, perspective.Key)
		if len(state.PathByPerspective[view]) == 0 {
			state.PathByPerspective[view] = []string{perspective.RootNode}
		}
		if len(state.StepsByPerspective[view]) == 0 {
			state.StepsByPerspective[view] = []PathStep{{NodeKey: perspective.RootNode}}
		}
		state.Status = StatusReady
		state.Error = ""
	case EventDrill:
		perspective, steps, err := activeSteps(spec, state)
		if err != nil {
			return State{}, err
		}
		if _, ok := perspective.Node(event.NodeKey); !ok {
			return State{}, fmt.Errorf("perspective %s has no node %q", perspective.Key, event.NodeKey)
		}
		currentNode, ok := perspective.Node(steps[len(steps)-1].NodeKey)
		if !ok {
			return State{}, fmt.Errorf("perspective %s has no active node %q", perspective.Key, steps[len(steps)-1].NodeKey)
		}
		reachable := false
		for _, edge := range currentNode.Edges {
			if edge.ToNode == event.NodeKey && (event.PointKey == "" || edge.PointKey == event.PointKey) {
				reachable = true
				if event.PointKey == "" {
					event.PointKey = edge.PointKey
				}
				break
			}
		}
		if !reachable && currentNode.DynamicEdges && event.PointKey != "" {
			for _, target := range currentNode.DynamicTargets {
				if target == event.NodeKey {
					reachable = true
					break
				}
			}
		}
		if !reachable {
			return State{}, fmt.Errorf("node %s is not reachable from %s", event.NodeKey, currentNode.Key)
		}
		view := viewKey(state.BranchKey, state.PerspectiveKey)
		steps = append(steps, PathStep{NodeKey: event.NodeKey, PointKey: event.PointKey})
		state.StepsByPerspective[view] = steps
		state.PathByPerspective[view] = nodePath(steps)
		state.Status = StatusReady
		state.Error = ""
	case EventBack:
		_, steps, err := activeSteps(spec, state)
		if err != nil {
			return State{}, err
		}
		if len(steps) <= 1 {
			state.BranchKey = ""
			state.PerspectiveKey = ""
			state.Status = StatusIdle
		} else {
			view := viewKey(state.BranchKey, state.PerspectiveKey)
			steps = append([]PathStep(nil), steps[:len(steps)-1]...)
			state.StepsByPerspective[view] = steps
			state.PathByPerspective[view] = nodePath(steps)
		}
		state.Error = ""
	case EventHome:
		state.BranchKey = ""
		state.PerspectiveKey = ""
		state.Status = StatusIdle
		state.Error = ""
	case EventRequestStarted:
		if event.RequestVersion <= state.RequestVersion {
			return state, nil
		}
		state.RequestVersion = event.RequestVersion
		state.Status = StatusLoading
		state.Error = ""
	case EventRequestSucceeded:
		if event.RequestVersion != state.RequestVersion {
			return state, nil
		}
		state.Status = StatusReady
		state.Error = ""
	case EventRequestFailed:
		if event.RequestVersion != state.RequestVersion {
			return state, nil
		}
		state.Status = StatusError
		state.Error = event.Error
	case EventSetHidden:
		if _, _, err := activePath(spec, state); err != nil {
			return State{}, err
		}
		state.HiddenByView[activeNodeViewKey(state)] = append([]string(nil), event.Hidden...)
	default:
		return State{}, fmt.Errorf("unsupported explorer event %q", event.Kind)
	}
	return state, nil
}

func activePath(spec Spec, state State) (Perspective, []string, error) {
	perspective, steps, err := activeSteps(spec, state)
	return perspective, nodePath(steps), err
}

func activeSteps(spec Spec, state State) (Perspective, []PathStep, error) {
	branch, ok := spec.Branch(state.BranchKey)
	if !ok {
		return Perspective{}, nil, fmt.Errorf("explorer has no open branch")
	}
	perspective, ok := branch.Perspective(state.PerspectiveKey)
	if !ok {
		return Perspective{}, nil, fmt.Errorf("explorer branch %s has no active perspective", branch.Key)
	}
	view := viewKey(branch.Key, perspective.Key)
	steps := append([]PathStep(nil), state.StepsByPerspective[view]...)
	if len(steps) == 0 {
		for _, key := range state.PathByPerspective[view] {
			steps = append(steps, PathStep{NodeKey: key})
		}
	}
	if len(steps) == 0 {
		steps = []PathStep{{NodeKey: perspective.RootNode}}
	}
	return perspective, steps, nil
}

func cloneState(state State) State {
	state.PathByPerspective = cloneStringSlices(state.PathByPerspective)
	state.StepsByPerspective = clonePathSteps(state.StepsByPerspective)
	state.HiddenByView = cloneStringSlices(state.HiddenByView)
	if state.PathByPerspective == nil {
		state.PathByPerspective = make(map[string][]string)
	}
	if state.StepsByPerspective == nil {
		state.StepsByPerspective = make(map[string][]PathStep)
	}
	if state.HiddenByView == nil {
		state.HiddenByView = make(map[string][]string)
	}
	return state
}

func clonePathSteps(values map[string][]PathStep) map[string][]PathStep {
	if values == nil {
		return nil
	}
	cloned := make(map[string][]PathStep, len(values))
	for key, value := range values {
		cloned[key] = append([]PathStep(nil), value...)
	}
	return cloned
}

func nodePath(steps []PathStep) []string {
	path := make([]string, 0, len(steps))
	for _, step := range steps {
		path = append(path, step.NodeKey)
	}
	return path
}

func cloneStringSlices(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string][]string, len(values))
	for key, value := range values {
		cloned[key] = append([]string(nil), value...)
	}
	return cloned
}

func viewKey(branchKey, perspectiveKey string) string {
	return branchKey + "/" + perspectiveKey
}

func activeNodeViewKey(state State) string {
	view := viewKey(state.BranchKey, state.PerspectiveKey)
	steps := state.StepsByPerspective[view]
	if len(steps) == 0 {
		path := state.PathByPerspective[view]
		if len(path) == 0 {
			return view
		}
		return view + "/" + path[len(path)-1]
	}
	for _, step := range steps {
		view += "/" + step.NodeKey
		if step.PointKey != "" {
			view += "@" + step.PointKey
		}
	}
	return view
}
