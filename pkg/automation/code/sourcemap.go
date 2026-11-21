package code

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// sourceMap is a decoded source map v3 with one source: for each generated line, its segments by column.
type sourceMap struct {
	source string
	lines  [][]mapping
}

type mapping struct {
	genCol, line, col int // 0-based
}

// parseSourceMap decodes the mappings of a source map. It returns nil for a map it cannot read.
func parseSourceMap(data []byte) *sourceMap {
	var raw struct {
		Sources  []string `json:"sources"`
		Mappings string   `json:"mappings"`
	}
	if len(data) == 0 || json.Unmarshal(data, &raw) != nil {
		return nil
	}
	sm := &sourceMap{}
	if len(raw.Sources) > 0 {
		sm.source = raw.Sources[0]
	}
	var src, line, col int
	for _, group := range strings.Split(raw.Mappings, ";") {
		var segs []mapping
		genCol := 0
		for _, seg := range strings.Split(group, ",") {
			if seg == "" {
				continue
			}
			fields, ok := decodeVLQ(seg)
			if !ok || len(fields) == 0 {
				return nil
			}
			genCol += fields[0]
			if len(fields) < 4 {
				continue
			}
			src += fields[1]
			line += fields[2]
			col += fields[3]
			if src == 0 {
				segs = append(segs, mapping{genCol: genCol, line: line, col: col})
			}
		}
		sm.lines = append(sm.lines, segs)
	}
	return sm
}

const vlqChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func decodeVLQ(s string) ([]int, bool) {
	var out []int
	value, shift := 0, 0
	for i := 0; i < len(s); i++ {
		d := strings.IndexByte(vlqChars, s[i])
		if d < 0 || shift > 30 {
			return nil, false
		}
		value += (d & 31) << shift
		if d&32 != 0 {
			shift += 5
			continue
		}
		n := value >> 1
		if value&1 != 0 {
			n = -n
		}
		out = append(out, n)
		value, shift = 0, 0
	}
	return out, shift == 0
}

// lookup maps a 1-based generated position to a 1-based source position.
func (sm *sourceMap) lookup(line, col int) (int, int, bool) {
	if sm == nil || line < 1 || line > len(sm.lines) {
		return 0, 0, false
	}
	segs := sm.lines[line-1]
	i := sort.Search(len(segs), func(i int) bool { return segs[i].genCol > col-1 }) - 1
	if i < 0 {
		if len(segs) == 0 {
			return 0, 0, false
		}
		i = 0
	}
	return segs[i].line + 1, segs[i].col + 1, true
}

// evalFrame is a position in the evaluated script as QuickJS prints it in a stack.
var evalFrame = regexp.MustCompile(`<eval>:(\d+):(\d+)`)

// mapStack rewrites the positions of a QuickJS stack into source positions and drops the frames of the
// driver and of the script's glue. It returns the stack and the innermost source position (0, 0 if none).
func mapStack(stack string, sm *sourceMap, filename string, scriptLines int) (string, int, int) {
	var out []string
	firstLine, firstCol := 0, 0
	for _, frame := range strings.Split(strings.TrimRight(stack, "\n"), "\n") {
		if strings.TrimSpace(frame) == "" {
			continue
		}
		m := evalFrame.FindStringSubmatchIndex(frame)
		if m == nil {
			continue // a frame of the driver (<compile>) or of a native function
		}
		line, _ := strconv.Atoi(frame[m[2]:m[3]])
		col, _ := strconv.Atoi(frame[m[4]:m[5]])
		if line > scriptLines {
			continue // the driver call appended after the script
		}
		srcLine, srcCol, ok := sm.lookup(line, col)
		if !ok {
			continue
		}
		if firstLine == 0 {
			firstLine, firstCol = srcLine, srcCol
		}
		pos := filename + ":" + strconv.Itoa(srcLine) + ":" + strconv.Itoa(srcCol)
		out = append(out, frame[:m[0]]+pos+frame[m[1]:])
	}
	return strings.Join(out, "\n"), firstLine, firstCol
}
