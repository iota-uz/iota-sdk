package pagination

import (
	"net/url"
	"strconv"
)

// NextChunkURL returns the address of the chunk after page, keeping every
// other query parameter so search and filters still apply to it. A chunk
// shorter than limit is the last one, so the result is empty.
func NextChunkURL(current *url.URL, page, limit, loaded int) string {
	if current == nil || limit <= 0 || loaded < limit {
		return ""
	}
	query := current.Query()
	query.Set("page", strconv.Itoa(page+1))
	query.Set("limit", strconv.Itoa(limit))
	next := url.URL{Path: current.Path, RawQuery: query.Encode()}
	return next.String()
}
