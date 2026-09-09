/*
 * Convert different CSV formats into the politicaldissidence/data MP struct.
 * TODO: exported methods should take a reader directly, allows for easier switching between CSV and Response Body when
 * developing.
 * https://www.aph.gov.au/Senators_and_Members/Contacting_Senators_and_Members
 */
package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"politicaldissidence/data"
)

// ColumnMap maps data.MP fields to CSV header names for a source format.
// To add a new CSV config, define a ColumnMap and pass it to ParseReader with a Level.
type ColumnMap struct {
	Honorific     string
	FirstName     string
	Surname       string
	OtherName     string
	PreferredName string
	Party         string
	State         string
	Electorate    string
}

// SenatorColumns is the APH senators address-label CSV header map.
var SenatorColumns = ColumnMap{
	Honorific:     "Title",
	FirstName:     "First Name",
	Surname:       "Surname",
	OtherName:     "Other Name",
	PreferredName: "Preferred Name",
	Party:         "Political Party",
	State:         "State",
	Electorate:    "Electorate Suburb",
}

// MemberColumns is the APH House of Reps address-label CSV header map.
var MemberColumns = ColumnMap{
	Honorific:     "Honorific",
	FirstName:     "First Name",
	Surname:       "Surname",
	OtherName:     "Other Name",
	PreferredName: "Preferred Name",
	Party:         "Political Party",
	State:         "State",
	Electorate:    "Electorate",
}

// CustomColumns is an empty header map for ad-hoc CSVs. Fill in header names, then use ParseCustomFile / -custom.
var CustomColumns = ColumnMap{
	Honorific:     "",
	FirstName:     "",
	Surname:       "",
	OtherName:     "",
	PreferredName: "",
	Party:         "",
	State:         "",
	Electorate:    "",
}

func (c ColumnMap) headers() []string {
	return []string{
		c.Honorific,
		c.FirstName,
		c.Surname,
		c.OtherName,
		c.PreferredName,
		c.Party,
		c.State,
		c.Electorate,
	}
}

func ParseSenatorMps(url string) ([]data.MP, error) {
	_ = url
	return ParseSenatorFile("./allsenel.csv")
}

func ParseMemberMps(url string) ([]data.MP, error) {
	_ = url
	return ParseMemberFile("./FamilynameRepsCSV.csv")
}

// ParseSenatorFile reads a senators address-label CSV from path into MP rows.
func ParseSenatorFile(path string) ([]data.MP, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseSenatorReader(f)
}

// ParseMemberFile reads a House of Reps address-label CSV from path into MP rows.
func ParseMemberFile(path string) ([]data.MP, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseMemberReader(f)
}

// ParseCustomFile reads a CSV from path using CustomColumns into MP rows.
func ParseCustomFile(path string) ([]data.MP, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseCustomReader(f)
}

// ParseSenatorReader converts a senators CSV reader into MP rows (Federal Senator).
func ParseSenatorReader(r io.Reader) ([]data.MP, error) {
	return ParseReader(r, SenatorColumns, data.Level.FedSenator)
}

// ParseMemberReader converts a HoR members CSV reader into MP rows (Federal Rep).
func ParseMemberReader(r io.Reader) ([]data.MP, error) {
	return ParseReader(r, MemberColumns, data.Level.FedRep)
}

// ParseCustomReader converts a CSV reader into MP rows using CustomColumns (level left empty).
func ParseCustomReader(r io.Reader) ([]data.MP, error) {
	return ParseReader(r, CustomColumns, "")
}

// ParseReader converts a CSV reader into MP rows using cols for header mapping and level for each row.
func ParseReader(r io.Reader, cols ColumnMap, level string) ([]data.MP, error) {
	records, err := readCsv(r, true, true)
	if err != nil {
		fmt.Println("[-] Could not read CSV", err.Error())
		return nil, err
	}

	mps, err := getMpRows(records, cols)
	if err != nil {
		return nil, err
	}

	for i := range mps {
		mps[i].Level = level
	}
	return mps, nil
}

func getMpRows(records [][]string, cols ColumnMap) ([]data.MP, error) {
	rows, err := extractColumns(records, cols.headers())
	if err != nil {
		return nil, err
	}

	mps := make([]data.MP, len(rows))
	for i, row := range rows {
		mps[i] = data.MP{
			Honorific:     row[0],
			FirstName:     row[1],
			Surname:       row[2],
			OtherName:     row[3],
			PreferredName: row[4],
			Party:         row[5],
			State:         row[6],
			Electorate:    row[7],
		}
	}

	return mps, nil
}

/*
 * For each column header value in the columns array, calculate it's index based on the index it appears in within the
 * first array in records. This index is then used to extract all column values in the same order as the columns array.
 */
func extractColumns(records [][]string, columns []string) ([][]string, error) {
	indexes := make([]int, len(columns))
	for i := range indexes {
		indexes[i] = -1
	}

	firstRow := records[0]

	fmt.Println("[*] Calculating", len(columns), "columns from", len(records[0]), "rows")
	for x, col := range columns {
		for y, rowCol := range firstRow {
			if col == rowCol {
				indexes[x] = y
				break
			}
		}
		if indexes[x] == -1 {
			return nil, errors.New(fmt.Sprintf("[-] Expected column <%s> in first row of CSV", col))
		}
	}

	values := make([][]string, len(records)-1)
	for rI, rowCols := range records[1:] {
		rowVals := make([]string, len(columns))
		for cI, csvColIndex := range indexes {
			rowVals[cI] = rowCols[csvColIndex]
		}
		values[rI] = rowVals
	}

	return values, nil
}

func readCsv(reader io.Reader, lazyQuotes bool, variableFields bool) ([][]string, error) {
	r := csv.NewReader(reader)

	r.LazyQuotes = lazyQuotes
	if variableFields {
		r.FieldsPerRecord = -1
	} else {
		r.FieldsPerRecord = 0
	}

	return r.ReadAll()
}

/**
 * Caller responsible for closing returned Reader
 */
func downloadCsv(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("[-] Could not download CSV <%s> %s", url, err.Error())
		return nil, err
	}

	fmt.Println("[*]", resp.Request.Method, resp.StatusCode, resp.Request.URL.String())

	return resp.Body, nil
}
