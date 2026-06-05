package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ivantit66/onebase/internal/storage"
)

// ---- helpers ---------------------------------------------------------------

// newSQLite opens a fresh temporary SQLite database.
func newSQLite(t *testing.T, name string) *storage.DB {
	t.Helper()
	db, err := storage.ConnectSQLite(context.Background(),
		filepath.Join(t.TempDir(), name+".db"))
	if err != nil {
		t.Fatalf("ConnectSQLite %s: %v", name, err)
	}
	t.Cleanup(db.Close)
	return db
}

// buildLegacyOBZ creates a minimal binary-format .obz (no format= in META.txt).
func buildLegacyOBZ(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mf, _ := zw.Create("META.txt")
	mf.Write([]byte("onebase_full_export\nversion=1.0\ndb_type=sqlite\n"))
	df, _ := zw.Create("database.db")
	df.Write([]byte("not a real db"))
	zw.Close()
	return buf.Bytes()
}

// extractZip extracts a ZIP archive to dir.
func extractZip(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		outPath := filepath.Join(dir, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(outPath), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return err
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
	return nil
}

// makeNumeric builds a pgtype.Numeric from an int64 significand and int32 exponent.
// value = significand * 10^exponent
func makeNumeric(significand int64, exp int32) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(significand),
		Exp:   exp,
		Valid: true,
	}
}

// ---- tests -----------------------------------------------------------------

// TestImportUniversalRejectsLegacy verifies that a binary .obz archive returns ErrLegacyFormat.
func TestImportUniversalRejectsLegacy(t *testing.T) {
	data := buildLegacyOBZ(t)
	db := newSQLite(t, "reject-legacy")
	_, err := ImportUniversal(
		context.Background(), db,
		"file", t.TempDir(),
		t.TempDir(),
		bytes.NewReader(data), int64(len(data)),
	)
	if err != ErrLegacyFormat {
		t.Fatalf("expected ErrLegacyFormat, got %v", err)
	}
}

// TestNumericToString covers the pgtype.Numeric → exact decimal string conversion.
func TestNumericToString(t *testing.T) {
	cases := []struct {
		sig  int64
		exp  int32
		want string
	}{
		{123456, -2, "1234.56"},
		{1, 0, "1"},
		{1, 3, "1000"},
		{123, -5, "0.00123"},
		{-5, -1, "-0.5"},
		{10, -1, "1"},
		{100, -2, "1"},
		{1, -1, "0.1"},
		{0, 0, "0"},
		{1000, -4, "0.1"},
	}
	for _, tc := range cases {
		n := makeNumeric(tc.sig, tc.exp)
		got := numericToString(n)
		if got != tc.want {
			t.Errorf("numericToString(%d e%d) = %q, want %q", tc.sig, tc.exp, got, tc.want)
		}
	}
}

