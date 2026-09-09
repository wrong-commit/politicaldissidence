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
	"strings"

	"politicaldissidence/data"
)

const (
	// DefaultMPJSONFilename is the on-disk MP database when MP_DATA_PATH is unset.
	DefaultMPJSONFilename = "mp_data.json"

	// EnvMPDataPath overrides the path used by ReadMps / WriteMps / ReadMpsValidated.
	EnvMPDataPath = "MP_DATA_PATH"
)

// MPJSONPath returns the MP JSON file path: trimmed MP_DATA_PATH if set, else DefaultMPJSONFilename.
func MPJSONPath() string {
	if v := strings.TrimSpace(os.Getenv(EnvMPDataPath)); v != "" {
		return v
	}
	return DefaultMPJSONFilename
}

// MPJSONStatus is the outcome of reading and validating the MP JSON file.
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

// ReadMpsValidated reads and validates the configured MP JSON file (see MPJSONPath).
func ReadMpsValidated() MPJSONStatus {
	return ValidateMPJSONFile(MPJSONPath())
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
	return write(mps, MPJSONPath())
}

// WriteMpsTo writes MPs as validated indented JSON to path (atomic temp + replace).
func WriteMpsTo(path string, mps []data.MP) error {
	return write(mps, path)
}

// write encodes v as indented JSON to a uniquely named temp file next to
// filename, revalidates that temp file can be reloaded, then replaces the
// destination (delete + rename) so a failed write never corrupts the live DB.
func write(v interface{}, filename string) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent(" ", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Println("[-] Could serialize JSON before writing to", filename)
		return err
	}

	dir := filepath.Dir(filename)
	if dir == "" || dir == "." {
		dir = "."
	}
	// Random suffix avoids collisions when concurrent saves both write a tmp.
	tmp, err := os.CreateTemp(dir, "mp_data.*.tmp")
	if err != nil {
		fmt.Println("[-] Could not create temp file for", filename)
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		fmt.Println("[-] Could not write", buf.Len(), "bytes of JSON to", tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	status := ValidateMPJSONFile(tmpPath)
	if !status.Valid() {
		fmt.Println("[-] Temp MP JSON failed validation before replace:", status.LogMessage())
		return status.Err
	}

	if err := replaceFile(tmpPath, filename); err != nil {
		fmt.Println("[-] Could not replace", filename, "with", tmpPath)
		return err
	}
	cleanup = false
	return nil
}

// replaceFile deletes dest (if present) then renames src onto dest.
// Same-directory rename keeps the swap as atomic as the OS allows; on Windows
// rename cannot overwrite, so delete-then-rename is required.
func replaceFile(src, dest string) error {
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dest)
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
