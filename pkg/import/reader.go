package importpkg

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// ExcelFileReader is the excelize-based implementation
type ExcelFileReader struct{}

// NewExcelFileReader creates a new Excel file reader
func NewExcelFileReader() *ExcelFileReader {
	return &ExcelFileReader{}
}

func (r *ExcelFileReader) ReadExcelRows(filePath string) ([][]string, error) {
	file, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found")
	}

	return file.GetRows(sheets[0])
}
