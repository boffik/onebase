package equipment

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

func init() {
	Register("display", func() Device { return &displayDevice{width: 20} })
	Register("display_tcp", func() Device { return &displayDevice{width: 20} })
}

// displayDevice — драйвер дисплея покупателя (VFD) по протоколу CD5220 поверх TCP.
// Типовой дисплей — 20 символов × 2 строки. Транспорт — io.WriteCloser, как у
// escpos: serial добавится отдельным Connect без правки вывода.
type displayDevice struct {
	conn   io.WriteCloser
	width  int
	encode func(string) []byte // кодировка текста (по умолчанию UTF-8, опц. CP866)
}

// Команды CD5220.
var (
	dispInit  = []byte{0x1B, 0x40}       // ESC @         — инициализация
	dispClear = []byte{0x0C}             // CLR           — очистить экран
	dispUpper = []byte{0x1B, 0x51, 0x41} // ESC Q A … CR  — верхняя строка
	dispLower = []byte{0x1B, 0x51, 0x42} // ESC Q B … CR  — нижняя строка
	dispCR    = []byte{0x0D}
)

func (d *displayDevice) Kind() string { return "дисплей_покупателя" }

func (d *displayDevice) Connect(params map[string]string) error {
	if w := firstNonEmpty(params["ширина"], params["width"]); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			d.width = n
		}
	}
	// Кодировка текста: по умолчанию UTF-8 (как было), опционально CP866 для
	// реальных VFD, где UTF-8 даёт кракозябры.
	d.encode = deviceEncoder(firstNonEmpty(params["кодировка"], params["encoding"]))
	conn, err := openWriteTransport(params)
	if err != nil {
		return err
	}
	d.conn = conn
	return nil
}

func (d *displayDevice) Disconnect() error {
	if d.conn == nil {
		return nil
	}
	err := d.conn.Close()
	d.conn = nil
	return err
}

// ShowLines выводит строки: первая — верхняя, вторая — нижняя. Лишние строки
// игнорируются, недостающие выводятся пустыми; каждая обрезается/дополняется
// до ширины дисплея.
func (d *displayDevice) ShowLines(lines []string) error {
	var buf bytes.Buffer
	buf.Write(dispInit)
	buf.Write(dispClear)
	for i, cmd := range [][]byte{dispUpper, dispLower} {
		text := ""
		if i < len(lines) {
			text = lines[i]
		}
		buf.Write(cmd)
		buf.Write(d.encode(d.fit(text)))
		buf.Write(dispCR)
	}
	return d.write(buf.Bytes())
}

func (d *displayDevice) Clear() error {
	return d.write(append(append([]byte{}, dispInit...), dispClear...))
}

func (d *displayDevice) write(chunk []byte) error {
	if d.conn == nil {
		return fmt.Errorf("устройство не подключено")
	}
	_, err := d.conn.Write(chunk)
	return err
}

// fit обрезает или дополняет строку пробелами до ширины дисплея (по рунам).
func (d *displayDevice) fit(s string) string {
	n := utf8.RuneCountInString(s)
	if n > d.width {
		return string([]rune(s)[:d.width])
	}
	return s + strings.Repeat(" ", d.width-n)
}
