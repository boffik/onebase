package interpreter

import (
	"io"
	"os"
	"path/filepath"
)

// init регистрирует глобальные файловые операции. Объект Файл и
// ЧтениеТекста/ЗаписьТекста уже есть (file_builtins.go); здесь — процедуры
// уровня файловой системы. Рассчитаны на однопользовательский desktop-режим,
// где DSL-обработки выполняет сам разработчик.
func init() {
	builtins["копироватьфайл"] = copyFileFn
	builtins["copyfile"] = copyFileFn
	builtins["переместитьфайл"] = moveFileFn
	builtins["movefile"] = moveFileFn
	builtins["удалитьфайлы"] = deleteFileFn
	builtins["deletefiles"] = deleteFileFn
	builtins["создатькаталог"] = makeDirFn
	builtins["createdirectory"] = makeDirFn
	builtins["найтифайлы"] = findFilesFn
	builtins["findfiles"] = findFilesFn
}

// КопироватьФайл(Откуда, Куда) — копирование содержимого файла.
func copyFileFn(args []any, _ string, _ int) (any, error) {
	src := strArg(args, 0)
	dst := strArg(args, 1)
	in, err := os.Open(src)
	if err != nil {
		RaiseUserError("КопироватьФайл: " + err.Error())
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		RaiseUserError("КопироватьФайл: " + err.Error())
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		RaiseUserError("КопироватьФайл: " + err.Error())
	}
	return nil, nil
}

// ПереместитьФайл(Откуда, Куда) — переименование/перемещение.
func moveFileFn(args []any, _ string, _ int) (any, error) {
	if err := os.Rename(strArg(args, 0), strArg(args, 1)); err != nil {
		RaiseUserError("ПереместитьФайл: " + err.Error())
	}
	return nil, nil
}

// УдалитьФайлы(Путь) — удаление файла или пустого каталога. Намеренно не
// рекурсивно (os.Remove), чтобы случайно не снести дерево каталогов.
func deleteFileFn(args []any, _ string, _ int) (any, error) {
	if err := os.Remove(strArg(args, 0)); err != nil {
		RaiseUserError("УдалитьФайлы: " + err.Error())
	}
	return nil, nil
}

// СоздатьКаталог(Путь) — создание каталога вместе с родительскими.
func makeDirFn(args []any, _ string, _ int) (any, error) {
	if err := os.MkdirAll(strArg(args, 0), 0o755); err != nil {
		RaiseUserError("СоздатьКаталог: " + err.Error())
	}
	return nil, nil
}

// НайтиФайлы(Путь, Маска) → Массив путей подходящих файлов.
func findFilesFn(args []any, _ string, _ int) (any, error) {
	pattern := filepath.Join(strArg(args, 0), strArg(args, 1))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		RaiseUserError("НайтиФайлы: " + err.Error())
	}
	arr := &Array{}
	for _, m := range matches {
		arr.items = append(arr.items, m)
	}
	return arr, nil
}
