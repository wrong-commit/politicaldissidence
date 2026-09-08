/**
 * Package responsible for reading and writing application data to a persistent storage medium.
 */
package db

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"politicaldissidence/data"
)

const mpJsonFilename = "mp_data.json"

// MPJSONStatus is the outcome of reading and validating mp_data.json.
type MPJSONStatus struct {
	Path      string // absolute path of the file checked
	SizeBytes int64
	MPs       []data.MP
	Err       error // nil when Valid
	Line      int   // 1-based line of Err when known; 0 if unknown
	Col       int   // 1-based column of Err when known; 0 if unknown
}

// Valid reports whether the JSON file passed structural validation.
func (s MPJSONStatus) Valid() bool {
	return s.Err == nil
}

// SizeKB returns file size in kibibytes (bytes/1024).
func (s MPJSONStatus) SizeKB() float64 {
	return float64(s.SizeBytes) / 1024.0
}

// LogMessage returns the console line for this validation result.
func (s MPJSONStatus) LogMessage() string {
	if s.Valid() {
		return "INFO mp_data.json valid"
	}
	reason := "unknown error"
	if s.Err != nil {
		reason = s.Err.Error()
	}
	if s.Line > 0 {
		return fmt.Sprintf("ERROR mp_data.json invalid path=%s size=%.1fKB line=%d col=%d: %s",
			s.Path, s.SizeKB(), s.Line, s.Col, reason)
	}
	return fmt.Sprintf("ERROR mp_data.json invalid path=%s size=%.1fKB: %s", s.Path, s.SizeKB(), reason)
}

// ReadMpsValidated reads and validates the default MP JSON file.
func ReadMpsValidated() MPJSONStatus {
	return ValidateMPJSONFile(mpJsonFilename)
}

// ValidateMPJSONFile reads path, checks it is entirely correct MP JSON, and returns status.
func ValidateMPJSONFile(path string) MPJSONStatus {
	abs, absErr := filepath.Abs(path)
	if absErr != nil {
		abs = path
	}

	status := MPJSONStatus{Path: abs}

	if fi, err := os.Stat(path); err == nil {
		status.SizeBytes = fi.Size()
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		status.Err = err
		return status
	}
	// Prefer size from bytes read when Stat failed but read succeeded.
	if status.SizeBytes == 0 {
		status.SizeBytes = int64(len(buf))
	}

	mps, loc, err := decodeMpsStrict(buf)
	if err != nil {
		status.Err = err
		status.Line = loc.line
		status.Col = loc.col
		return status
	}
	status.MPs = mps
	return status
}

func ReadMps() ([]data.MP, error) {
	status := ReadMpsValidated()
	if status.Err != nil {
		fmt.Println("[-] Could not read MP data")
		return nil, status.Err
	}
	return status.MPs, nil
}

func WriteMps(mps []data.MP) error {
	return write(mps, mpJsonFilename)
}

func write(v interface{}, filename string) error {
	var file *os.File

	// create/open file
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Println("[+] Creating file", filename)
		if file, err = os.Create(filename); err != nil {
			fmt.Println("[-] Could not create file", filename)
			return err
		}
	} else {
		file, err = os.OpenFile(filename, os.O_RDWR, os.ModeExclusive)
		if err != nil {
			fmt.Println("[-] Could not open file", filename)
			return err
		}
	}
	defer file.Close()

	// convert v to json bytes[]
	var buf bytes.Buffer

	encoder := json.NewEncoder(&buf)
	encoder.SetIndent(" ", "  ")
	err := encoder.Encode(v)
	if err != nil {
		fmt.Println("[-] Could serialize JSON before writing to", filename)
		return err
	}

	// write json to file
	_, err = file.Write(buf.Bytes())
	if err != nil {
		fmt.Println("[-] Could not write", buf.Len(), "bytes of JSON to", filename)
	}
	return err
}

type filePos struct {
	line, col int
}

// decodeMpsStrict requires a single root JSON array of MP objects and rejects trailing garbage.
func decodeMpsStrict(buf []byte) ([]data.MP, filePos, error) {
	dec := json.NewDecoder(bytes.NewReader(buf))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		pos, ann := annotateJSONErr(buf, err, 0)
		if pos.line == 0 {
			pos = offsetToLineCol(buf, dec.InputOffset())
		}
		return nil, pos, ann
	}
	rawStart := dec.InputOffset() - int64(len(raw))

	if tok, err := dec.Token(); err != io.EOF {
		off := dec.InputOffset()
		if err != nil {
			pos, ann := annotateJSONErr(buf, err, 0)
			if pos.line == 0 {
				pos = offsetToLineCol(buf, off)
			}
			return nil, pos, fmt.Errorf("trailing garbage after JSON value: %w", ann)
		}
		pos := offsetToLineCol(buf, off)
		return nil, pos, fmt.Errorf("trailing garbage after JSON value: %v", tok)
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		pos := offsetToLineCol(buf, rawStart)
		return nil, pos, fmt.Errorf("root must be a JSON array")
	}

	var mps []data.MP
	if err := json.Unmarshal(raw, &mps); err != nil {
		pos, ann := annotateJSONErr(buf, err, rawStart)
		return nil, pos, ann
	}
	return mps, filePos{}, nil
}

// annotateJSONErr maps json.SyntaxError / UnmarshalTypeError offsets onto line/col in buf.
// baseOffset is added for errors whose Offset is relative to a sub-slice (e.g. RawMessage).
func annotateJSONErr(buf []byte, err error, baseOffset int64) (filePos, error) {
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		pos := offsetToLineCol(buf, baseOffset+syn.Offset)
		return pos, err
	}
	var typ *json.UnmarshalTypeError
	if errors.As(err, &typ) {
		pos := offsetToLineCol(buf, baseOffset+typ.Offset)
		return pos, err
	}
	return filePos{}, err
}

// offsetToLineCol converts a byte offset into 1-based line and column.
// Offset is treated as the number of bytes before the error position (Go json.SyntaxError convention).
func offsetToLineCol(buf []byte, offset int64) filePos {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(buf)) {
		offset = int64(len(buf))
	}
	line, col := 1, 1
	for i := int64(0); i < offset && i < int64(len(buf)); i++ {
		if buf[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return filePos{line: line, col: col}
}
