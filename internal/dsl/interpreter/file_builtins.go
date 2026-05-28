package interpreter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// decodeText tries UTF-8 first; if not valid, decodes as Windows-1251.
func decodeText(data []byte) string {
	s := string(data)
	if isValidUTF8(s) {
		return s
	}
	decoded, err := charmap.Windows1251.NewDecoder().Bytes(data)
	if err != nil {
		return s
	}
	return string(decoded)
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}

// ─── dslTextReader (ЧтениеТекста) ──────────────────────────────────────────

type dslTextReader struct {
	path    string
	content string
	lines   []string
	lineIdx int
	isOpen  bool
}

func (r *dslTextReader) Get(field string) any {
	switch field {
	case "открыта", "isopen":
		return r.isOpen
	case "путь", "path":
		return r.path
	}
	return nil
}

func (r *dslTextReader) Set(field string, val any) {}

func (r *dslTextReader) CallMethod(name string, args []any) any {
	switch name {
	case "открыть", "open":
		if r.path == "" {
			panic(userError{Msg: "ЧтениеТекста.Открыть: не указан путь к файлу"})
		}
		data, err := os.ReadFile(r.path)
		if err != nil {
			panic(userError{Msg: "ЧтениеТекста: ошибка чтения файла " + r.path + ": " + err.Error()})
		}
		r.content = decodeText(data)
		r.lines = strings.Split(r.content, "\n")
		r.lineIdx = 0
		r.isOpen = true
		return nil
	case "прочитать", "read":
		if !r.isOpen {
			panic(userError{Msg: "ЧтениеТекста.Прочитать: файл не открыт"})
		}
		return r.content
	case "прочитатьстроку", "readline":
		if !r.isOpen {
			panic(userError{Msg: "ЧтениеТекста.ПрочитатьСтроку: файл не открыт"})
		}
		if r.lineIdx >= len(r.lines) {
			return nil
		}
		line := r.lines[r.lineIdx]
		r.lineIdx++
		return line
	case "закрыть", "close":
		r.isOpen = false
		return nil
	}
	panic(userError{Msg: "ЧтениеТекста: неизвестный метод " + name})
}

// ─── dslTextWriter (ЗаписьТекста) ──────────────────────────────────────────

type dslTextWriter struct {
	path   string
	buf    strings.Builder
	isOpen bool
}

func (w *dslTextWriter) Get(field string) any {
	switch field {
	case "открыта", "isopen":
		return w.isOpen
	case "путь", "path":
		return w.path
	}
	return nil
}

func (w *dslTextWriter) Set(field string, val any) {}

func (w *dslTextWriter) CallMethod(name string, args []any) any {
	switch name {
	case "открыть", "open":
		if w.path == "" {
			panic(userError{Msg: "ЗаписьТекста.Открыть: не указан путь к файлу"})
		}
		w.buf.Reset()
		w.isOpen = true
		return nil
	case "записать", "write":
		if !w.isOpen {
			panic(userError{Msg: "ЗаписьТекста.Записать: файл не открыт"})
		}
		if len(args) > 0 {
			w.buf.WriteString(fmt.Sprintf("%v", args[0]))
		}
		return nil
	case "записатьстроку", "writeline":
		if !w.isOpen {
			panic(userError{Msg: "ЗаписьТекста.ЗаписатьСтроку: файл не открыт"})
		}
		if len(args) > 0 {
			w.buf.WriteString(fmt.Sprintf("%v", args[0]))
		}
		w.buf.WriteByte('\n')
		return nil
	case "закрыть", "close":
		if w.isOpen && w.path != "" {
			err := os.WriteFile(w.path, []byte(w.buf.String()), 0644)
			if err != nil {
				panic(userError{Msg: "ЗаписьТекста: ошибка записи файла " + w.path + ": " + err.Error()})
			}
		}
		w.isOpen = false
		return nil
	}
	panic(userError{Msg: "ЗаписьТекста: неизвестный метод " + name})
}

// ─── dslFile (Файл) ───────────────────────────────────────────────────────

type dslFile struct {
	path string
	info os.FileInfo
}

func (f *dslFile) loadInfo() {
	if f.info == nil {
		f.info, _ = os.Stat(f.path)
	}
}

func (f *dslFile) Get(field string) any {
	f.loadInfo()
	switch field {
	case "существует", "exists":
		return f.info != nil
	case "размер", "size":
		if f.info != nil {
			return float64(f.info.Size())
		}
		return float64(0)
	case "полноеимя", "fullname":
		return f.path
	case "имя", "name":
		return filepath.Base(f.path)
	case "расширение", "extension":
		return filepath.Ext(f.path)
	case "имябезрасширения", "namewithoutextension":
		name := filepath.Base(f.path)
		ext := filepath.Ext(name)
		return name[:len(name)-len(ext)]
	}
	return nil
}

func (f *dslFile) Set(field string, val any) {}

func (f *dslFile) CallMethod(name string, args []any) any {
	switch name {
	case "существует", "exists":
		f.info = nil
		f.loadInfo()
		return f.info != nil
	}
	panic(userError{Msg: "Файл: неизвестный метод " + name})
}

// ─── DecodeFile builtin ───────────────────────────────────────────────────

// decodeFileBuiltin converts raw bytes (Windows-1251 → UTF-8) when needed.
// Used for uploaded file content that may not be UTF-8.
func decodeFileBuiltin(args []any, file string, line int) (any, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("ДекодироватьФайл: требуется текст")
	}
	s := fmt.Sprintf("%v", args[0])
	if isValidUTF8(s) {
		return s, nil
	}
	decoded, err := charmap.Windows1251.NewDecoder().String(s)
	if err != nil {
		return s, nil
	}
	return decoded, nil
}

// ─── NewFileFunctions ──────────────────────────────────────────────────────

func NewFileFunctions() map[string]any {
	m := map[string]any{}

	textReaderFactory := func(args []any) any {
		path := strArg(args, 0)
		return &dslTextReader{path: path}
	}
	textWriterFactory := func(args []any) any {
		path := strArg(args, 0)
		return &dslTextWriter{path: path}
	}
	fileFactory := func(args []any) any {
		path := strArg(args, 0)
		return &dslFile{path: path}
	}

	m["__factory_ЧтениеТекста"] = textReaderFactory
	m["__factory_TextReader"] = textReaderFactory
	m["__factory_ЗаписьТекста"] = textWriterFactory
	m["__factory_TextWriter"] = textWriterFactory
	m["__factory_Файл"] = fileFactory
	m["__factory_File"] = fileFactory

	m["ДекодироватьФайл"] = BuiltinFunc(decodeFileBuiltin)
	m["DecodeFile"] = BuiltinFunc(decodeFileBuiltin)

	return m
}
