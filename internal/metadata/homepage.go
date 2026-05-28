package metadata

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// HomePageWidget is a single widget reference inside the home page layout.
type HomePageWidget struct {
	Name string `yaml:"name"`
	Span int    `yaml:"span"` // for layout=grid, default 1; chart-row widgets often span 3
}

// HomePageRow groups widgets that should render side-by-side.
type HomePageRow struct {
	Widgets []string `yaml:"widgets"`
}

// HomePage describes the dashboard layout for /ui/.
type HomePage struct {
	Title   string            `yaml:"title"`
	Titles  map[string]string `yaml:"titles"`
	Layout  string            `yaml:"layout"` // grid | rows (default rows)
	Rows    []HomePageRow     `yaml:"rows"`
	Widgets []HomePageWidget  `yaml:"widgets"` // flat list, used when layout=grid
}

// DisplayTitle возвращает заголовок главной страницы с учётом языка.
// Если ни перевода, ни Title нет — возвращает русский «Главная».
func (h *HomePage) DisplayTitle(lang string) string {
	if h == nil {
		return "Главная"
	}
	if lang != "" {
		if v, ok := h.Titles[lang]; ok && v != "" {
			return v
		}
	}
	if h.Title != "" {
		return h.Title
	}
	return "Главная"
}

// applyDefaults fills in zero-value fields with sensible defaults.
func (h *HomePage) applyDefaults() {
	if h.Title == "" {
		h.Title = "Главная"
	}
	if h.Layout == "" {
		if len(h.Widgets) > 0 {
			h.Layout = "grid"
		} else {
			h.Layout = "rows"
		}
	}
}

// LoadHomePage reads config/home_page.yaml. Returns nil, nil when file does not exist —
// caller is expected to fall back to a default page in that case.
func LoadHomePage(path string) (*HomePage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var hp HomePage
	if err := yaml.Unmarshal(data, &hp); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	hp.applyDefaults()
	return &hp, nil
}

// WidgetNames returns every widget name referenced by the page in the order it
// would be rendered.
func (h *HomePage) WidgetNames() []string {
	if h == nil {
		return nil
	}
	var out []string
	for _, r := range h.Rows {
		out = append(out, r.Widgets...)
	}
	for _, w := range h.Widgets {
		out = append(out, w.Name)
	}
	return out
}