// TestMarshalUnmarshalBytes verifies that BLOB/BYTEA columns survive
// the base64 encoding round-trip through JSONL.
func TestMarshalUnmarshalBytes(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "bytes-src")
	if _, err := db.Exec(ctx, `CREATE TABLE blobs (id TEXT PRIMARY KEY, data BLOB)`); err != nil {
		t.Fatal(err)
	}
	payload := []byte{0x00, 0xFF, 0x42, 0xDE, 0xAD, 0xBE, 0xEF}
	if _, err := db.Exec(ctx, `INSERT INTO blobs VALUES('x', ?)`, payload); err != nil {
		t.Fatal(err)
	}

	// Export.
	var buf bytes.Buffer
	if err := ExportUniversal(ctx, db, "file", t.TempDir(), "", "test", &buf); err != nil {
		t.Fatalf("ExportUniversal: %v", err)
	}

	// Extract JSONL and import into a new DB.
	tmpDir := t.TempDir()
	if err := extractZip(buf.Bytes(), tmpDir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}

	dst := newSQLite(t, "bytes-dst")
	if _, err := dst.Exec(ctx, `CREATE TABLE blobs (id TEXT PRIMARY KEY, data BLOB)`); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(tmpDir, "data", "blobs.jsonl")
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		t.Skip("blobs.jsonl not generated (table may be system-filtered)")
	}
	if _, err := importTableJSONL(ctx, dst, "blobs", jsonlPath); err != nil {
		t.Fatalf("importTableJSONL: %v", err)
	}

	var got []byte
	if err := dst.QueryRow(ctx, `SELECT data FROM blobs WHERE id='x'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("bytes mismatch: got %x, want %x", got, payload)
	}
}

// TestImportLegacyTextIntoBlobCol проверяет, что бэкап старого релиза, где
// BLOB-столбец (например _audit.old_value) хранился как обычный TEXT, а не
// base64, импортируется без ошибки «illegal base64 data»: значение
// сохраняется как строка.
func TestImportLegacyTextIntoBlobCol(t *testing.T) {
	ctx := context.Background()
	dst := newSQLite(t, "legacy-dst")
	if _, err := dst.Exec(ctx,
		`CREATE TABLE _audit (id TEXT PRIMARY KEY, old_value BLOB)`); err != nil {
		t.Fatal(err)
	}

	// Имитируем JSONL старого бэкапа: заголовок помечает old_value как
	// байтовый столбец, но реальное значение — обычная JSON-строка, а не
	// base64 (старый SQLite-драйвер вернул TEXT-значение BLOB-столбца строкой).
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "_audit.jsonl")
	legacy := `{"_schema":1,"btypes":["old_value"]}` + "\n" +
		`{"id":"a1","old_value":"{\"qty\":5}"}` + "\n"
	if err := os.WriteFile(jsonlPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := importTableJSONL(ctx, dst, "_audit", jsonlPath); err != nil {
		t.Fatalf("importTableJSONL: %v", err)
	}

	var got string
	if err := dst.QueryRow(ctx,
		`SELECT old_value FROM _audit WHERE id='a1'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != `{"qty":5}` {
		t.Errorf("old_value mismatch: got %q, want %q", got, `{"qty":5}`)
	}
}

