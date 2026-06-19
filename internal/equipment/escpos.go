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
	Register("escpos", func() Device { return &escposDevice{width: 42} })
	Register("escpos_tcp", func() Device { return &escposDevice{width: 42} })
}

// escposDevice — драйвер чекового принтера по протоколу ESC/POS.
//
// Транспорт сейчас TCP (сетевые термопринтеры с Ethernet-портом, обычно :9100).
// Поле conn — io.WriteCloser, поэтому serial/USB добавляется отдельным Connect
// без изменения логики формирования чека.
type escposDevice struct {
	conn   io.WriteCloser
	width  int                 // ширина ленты в символах (42 для 80мм, 32 для 58мм)
	encode func(string) []byte // кодировка текста (по умолчанию UTF-8, опц. CP866)
}

// text кодирует текстовую часть чека выбранной кодировкой; управляющие
// ESC/POS-байты пишутся отдельно через Write и кодированию не подлежат.
func (d *escposDevice) text(s string) []byte {
	if d.encode == nil {
		return []byte(s)
	}
	return d.encode(s)
}

// ESC/POS управляющие последовательности.
var (
	escInit      = []byte{0x1B, 0x40}                   // ESC @       — инициализация
	escAlignLeft = []byte{0x1B, 0x61, 0x00}             // ESC a 0     — выравнивание влево
	escAlignCtr  = []byte{0x1B, 0x61, 0x01}             // ESC a 1     — по центру
	escBoldOn    = []byte{0x1B, 0x45, 0x01}             // ESC E 1     — жирный вкл
	escBoldOff   = []byte{0x1B, 0x45, 0x00}             // ESC E 0     — жирный выкл
	escCutFull   = []byte{0x1D, 0x56, 0x00}             // GS V 0      — полный рез бумаги
	escDrawer    = []byte{0x1B, 0x70, 0x00, 0x19, 0xFA} // ESC p 0 25 250 — импульс денежного ящика
)

func (d *escposDevice) Kind() string { return "принтер_чеков" }

func (d *escposDevice) Connect(params map[string]string) error {
	if w := firstNonEmpty(params["ширина"], params["width"]); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			d.width = n
		}
	}
	d.encode = deviceEncoder(firstNonEmpty(params["кодировка"], params["encoding"]))
	conn, err := openWriteTransport(params)
	if err != nil {
		return err
	}
	d.conn = conn
	return nil
}

func (d *escposDevice) Disconnect() error {
	if d.conn == nil {
		return nil
	}
	err := d.conn.Close()
	d.conn = nil
	return err
}

// PrintReceipt формирует ESC/POS-поток нефискального чека и отправляет его
// одним Write: шапка по центру жирным, позиции «наименование / сумма»,
// итог жирным, подвал по центру, затем рез бумаги.
func (d *escposDevice) PrintReceipt(r Receipt) error {
	var buf bytes.Buffer
	buf.Write(escInit)

	if len(r.Header) > 0 {
		buf.Write(escAlignCtr)
		buf.Write(escBoldOn)
		for _, line := range r.Header {
			buf.Write(d.text(line))
			buf.WriteByte('\n')
		}
		buf.Write(escBoldOff)
		buf.WriteByte('\n')
	}

	buf.Write(escAlignLeft)
	for _, it := range r.Items {
		buf.Write(d.text(it.Name))
		buf.WriteByte('\n')
		qtyPrice := fmt.Sprintf("  %s x %s", num(it.Qty), num(it.Price))
		buf.Write(d.text(d.row(qtyPrice, num(it.Sum))))
		buf.WriteByte('\n')
	}
	if len(r.Items) > 0 {
		buf.WriteString(strings.Repeat("-", d.width))
		buf.WriteByte('\n')
	}

	buf.Write(escBoldOn)
	buf.Write(d.text(d.row("ИТОГО:", num(r.Total))))
	buf.WriteByte('\n')
	buf.Write(escBoldOff)
	if r.Payment != "" {
		buf.Write(d.text(d.row("Оплата:", r.Payment)))
		buf.WriteByte('\n')
	}

	if len(r.Footer) > 0 {
		buf.WriteByte('\n')
		buf.Write(escAlignCtr)
		for _, line := range r.Footer {
			buf.Write(d.text(line))
			buf.WriteByte('\n')
		}
		buf.Write(escAlignLeft)
	}

	buf.WriteString("\n\n\n")
	buf.Write(escCutFull)
	return d.write(buf.Bytes())
}

func (d *escposDevice) OpenDrawer() error { return d.write(escDrawer) }
func (d *escposDevice) CutPaper() error   { return d.write(escCutFull) }

func (d *escposDevice) write(chunk []byte) error {
	if d.conn == nil {
		return fmt.Errorf("устройство не подключено")
	}
	_, err := d.conn.Write(chunk)
	return err
}

// row выравнивает строку по ширине ленты: left прижато влево, right — вправо.
// Длина считается в рунах, иначе кириллица ломает выравнивание.
func (d *escposDevice) row(left, right string) string {
	pad := d.width - utf8.RuneCountInString(left) - utf8.RuneCountInString(right)
	if pad < 1 {
		return left + " " + right
	}
	return left + strings.Repeat(" ", pad) + right
}

// num форматирует число без лишних нулей: 1500 → "1500", 2.5 → "2.5".
func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
