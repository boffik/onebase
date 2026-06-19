// Package equipment реализует слой подключаемого торгового оборудования:
// единый интерфейс драйверов и реестр, не зависящие ни от DSL, ни от транспорта.
//
// Развязка намеренная: один и тот же драйвер вызывается как in-process
// (сервер = касса), так и из будущего device-agent по localhost. Сценарий
// «локально» — это частный случай клиент-серверного, меняется лишь транспорт,
// а код драйверов остаётся общим.
package equipment

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Device — базовый интерфейс любого подключаемого оборудования.
type Device interface {
	// Connect открывает соединение с устройством по параметрам драйвера
	// (например, "порт": "192.168.1.50:9100" или "COM3").
	Connect(params map[string]string) error
	// Disconnect закрывает соединение. Должен быть идемпотентен.
	Disconnect() error
	// Kind возвращает категорию устройства ("принтер_чеков", "весы", ...).
	Kind() string
}

// ReceiptPrinter — устройство печати чеков (нефискальное: ESC/POS и совместимые).
type ReceiptPrinter interface {
	Device
	PrintReceipt(r Receipt) error
	OpenDrawer() error
	CutPaper() error
}

// CustomerDisplay — дисплей покупателя (VFD): вывод строк и очистка экрана.
type CustomerDisplay interface {
	Device
	ShowLines(lines []string) error
	Clear() error
}

// Scale — электронные весы: запрос текущего веса (двунаправленный обмен).
type Scale interface {
	Device
	Weight() (float64, error)
}

// PaymentTerminal — платёжный терминал (эквайринг): оплата картой. Обмен
// асинхронный по природе (вставка карты, ПИН, связь с банком), поэтому драйвер
// держит увеличенный таймаут, а результат — составной.
type PaymentTerminal interface {
	Device
	Pay(amount float64) (PaymentResult, error)
}

// PaymentResult — итог операции оплаты картой.
type PaymentResult struct {
	Approved bool   // одобрено банком
	RRN      string // ссылочный номер операции
	Card     string // маскированный номер карты
	Message  string // сырой ответ терминала
}

// EventSource — устройство с асинхронным потоком событий (сканер ШК: коды
// приходят по мере сканирования). Это «события внутрь», которых нет в обычной
// request-response модели; на кассу они доставляются через SSE агента.
// Stream блокирует, вызывая fn на каждое событие, пока не закроют соединение
// или не отменят контекст.
type EventSource interface {
	Device
	Stream(ctx context.Context, fn func(event string)) error
}

// ReceiptItem — позиция чека.
type ReceiptItem struct {
	Name  string  // наименование
	Qty   float64 // количество
	Price float64 // цена за единицу
	Sum   float64 // сумма по позиции
}

// Receipt — данные нефискального чека для печати.
type Receipt struct {
	Header  []string      // строки шапки (магазин, адрес) — по центру
	Items   []ReceiptItem // позиции
	Total   float64       // итого к оплате
	Payment string        // вид оплаты: "Наличные" / "Карта"
	Footer  []string      // строки подвала ("Спасибо за покупку") — по центру
}

// Factory создаёт новый, ещё не подключённый экземпляр драйвера.
type Factory func() Device

var (
	regMu    sync.RWMutex
	registry = make(map[string]Factory)
)

// Register регистрирует драйвер под именем (например, "escpos_tcp").
// Вызывается из init() пакетов-драйверов; повтор имени — ошибка программиста.
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[name]; dup {
		panic("equipment: драйвер уже зарегистрирован: " + name)
	}
	registry[name] = f
}

// Open создаёт устройство по имени драйвера и подключает его. При ошибке
// Connect соединение закрывается, чтобы не текли ресурсы.
func Open(driver string, params map[string]string) (Device, error) {
	regMu.RLock()
	f, ok := registry[driver]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("неизвестный драйвер %q (доступны: %v)", driver, Drivers())
	}
	dev := f()
	if err := dev.Connect(params); err != nil {
		dev.Disconnect()
		return nil, fmt.Errorf("подключение %q: %w", driver, err)
	}
	return dev, nil
}

// Drivers возвращает отсортированный список зарегистрированных драйверов.
func Drivers() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
