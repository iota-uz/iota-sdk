package excel

import "time"

// ExportOptions configures the Excel export behavior
type ExportOptions struct {
	// IncludeHeaders adds header row with column names
	IncludeHeaders bool
	// AutoFilter adds auto-filter to the header row
	AutoFilter bool
	// FreezeHeader freezes the header row
	FreezeHeader bool
	// DateFormat specifies the format for date columns
	DateFormat string
	// TimeFormat specifies the format for time columns
	TimeFormat string
	// DateTimeFormat specifies the format for datetime columns
	DateTimeFormat string
	// MaxRows limits the number of rows to export (0 = no limit)
	MaxRows int
}

// DefaultOptions returns default export options
func DefaultOptions() *ExportOptions {
	return &ExportOptions{
		IncludeHeaders: true,
		AutoFilter:     true,
		FreezeHeader:   true,
		DateFormat:     "2006-01-02",
		TimeFormat:     "15:04:05",
		DateTimeFormat: "2006-01-02 15:04:05",
		MaxRows:        0,
	}
}

// StyleOptions defines styling for the Excel file
type StyleOptions struct {
	HeaderStyle  *CellStyle
	DataStyle    *CellStyle
	AlternateRow bool
}

// CellStyle defines styling for cells
type CellStyle struct {
	Font      *FontStyle
	Fill      *FillStyle
	Border    *BorderStyle
	Alignment *AlignmentStyle
}

// FontStyle defines font styling
type FontStyle struct {
	Bold   bool
	Italic bool
	Size   int
	Color  string
}

// FillStyle defines fill styling
type FillStyle struct {
	Type    string
	Pattern int
	Color   string
}

// BorderStyle defines border styling
type BorderStyle struct {
	Type  string
	Color string
}

// AlignmentStyle defines alignment
type AlignmentStyle struct {
	Horizontal string
	Vertical   string
	WrapText   bool
}

// DefaultStyleOptions returns default styling options
func DefaultStyleOptions() *StyleOptions {
	return &StyleOptions{
		HeaderStyle: &CellStyle{
			Font: &FontStyle{
				Bold: true,
				Size: 11,
			},
			Fill: &FillStyle{
				Type:    "pattern",
				Pattern: 1,
				Color:   "#E0E0E0",
			},
			Alignment: &AlignmentStyle{
				Horizontal: "center",
				Vertical:   "center",
			},
		},
		DataStyle: &CellStyle{
			Font: &FontStyle{
				Size: 10,
			},
			Alignment: &AlignmentStyle{
				Vertical: "center",
				WrapText: true,
			},
		},
		AlternateRow: true,
	}
}

// ColumnOptions defines column-specific options
type ColumnOptions struct {
	Width    float64
	Format   string
	DataType string
}

// formatValue formats a value based on its type
func formatValue(val interface{}, opts *ExportOptions) interface{} {
	switch v := val.(type) {
	case time.Time:
		return v.Format(opts.DateTimeFormat)
	case *time.Time:
		if v != nil {
			return v.Format(opts.DateTimeFormat)
		}
		return nil
	default:
		return val
	}
}
