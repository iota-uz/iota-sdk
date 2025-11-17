package changelog

// FileStats represents statistics about changed files
type FileStats struct {
	ChangedFiles   []string
	AllFiles       int
	Total          int
	Presentation   int
	Business       int
	Infrastructure int
	Migrations     int
	Layers         int
	TestsOnly      bool
	DocsOnly       bool
}

// CheckResult represents the result of a changelog requirement check
type CheckResult struct {
	Stats          FileStats
	Recommendation string
	Reason         string
}
