package launcher

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ivantit66/onebase/internal/configdb"
)

// Контекстное меню дерева отправляет обработки, справочники и документы в
// один handler. В database-режиме он должен удалить не только YAML объекта,
// но и принадлежащие ему модули, макет обработки и формы.
func TestConfiguratorDeleteObject_DBModeRemovesOwnedFiles(t *testing.T) {
	store := newTestStore(t)
	base := &Base{
		ID:           "delete-object-db",
		Name:         "Тест удаления",
		ConfigSource: "database",
		DBType:       "sqlite",
		DBPath:       filepath.Join(t.TempDir(), "config.db"),
	}
	if err := store.Add(base); err != nil {
		t.Fatalf("store.Add: %v", err)
	}

	ctx := context.Background()
	db, err := OpenDB(ctx, base)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	repo := configdb.New(db)
	if err := repo.EnsureSchema(ctx); err != nil {
		db.Close()
		t.Fatalf("EnsureSchema: %v", err)
	}
	processorLayout := `name: Обработка1
areas:
  Заголовок:
    rows:
      - cells:
          - text: "Тест"
`
	seed := []configdb.ConfigFile{
		{Path: "config/app.yaml", Content: []byte("name: Тест\n")},
		{Path: "catalogs/клиент.yaml", Content: []byte("name: Клиент\nfields: []\n")},
		{Path: "src/клиент.os", Content: []byte("")},
		{Path: "src/клиент.manager.os", Content: []byte("")},
		{Path: "src/клиент.form.os", Content: []byte("")},
		{Path: "forms/клиент/_resources/icon.json", Content: []byte("{}")},
		{Path: "documents/заказ.yaml", Content: []byte("name: Заказ\nfields: []\n")},
		{Path: "src/заказ.os", Content: []byte("")},
		{Path: "src/заказ.posting.os", Content: []byte("")},
		{Path: "src/заказ.manager.os", Content: []byte("")},
		{Path: "src/заказ_list.form.os", Content: []byte("")},
		{Path: "forms/заказ/_resources/icon.json", Content: []byte("{}")},
		{Path: "processors/обработка1.yaml", Content: []byte("name: Обработка1\ntitle: Обработка 1\n")},
		{Path: "src/обработка1.proc.os", Content: []byte("Процедура Выполнить()\nКонецПроцедуры\n")},
		{Path: "src/обработка1.proc.layout.yaml", Content: []byte(processorLayout)},
		{Path: "forms/обработка1/_resources/icon.json", Content: []byte("{}")},
		{Path: "processors/оставить.yaml", Content: []byte("name: Оставить\n")},
	}
	if err := repo.SaveFiles(ctx, seed, configdb.VersionOptions{Message: "seed delete objects"}); err != nil {
		db.Close()
		t.Fatalf("seed configdb: %v", err)
	}
	db.Close()

	h := &handler{store: store, runner: NewRunner()}
	cases := []struct {
		kind  string
		name  string
		paths []string
	}{
		{
			kind: "processor",
			name: "Обработка1",
			paths: []string{
				"processors/обработка1.yaml",
				"src/обработка1.proc.os",
				"src/обработка1.proc.layout.yaml",
				"forms/обработка1/_resources/icon.json",
			},
		},
		{
			kind: "catalog",
			name: "Клиент",
			paths: []string{
				"catalogs/клиент.yaml",
				"src/клиент.os",
				"src/клиент.manager.os",
				"src/клиент.form.os",
				"forms/клиент/_resources/icon.json",
			},
		},
		{
			kind: "document",
			name: "Заказ",
			paths: []string{
				"documents/заказ.yaml",
				"src/заказ.os",
				"src/заказ.posting.os",
				"src/заказ.manager.os",
				"src/заказ_list.form.os",
				"forms/заказ/_resources/icon.json",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{
				"entity": {tc.name},
				"kind":   {tc.kind},
			}
			rec := postCfgRv(
				t,
				base.ID,
				"/bases/"+base.ID+"/configurator/entity-delete",
				form,
				h.configuratorDeleteEntity,
			)
			var response struct {
				OK    bool   `json:"ok"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("ответ не JSON: %v (%s)", err, rec.Body.String())
			}
			if !response.OK {
				t.Fatalf("удаление %s не выполнено: %s", tc.name, response.Error)
			}

			checkDB, err := OpenDB(ctx, base)
			if err != nil {
				t.Fatalf("OpenDB для проверки: %v", err)
			}
			checkRepo := configdb.New(checkDB)
			for _, path := range tc.paths {
				if _, ok, err := checkRepo.ReadFile(ctx, path); err != nil || ok {
					checkDB.Close()
					t.Fatalf("%s остался после удаления: ok=%v err=%v", path, ok, err)
				}
			}
			checkDB.Close()
		})
	}

	checkDB, err := OpenDB(ctx, base)
	if err != nil {
		t.Fatalf("OpenDB для проверки несвязанного объекта: %v", err)
	}
	defer checkDB.Close()
	if _, ok, err := configdb.New(checkDB).ReadFile(ctx, "processors/оставить.yaml"); err != nil || !ok {
		t.Fatalf("несвязанный объект удалён: ok=%v err=%v", ok, err)
	}
}

func TestConfiguratorDeleteObject_FileModeRemovesProcessorFiles(t *testing.T) {
	h, cfgDir := newFileBaseHandler(t)
	h.runner = NewRunner()

	writeCfgFileRv(t, cfgDir, "config", "app.yaml", "name: Тест\n")
	writeCfgFileRv(t, cfgDir, "processors", "обработка1.yaml", "name: Обработка1\n")
	writeCfgFileRv(t, cfgDir, "processors", "оставить.yaml", "name: Оставить\n")
	writeCfgFileRv(t, cfgDir, "src", "обработка1.proc.os", "Процедура Выполнить()\nКонецПроцедуры\n")
	writeCfgFileRv(t, cfgDir, "src", "обработка1.proc.layout.yaml", `name: Обработка1
areas:
  Заголовок:
    rows:
      - cells:
          - text: "Тест"
`)
	resource := writeCfgFileRv(t, cfgDir, "forms/обработка1/_resources", "icon.json", "{}")

	rec := postCfgRv(
		t,
		"test",
		"/bases/test/configurator/entity-delete",
		url.Values{"entity": {"Обработка1"}, "kind": {"processor"}},
		h.configuratorDeleteEntity,
	)
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("ответ не JSON: %v (%s)", err, rec.Body.String())
	}
	if !response.OK {
		t.Fatalf("удаление обработки не выполнено: %s", response.Error)
	}

	for _, path := range []string{
		filepath.Join(cfgDir, "processors", "обработка1.yaml"),
		filepath.Join(cfgDir, "src", "обработка1.proc.os"),
		filepath.Join(cfgDir, "src", "обработка1.proc.layout.yaml"),
		resource,
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s остался после удаления (err=%v)", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(cfgDir, "processors", "оставить.yaml")); err != nil {
		t.Fatalf("несвязанная обработка удалена: %v", err)
	}
}

func TestConfiguratorObjectDeletePathsUsesCurrentMetadataDirs(t *testing.T) {
	tests := []struct {
		kind string
		path string
	}{
		{kind: "register", path: "registers/остатки.yaml"},
		{kind: "inforeg", path: "inforegs/цены.yaml"},
		{kind: "accountreg", path: "accountregs/хозрасчётный.yaml"},
		{kind: "enum", path: "enums/статус.yaml"},
		{kind: "report", path: "reports/продажи.yaml"},
		{kind: "processor", path: "processors/загрузка.yaml"},
		{kind: "printform", path: "printforms/накладная.yaml"},
		{kind: "subsystem", path: "subsystems/торговля.yaml"},
	}
	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			files := []configdb.ConfigFile{{
				Path:    tc.path,
				Content: []byte("name: Объект\n"),
			}}
			paths, kind, err := configuratorObjectDeletePaths(files, tc.kind, "Объект")
			if err != nil {
				t.Fatalf("configuratorObjectDeletePaths: %v", err)
			}
			if kind != tc.kind || !slices.Equal(paths, []string{tc.path}) {
				t.Fatalf("kind=%q paths=%v, ожидались %q и [%s]", kind, paths, tc.kind, tc.path)
			}
		})
	}

	modulePath := "src/общийМодуль.module.os"
	paths, kind, err := configuratorObjectDeletePaths(
		[]configdb.ConfigFile{{Path: modulePath, Content: []byte("")}},
		"module",
		"ОбщийМодуль",
	)
	if err != nil || kind != "module" || !slices.Equal(paths, []string{modulePath}) {
		t.Fatalf("общий модуль: kind=%q paths=%v err=%v", kind, paths, err)
	}

	// Одинаковые имена допустимы в разных пространствах метаданных: тип узла
	// не должен позволить удалению справочника задеть одноимённый документ.
	sameName := []configdb.ConfigFile{
		{Path: "catalogs/заказ.yaml", Content: []byte("name: Заказ\n")},
		{Path: "documents/заказ.yaml", Content: []byte("name: Заказ\n")},
	}
	paths, kind, err = configuratorObjectDeletePaths(sameName, "catalog", "Заказ")
	if err != nil || kind != "catalog" || !slices.Equal(paths, []string{"catalogs/заказ.yaml"}) {
		t.Fatalf("одноимённые объекты: kind=%q paths=%v err=%v", kind, paths, err)
	}
}

func TestConfiguratorDeleteContextMenuSendsObjectKind(t *testing.T) {
	js, err := os.ReadFile("static/configurator.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(js)
	for _, fragment := range []string{
		`data-delete-kind="catalog"`,
		`data-delete-kind="document"`,
	} {
		if !strings.Contains(cfgTabTree, fragment) {
			t.Errorf("дерево не различает типы узлов: нет %q", fragment)
		}
	}
	for _, fragment := range []string{
		"item.dataset.deleteKind",
		"e:'entity'",
		"proc:'processor'",
		"ir:'inforeg'",
		"ar:'accountreg'",
		"kindInp.name='kind'",
	} {
		if !strings.Contains(source, fragment) {
			t.Errorf("контекстное удаление не передаёт тип узла: нет %q", fragment)
		}
	}
}
