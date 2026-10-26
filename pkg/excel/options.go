// Package excel provides this package.
package excel

// ExportOptions configures the Excel export behavior
type ExportOptions struct {
	// IncludeHeaders adds header row with column names
	IncludeHeaders bool
	// AutoFilter adds auto-filter to the header row
	AutoFilter bool
	// FreezeHeader freezes the header row
	FreezeHeader bool
	// DateFormat is retained for source compatibility. XLSX values stay typed;
	// use Date or Cell.NumberFormat to choose their Excel display format.
	DateFormat string
	// TimeFormat is retained for source compatibility; use Cell.NumberFormat.
	TimeFormat string
	// DateTimeFormat is a legacy Go layout. The default ISO layout and an empty
	// layout keep native dates; other layouts require DateTimeNumberFormat or an
	// explicit Cell.NumberFormat instead of being silently emitted as text.
	DateTimeFormat string
	// DateTimeNumberFormat is an Excel format code for native time.Time values.
	// Empty uses "yyyy-mm-dd hh:mm:ss". Cell.NumberFormat overrides this option.
	DateTimeNumberFormat string
	// MaxRows limits the number of rows to export (0 = no limit)
	MaxRows int
	// FloatNumberFormat is an optional Excel number format code for floating-point
	// cells. Values remain numeric and can be used in formulas.
	FloatNumberFormat string
	// DecimalComma is deprecated and ignored for XLSX. Numeric cells must remain
	// numbers; Excel chooses decimal separators from the user's locale.
	// CSVOptions.DecimalComma still controls CSV text rendering.
	DecimalComma bool
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
