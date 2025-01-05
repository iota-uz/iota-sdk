package repo

import (
	"bufio"
	"fmt"
	"strings"
)

func MustParseSQLQueries(sqlContent string) map[string]string {
	queries, err := ParseSQLQueries(sqlContent)
	if err != nil {
		panic(err)
	}
	return queries
}

func ParseSQLQueries(sqlContent string) (map[string]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(sqlContent))
	queries := make(map[string]string)
	var currentKey string
	var queryBuilder strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "-- name:") {
			// Save the previous query
			if currentKey != "" {
				queries[currentKey] = queryBuilder.String()
				queryBuilder.Reset()
			}

			// Set new key
			currentKey = strings.TrimSpace(strings.TrimPrefix(line, "-- name:"))
		} else if currentKey != "" {
			// Append lines to the current query
			queryBuilder.WriteString(line + "\n")
		}
	}

	// Save the last query
	if currentKey != "" {
		queries[currentKey] = queryBuilder.String()
	}

	for k, v := range queries {
		queries[k] = strings.TrimRight(v, " ;\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SQL content: %w", err)
	}

	return queries, nil
}
