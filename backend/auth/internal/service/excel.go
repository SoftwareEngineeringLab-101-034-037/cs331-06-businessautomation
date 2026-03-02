package service

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// EmployeeRow represents a single valid row parsed from the bulk invite Excel file.
type EmployeeRow struct {
	RowNumber  int    `json:"row_number"`
	Email      string `json:"email"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Department string `json:"department"`
	Role       string `json:"role"`
	JobTitle   string `json:"job_title"`
}

// ParseError describes a validation error for a specific row in the Excel file.
type ParseError struct {
	Row     int    `json:"row"`
	Email   string `json:"email"`
	Message string `json:"message"`
}

// ParseResult holds the successfully parsed rows and any validation errors.
type ParseResult struct {
	Rows   []EmployeeRow `json:"rows"`
	Errors []ParseError  `json:"errors"`
}

// expectedColumns maps lowercased header names to their logical field.
var expectedColumns = map[string]string{
	"email":      "email",
	"first_name": "first_name",
	"first name": "first_name",
	"firstname":  "first_name",
	"last_name":  "last_name",
	"last name":  "last_name",
	"lastname":   "last_name",
	"department": "department",
	"role":       "role",
	"job_title":  "job_title",
	"job title":  "job_title",
	"jobtitle":   "job_title",
}

func ParseEmployeeExcel(r io.Reader) (*ParseResult, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file must have a header row and at least one data row")
	}

	headerRow := rows[0]
	colIndex := make(map[string]int)
	for i, cell := range headerRow {
		key := strings.TrimSpace(strings.ToLower(cell))
		if field, ok := expectedColumns[key]; ok {
			colIndex[field] = i
		}
	}

	// Validate required headers
	for _, required := range []string{"email", "first_name", "last_name", "department"} {
		if _, ok := colIndex[required]; !ok {
			return nil, fmt.Errorf("missing required column: %s", required)
		}
	}

	result := &ParseResult{}

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		rowNum := rowIdx + 1

		get := func(field string) string {
			idx, ok := colIndex[field]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		email := get("email")
		firstName := get("first_name")
		lastName := get("last_name")
		department := get("department")
		role := get("role")
		jobTitle := get("job_title")

		// Validate required fields
		var missing []string
		if email == "" {
			missing = append(missing, "email")
		}
		if firstName == "" {
			missing = append(missing, "first_name")
		}
		if lastName == "" {
			missing = append(missing, "last_name")
		}
		if department == "" {
			missing = append(missing, "department")
		}

		if len(missing) > 0 {
			result.Errors = append(result.Errors, ParseError{
				Row:     rowNum,
				Email:   email,
				Message: fmt.Sprintf("missing required fields: %s", strings.Join(missing, ", ")),
			})
			continue
		}

		result.Rows = append(result.Rows, EmployeeRow{
			RowNumber:  rowNum,
			Email:      email,
			FirstName:  firstName,
			LastName:   lastName,
			Department: department,
			Role:       role,
			JobTitle:   jobTitle,
		})
	}

	return result, nil
}