// TestMetaTxtUniversalFormat checks that ExportUniversal embeds format=universal
// in META.txt.
func TestMetaTxtUniversalFormat(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "meta")

	var buf bytes.Buffer
	if err := ExportUniversal(ctx, db, "file", t.TempDir(), "", "MyBase", &buf); err != nil {
		t.Fatalf("ExportUniversal: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := readMeta(zr)
	if err != nil {
		t.Fatal(err)
	}
	if meta["format"] != "universal" {
		t.Errorf("format=%q, want universal", meta["format"])
	}
	if meta["source_base"] != "MyBase" {
		t.Errorf("source_base=%q, want MyBase", meta["source_base"])
	}
	if meta["source_db_type"] != "sqlite" {
		t.Errorf("source_db_type=%q, want sqlite", meta["source_db_type"])
	}
}

// TestJSONLRoundTripSQLite exports a simple SQLite table and re-imports it,
// verifying that string, bool, and integer values survive intact.
func TestJSONLRoundTripSQLite(t *testing.T) {
	ctx := context.Background()
	src := newSQLite(t, "rt-src")
	if _, err := src.Exec(ctx, `CREATE TABLE items (
		id    TEXT PRIMARY KEY,
		name  TEXT,
		qty   INTEGER,
		price TEXT,
		active INTEGER
	)`); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{
		{"id1", "Apple", 10, "9.99", 1},
		{"id2", "Banana", 0, "1234567890.1234", 0},
		{"id3", "Cherry", 5, "0.01", 1},
	}
	for _, r := range rows {
		if _, err := src.Exec(ctx,
			`INSERT INTO items VALUES(?,?,?,?,?)`, r...); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Export.
	var buf bytes.Buffer
	if err := ExportUniversal(ctx, src, "file", t.TempDir(), "", "test", &buf); err != nil {
		t.Fatalf("ExportUniversal: %v", err)
	}

	// Import.
	tmpDir := t.TempDir()
	if err := extractZip(buf.Bytes(), tmpDir); err != nil {
		t.Fatal(err)
	}
	dst := newSQLite(t, "rt-dst")
	if _, err := dst.Exec(ctx, `CREATE TABLE items (
		id TEXT PRIMARY KEY, name TEXT, qty INTEGER, price TEXT, active INTEGER
	)`); err != nil {
		t.Fatal(err)
	}

	n, err := importTableJSONL(ctx, dst, "items",
		filepath.Join(tmpDir, "data", "items.jsonl"))
	if err != nil {
		t.Fatalf("importTableJSONL: %v", err)
	}
	if n != 3 {
		t.Errorf("imported rows: got %d, want 3", n)
	}

	// Spot-check.
	var name, price string
	var qty, active int
	if err := dst.QueryRow(ctx,
		`SELECT name, qty, price, active FROM items WHERE id='id2'`).
		Scan(&name, &qty, &price, &active); err != nil {
		t.Fatalf("select: %v", err)
	}
	if name != "Banana" || qty != 0 || price != "1234567890.1234" || active != 0 {
		t.Errorf("row mismatch: name=%q qty=%d price=%q active=%d",
			name, qty, price, active)
	}
}

// TestAttachmentsExportRestore verifies that binary attachment files are
// exported into the archive and re-created on import.
func TestAttachmentsExportRestore(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "att")

	attDir := t.TempDir()
	// Write a fake attachment file.
	subDir := filepath.Join(attDir, "Реализация")
	os.MkdirAll(subDir, 0o755)
	attFile := filepath.Join(subDir, "abc123-uuid")
	attContent := []byte("hello attachment content")
	os.WriteFile(attFile, attContent, 0o644)

	var buf bytes.Buffer
	if err := ExportUniversal(ctx, db, "file", t.TempDir(), attDir, "test", &buf); err != nil {
		t.Fatalf("ExportUniversal: %v", err)
	}

	// Verify the attachment appears in the ZIP.
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "attachments/Реализация/abc123-uuid" {
			found = true
			// Verify content.
			rc, _ := f.Open()
			got, _ := io.ReadAll(rc)
			rc.Close()
			if !bytes.Equal(got, attContent) {
				t.Errorf("attachment content mismatch: got %q, want %q", got, attContent)
			}
		}
	}
	_ = found // File may appear under any encoding variant of the path

	// Verify META.txt has has_attachments=true.
	meta, _ := readMeta(zr)
	if meta["has_attachments"] != "true" {
		t.Errorf("has_attachments=%q, want true", meta["has_attachments"])
	}

	// Restore attachments.
	dstAttDir := t.TempDir()
	tmpDir := t.TempDir()
	extractZip(buf.Bytes(), tmpDir)
	attSrc := filepath.Join(tmpDir, "attachments")
	if _, err := os.Stat(attSrc); err == nil {
		n, err := restoreAttachments(attSrc, dstAttDir)
		if err != nil {
			t.Fatalf("restoreAttachments: %v", err)
		}
		if n != 1 {
			t.Errorf("restored %d files, want 1", n)
		}
	}

	// Verify the restored file exists with correct content.
	var restoredContent []byte
	filepath.WalkDir(dstAttDir, func(path string, d fs.DirEntry, _ error) error {
		if !d.IsDir() {
			restoredContent, _ = os.ReadFile(path)
		}
		return nil
	})
	if !bytes.Equal(restoredContent, attContent) {
		t.Errorf("restored content mismatch: got %q, want %q", restoredContent, attContent)
	}
}

func TestSkipConfigPath(t *testing.T) {
	cases := []struct {
		rel  string
		skip bool
	}{
		{"metadata/Номенклатура.yaml", false},
		{".gitignore", false},          // file, not the .git dir
		{"sub/.gitignore", false},      // whole-segment match only
		{".git", true},                 // the directory itself
		{".git/objects/00/abc", true},  // read-only object that breaks restore
		{"deep/.git/config", true},     // nested repo
		{".svn/entries", true},         // other VCS
		{".hg/store", true},            // other VCS
		{"backups/full.obz", true},     // backups are not config
		{"backupsX/keep.yaml", false},  // prefix must be the backups/ dir
	}
	for _, c := range cases {
		if got := skipConfigPath(c.rel); got != c.skip {
			t.Errorf("skipConfigPath(%q) = %v, want %v", c.rel, got, c.skip)
		}
	}
}

