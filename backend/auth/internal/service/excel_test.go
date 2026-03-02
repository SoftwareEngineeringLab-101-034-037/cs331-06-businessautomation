package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestParseEmployeeExcelInvalidFile(t *testing.T) {
	_, err := ParseEmployeeExcel(strings.NewReader("not-an-excel-file"))
	if err == nil || !strings.Contains(err.Error(), "failed to open Excel file") {
		t.Fatalf("expected invalid excel error, got %v", err)
	}
}

func TestParseEmployeeExcelRequiresDataRows(t *testing.T) {
	data := buildWorkbookBytes(t, []string{"email", "first_name", "last_name", "department"}, nil)
	_, err := ParseEmployeeExcel(bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "header row and at least one data row") {
		t.Fatalf("expected missing data rows error, got %v", err)
	}
}

func TestParseEmployeeExcelMissingRequiredColumn(t *testing.T) {
	data := buildWorkbookBytes(t, []string{"email", "first_name", "last_name"}, [][]string{
		{"user@example.com", "User", "One"},
	})
	_, err := ParseEmployeeExcel(bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "missing required column: department") {
		t.Fatalf("expected missing column error, got %v", err)
	}
}

func TestParseEmployeeExcelParsesRowsAndCollectsValidationErrors(t *testing.T) {
	data := buildWorkbookBytes(t,
		[]string{"email", "first name", "last name", "department", "role", "job title"},
		[][]string{
			{"ok@example.com", "Ok", "User", "Engineering", "member", "Analyst"},
			{"missing@example.com", "Miss", "Ing", "", "member", "Analyst"},
		},
	)

	res, err := ParseEmployeeExcel(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected no parser error, got %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 valid row, got %d", len(res.Rows))
	}
	if len(res.Errors) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(res.Errors))
	}
	if res.Rows[0].Email != "ok@example.com" || res.Rows[0].JobTitle != "Analyst" {
		t.Fatalf("unexpected parsed row: %+v", res.Rows[0])
	}
	if !strings.Contains(res.Errors[0].Message, "missing required fields: department") {
		t.Fatalf("unexpected validation error: %+v", res.Errors[0])
	}
}

func buildWorkbookBytes(t *testing.T, headers []string, rows [][]string) []byte {
	t.Helper()

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)

	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			t.Fatalf("failed creating header cell name: %v", err)
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			t.Fatalf("failed setting header cell: %v", err)
		}
	}

	for r, row := range rows {
		for c, v := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+2)
			if err != nil {
				t.Fatalf("failed creating row cell name: %v", err)
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				t.Fatalf("failed setting row cell value: %v", err)
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("failed writing workbook to buffer: %v", err)
	}

	return buf.Bytes()
}
