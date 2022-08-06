/*
 * Convert different CSV formats into the politicaldissidence/data MP struct.
 * TODO: exported methods should take a reader directly, allows for easier switching between CSV and Response Body when
 * developing.
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

func ParseSenatorMps(url string) ([]data.MP, error) {
	reader, err := os.Open("/Users/quinn/Downloads/allsenph.csv")
	// reader, err := downloadCsv(url)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// open CSV
	records, err := readCsv(reader, true, true)
	if err != nil {
		fmt.Println("[-] Could not read CSV", err.Error())
	}

	mps, err := getMpRows(records,
		"Title", "First Name", "Surname", "Other Name", "Preferred Name", "Political Party", "State", "Electorate")
	if err != nil {
		return nil, err
	}

	for _, mp := range mps {
		mp.Level = data.Level.FedSenator
	}
	return mps, nil
}

func ParseMemberMps(url string) ([]data.MP, error) {
	reader, err := os.Open("/Users/quinn/Downloads/FamilynameRepsCSV.csv")
	// reader, err := downloadCsv(url)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// open CSV
	records, err := readCsv(reader, true, true)
	if err != nil {
		fmt.Println("[-] Could not read CSV", err.Error())
	}

	mps, err := getMpRows(records,
		"Honorific", "First Name", "Surname", "Other Name", "Preferred Name", "Political Party", "State", "Electorate")
	if err != nil {
		return nil, err
	}

	for _, mp := range mps {
		mp.Level = data.Level.FedRep
	}
	return mps, nil
}

func getMpRows(records [][]string,
	honorific string,
	firstName string,
	surname string,
	otherName string,
	preferedName string,
	party string,
	state string,
	electorate string) ([]data.MP, error) {
	columns := []string{
		honorific,
		firstName,
		surname,
		otherName,
		preferedName,
		party,
		state,
		electorate,
	}

	rows, err := extractColumns(records, columns)
	if err != nil {
		return nil, err
	}

	// fmt.Println("Mapping", len(rows), "rows")
	// convert each row into an MP
	var mps []data.MP = make([]data.MP, len(rows))

	for i, row := range rows {
		mp := data.MP{
			Honorific:     row[0],
			FirstName:     row[1],
			Surname:       row[2],
			OtherName:     row[3],
			PreferredName: row[4],
			Party:         row[5],
			State:         row[6],
			Electorate:    row[7],
		}
		mps[i] = mp
	}

	return mps, nil
}

/*
 * For each column header value in the columns array, calculate it's index based on the index it appears in within the
 * first array in records. This index is then used to extract all column values in the same order as the columns array.
 */
func extractColumns(records [][]string, columns []string) ([][]string, error) {
	var indexes []int = make([]int, len(columns))
	// -1 initialize array
	for i, _ := range indexes {
		indexes[i] = -1
	}

	firstRow := records[0]

	fmt.Println("[*] Calculating", len(columns), "columns from", len(records[0]), "rows")
	// find index of column headers
	for x, col := range columns {
		for y, rowCol := range firstRow {
			// fmt.Println(col, "=", rowCol)
			if col == rowCol {
				// fmt.Println("[+] Column ", col, "in index", y)
				indexes[x] = y
				break
			}
		}
		if indexes[x] == -1 {
			// expect columns must appear
			return nil, errors.New(fmt.Sprintf("[-] Expected column <%s> in first row of CSV", col))
		}
	}

	var values [][]string = make([][]string, len(records)-1)

	// TODO: could this be done more simply ?
	// push the value of each column into the corresponding index in values
	for rI, rowCols := range records[1:] {
		// fmt.Println("~~~~~~")
		var rowVals []string = make([]string, len(columns))

		for cI, csvColIndex := range indexes {
			rowVals[cI] = rowCols[csvColIndex]
			// fmt.Printf("Column:%s=%s\n", columns[cI], rowVals[cI])
		}
		values[rI] = rowVals
	}

	return values, nil
}

func readCsv(reader io.ReadCloser, lazyQuotes bool, variableFields bool) ([][]string, error) {
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
