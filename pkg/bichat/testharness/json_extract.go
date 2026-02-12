package testharness

import "bytes"

func extractJSONObject(data []byte) ([]byte, bool) {
	start := bytes.IndexByte(data, '{')
	if start < 0 {
		return nil, false
	}
	end := bytes.LastIndexByte(data, '}')
	if end < 0 || end < start {
		return nil, false
	}
	return bytes.TrimSpace(data[start : end+1]), true
}