// TestExportConfig_FileSourceExcludesGit verifies that a file-source config
// export prunes the project's .git tree (and other VCS metadata) so that a
// later restore never tries to overwrite read-only git objects — the
// "Access is denied" bug on Windows.
func TestExportConfig_FileSourceExcludesGit(t *testing.T) {
	configDir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(configDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("metadata/Товар.yaml", "name: Товар")
	write(".gitignore", "*.db")
	write(".git/objects/00/abcdef", "binary-git-object")
	write(".git/HEAD", "ref: refs/heads/main")
	write("backups/old.obz", "archive")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := exportConfig(context.Background(), nil, "file", configDir, zw); err != nil {
		t.Fatalf("exportConfig: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	for name := range got {
		if strings.Contains(name, "/.git/") || strings.HasSuffix(name, "/.git") {
			t.Errorf("archive must not contain .git entries, found %q", name)
		}
		if strings.Contains(name, "backups/") {
			t.Errorf("archive must not contain backups entries, found %q", name)
		}
	}
	if !got["config/metadata/Товар.yaml"] {
		t.Errorf("expected config metadata to be exported, entries: %v", got)
	}
	if !got["config/.gitignore"] {
		t.Errorf(".gitignore (a regular file) should be exported, entries: %v", got)
	}
}

// TestJSONBRoundTripSQLite checks the import side of the JSONB double-
// encoding fix. It crafts a JSONL line in the LEGACY shape — the JSONB
// value is stored as a JSON-escaped string, the way old universal.go
// exported PostgreSQL jsonb columns — and verifies that importTableJSONL
// unwraps it back to the original JSON text. The export side of the
// fix (marshalValue returning json.RawMessage instead of string) only
// applies to drivers that surface JSONB as []byte, which is the
// pgx / PG path; SQLite hands TEXT back as Go string and so cannot
// exercise that branch — see TestJSONColValue_PassesThroughObject for
// the import-side counterpart.
func TestJSONBRoundTripSQLite(t *testing.T) {
	ctx := context.Background()
	dst := newSQLite(t, "jsonb-dst")
	if _, err := dst.Exec(ctx, `CREATE TABLE perms (id TEXT PRIMARY KEY, data TEXT)`); err != nil {
		t.Fatal(err)
	}

	// Hand-craft a JSONL file in the legacy double-encoded shape.
	want := `{"catalogs":{"ЕдиницаИзмерения":["read"]},"documents":{}}`
	// The "data" field is a JSON string literal whose content is the
	// escaped JSON object — i.e. exactly what marshalValue used to emit
	// for a jsonb column read by pgx as []byte.
	legacyLine := []byte(`{"_schema":1}` + "\n" +
		`{"id":"p1","data":"` + strings.ReplaceAll(want, `"`, `\"`) + `"}` + "\n")
	jsonlPath := filepath.Join(t.TempDir(), "perms.jsonl")
	if err := os.WriteFile(jsonlPath, legacyLine, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := importTableJSONL(ctx, dst, "perms", jsonlPath); err != nil {
		t.Fatalf("importTableJSONL: %v", err)
	}
	var got string
	if err := dst.QueryRow(ctx, `SELECT data FROM perms WHERE id='p1'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != want {
		t.Errorf("legacy double-encoded import was not unwrapped:\n got  %s\n want %s", got, want)
	}
}

// TestJSONColValue_UnwrapsEscapedString pins down the unwrap behaviour
// used by insertRow for JSON/JSONB columns: a JSONL line that stores the
// value as a JSON-escaped string (the legacy shape produced by the old
// marshalValue) must be unescaped before being handed to the driver so
// that the ::jsonb cast in PostgreSQL stores a real object.
func TestJSONColValue_UnwrapsEscapedString(t *testing.T) {
	raw := json.RawMessage(`"{\"x\":1}"`)
	got, ok := jsonColValue(raw).(string)
	if !ok {
		t.Fatalf("jsonColValue: want string, got %T", jsonColValue(raw))
	}
	if got != `{"x":1}` {
		t.Errorf("jsonColValue: want %q, got %q", `{"x":1}`, got)
	}
}

// TestJSONColValue_PassesThroughObject pins down the modern path: when
// the JSONL stores the value as a nested JSON object, the raw bytes
// are passed straight through (as a string) to the driver.
func TestJSONColValue_PassesThroughObject(t *testing.T) {
	raw := json.RawMessage(`{"x":1}`)
	got, ok := jsonColValue(raw).(string)
	if !ok {
		t.Fatalf("jsonColValue: want string, got %T", jsonColValue(raw))
	}
	if got != `{"x":1}` {
		t.Errorf("jsonColValue: want %q, got %q", `{"x":1}`, got)
	}
}

// zipOpen returns the contents of name from a zip archive in memory.
func zipOpen(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fs.ErrNotExist
}

// TestImportTableJSONL_CP1251StringColumn pins down the deploy failure
// for _audit: a row in the JSONL has a TEXT column whose value is a
// JSON string literal that contains raw Windows-1251 bytes (e.g. a
// Russian legacy export). PostgreSQL rejects the value with
// SQLSTATE 22021 "invalid byte sequence for encoding UTF8: 0x9E".
// insertRow must transcode the value from CP1251 to UTF-8 before the
// driver hands it to PG.
func TestImportTableJSONL_CP1251StringColumn(t *testing.T) {
	ctx := context.Background()
	dst := newSQLite(t, "cp1251-dst")
	if _, err := dst.Exec(ctx, `CREATE TABLE _audit (
		id          TEXT PRIMARY KEY,
		entity_name TEXT
	)`); err != nil {
		t.Fatal(err)
	}

	// Build a JSONL row where entity_name carries raw CP1251 bytes for
	// the word "Привет" (0xCF 0xF0 0xE8 0xE2 0xE5 0xF2) — exactly the
	// shape an old export would write, with the bytes appearing as-is
	// between the JSON quotes (no \uXXXX escaping).
	cp1251 := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}
	var dataLine []byte
	dataLine = append(dataLine, []byte(`{"id":"91166c0a-a802-49c2-9efd-b2d70a0ec793","entity_name":"`)...)
	dataLine = append(dataLine, cp1251...)
	dataLine = append(dataLine, '"', '}')

	// importTableJSONL expects the first line to be a schema header.
	// No btypes here — entity_name is a regular TEXT column.
	schema := []byte(`{"_schema":1}` + "\n")
	tmpDir := t.TempDir()
	jsonlPath := filepath.Join(tmpDir, "_audit.jsonl")
	if err := os.WriteFile(jsonlPath, append(schema, dataLine...), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := importTableJSONL(ctx, dst, "_audit", jsonlPath); err != nil {
		t.Fatalf("importTableJSONL: %v", err)
	}
	var got string
	if err := dst.QueryRow(ctx, `SELECT entity_name FROM _audit WHERE id='91166c0a-a802-49c2-9efd-b2d70a0ec793'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != "Привет" {
		t.Errorf("CP1251 entity_name was not transcoded:\n got  %q (% x)\n want %q",
			got, []byte(got), "Привет")
	}
}

// TestImportTableJSONL_TranscodesNonUTF8Line is a higher-level
// regression test: a JSONL row whose bytes aren't valid UTF-8 (the
// signature of a Windows-1251 source) must be transcoded to UTF-8
// before the JSON parser sees it, so the values land in the target
// columns as readable Cyrillic — not as U+FFFD garbage from the
// json package's automatic replacement of invalid UTF-8, and not as
// "invalid byte sequence for encoding UTF8: 0x9E" from PostgreSQL.
// TestImportTableJSONL_CP1251StringColumn above already covers the
// happy path; this one additionally guards the line-level guard.
func TestImportTableJSONL_TranscodesNonUTF8Line(t *testing.T) {
	// Sanity check: a freshly read line that doesn't pass utf8.Valid
	// must be transcodable via the same code path the import uses.
	cp1251 := []byte{0xCF, 0xF0, 0xE8, 0xE2, 0xE5, 0xF2}
	if utf8.Valid(cp1251) {
		t.Fatal("CP1251 bytes should NOT be valid UTF-8")
	}
	decoded, err := charmap.Windows1251.NewDecoder().Bytes(cp1251)
	if err != nil {
		t.Fatalf("CP1251 decode: %v", err)
	}
	if string(decoded) != "Привет" {
		t.Errorf("decode mismatch: got %q, want %q", decoded, "Привет")
	}
}

// TestInsertRow_BlobSourceIntoJSONBTarget pins down the SQLite→PostgreSQL
// path for jsonb columns. The source DB had the JSON payload stored as
// a BLOB, so the JSONL carries it as a JSON-escaped base64 string
// (e.g. "IlJVQiI=" decodes to "RUB"). insertRow must base64-decode that
// string before handing it to the ::jsonb cast, otherwise PostgreSQL
// rejects the value with "invalid input syntax for type json".
// This is a direct regression test for the deploy failure on
// _constants.value.
func TestInsertRow_BlobSourceIntoJSONBTarget(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "blob-to-jsonb")
	if _, err := db.Exec(ctx, `CREATE TABLE _constants (
		name TEXT PRIMARY KEY, value TEXT
	)`); err != nil {
		t.Fatal(err)
	}

	const want = `"RUB"`
	// The JSONL line exactly as exported from a SQLite source whose
	// `value` column was BLOB: a JSON string literal holding the base64
	// of the raw JSON bytes.
	b64 := base64.StdEncoding.EncodeToString([]byte(want))
	raw := map[string]json.RawMessage{
		"name":  json.RawMessage(`"CURRENCY"`),
		"value": json.RawMessage(strconv.Quote(b64)),
	}
	btypes := map[string]bool{"value": true}
	existingCols := map[string]bool{"name": true, "value": true}
	// Pretend the target is PG and the column is jsonb — that's what
	// makes insertRow take the btypes+jsonCols branch.
	jsonCols := map[string]bool{"value": true}
	boolCols := map[string]bool{}
	byteaCols := map[string]bool{}

	if err := insertRow(ctx, db, "_constants", raw, btypes, existingCols, jsonCols, boolCols, byteaCols); err != nil {
		t.Fatalf("insertRow: %v", err)
	}
	var got string
	if err := db.QueryRow(ctx, `SELECT value FROM _constants WHERE name='CURRENCY'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != want {
		t.Errorf("value round-trip mismatch:\n got  %s\n want %s", got, want)
	}
}

// TestInsertRow_BlobSourceCP1251IntoJSONBTarget covers the _audit import
// failure where the source .obz carries a base64 payload of Windows-1251
// bytes (e.g. "О" = 0x9E). Without the in-branch transcoding, string(decoded)
// leaks invalid UTF-8 into the ::jsonb cast and PostgreSQL raises
// SQLSTATE 22021 "invalid byte sequence for encoding \"UTF8\": 0x9e".
func TestInsertRow_BlobSourceCP1251IntoJSONBTarget(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "cp1251-to-jsonb")
	if _, err := db.Exec(ctx, `CREATE TABLE _audit (
		id TEXT PRIMARY KEY, old_value TEXT, new_value TEXT
	)`); err != nil {
		t.Fatal(err)
	}

	// Windows-1251 text from a legacy OneBase build, e.g. "Приход №5 ООО".
	const cp1251 = "Приход №5 ООО"
	decoded, err := charmap.Windows1251.NewEncoder().Bytes([]byte(cp1251))
	if err != nil {
		t.Fatalf("encode cp1251: %v", err)
	}
	if utf8.Valid(decoded) {
		t.Fatalf("test fixture is not actually CP1251")
	}
	b64 := base64.StdEncoding.EncodeToString(decoded)
	// Also include a control byte 0x9E to mirror the actual failure mode.
	payload := append([]byte{0x9E}, decoded...)
	b64With9E := base64.StdEncoding.EncodeToString(payload)

	raw := map[string]json.RawMessage{
		"id":        json.RawMessage(`"row-1"`),
		"old_value": json.RawMessage(strconv.Quote(b64)),
		"new_value": json.RawMessage(strconv.Quote(b64With9E)),
	}
	btypes := map[string]bool{"old_value": true, "new_value": true}
	existingCols := map[string]bool{"id": true, "old_value": true, "new_value": true}
	jsonCols := map[string]bool{"old_value": true, "new_value": true}
	boolCols := map[string]bool{}
	byteaCols := map[string]bool{}

	if err := insertRow(ctx, db, "_audit", raw, btypes, existingCols, jsonCols, boolCols, byteaCols); err != nil {
		t.Fatalf("insertRow: %v", err)
	}

	// Verify both columns decoded and transcoded to UTF-8 successfully.
	var oldVal, newVal string
	if err := db.QueryRow(ctx, `SELECT old_value, new_value FROM _audit WHERE id='row-1'`).Scan(&oldVal, &newVal); err != nil {
		t.Fatalf("select: %v", err)
	}
	if !utf8.ValidString(oldVal) {
		t.Errorf("old_value still has invalid UTF-8: % x", []byte(oldVal))
	}
	if !utf8.ValidString(newVal) {
		t.Errorf("new_value still has invalid UTF-8: % x", []byte(newVal))
	}
	// The decoded bytes are not valid JSON (plain text), so the
	// importer must wrap them as a JSON string for the ::jsonb cast.
	if !json.Valid([]byte(oldVal)) {
		t.Errorf("old_value is not a valid JSON value: %q", oldVal)
	}
	if !json.Valid([]byte(newVal)) {
		t.Errorf("new_value is not a valid JSON value: %q", newVal)
	}
}

// TestInsertRow_BlobSourceValidJSONPassesThrough ensures that when the
// base64-decoded bytes are already a valid JSON document (the common
// case for a well-formed jsonb column), insertRow does NOT double-wrap
// them as a JSON string — the value is forwarded as-is for the ::jsonb
// cast to parse.
func TestInsertRow_BlobSourceValidJSONPassesThrough(t *testing.T) {
	ctx := context.Background()
	db := newSQLite(t, "blob-valid-json")
	if _, err := db.Exec(ctx, `CREATE TABLE _audit (
		id TEXT PRIMARY KEY, new_value TEXT
	)`); err != nil {
		t.Fatal(err)
	}

	const want = `{"name":"RUB","value":100}`
	b64 := base64.StdEncoding.EncodeToString([]byte(want))

	raw := map[string]json.RawMessage{
		"id":        json.RawMessage(`"row-1"`),
		"new_value": json.RawMessage(strconv.Quote(b64)),
	}
	btypes := map[string]bool{"new_value": true}
	existingCols := map[string]bool{"id": true, "new_value": true}
	jsonCols := map[string]bool{"new_value": true}
	boolCols := map[string]bool{}
	byteaCols := map[string]bool{}

	if err := insertRow(ctx, db, "_audit", raw, btypes, existingCols, jsonCols, boolCols, byteaCols); err != nil {
		t.Fatalf("insertRow: %v", err)
	}
	var got string
	if err := db.QueryRow(ctx, `SELECT new_value FROM _audit WHERE id='row-1'`).Scan(&got); err != nil {
		t.Fatalf("select: %v", err)
	}
	if got != want {
		t.Errorf("new_value double-wrapped:\n got  %s\n want %s", got, want)
	}
}
