package interpreter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/boffik/onebase/internal/dsl/interpreter"
	"github.com/boffik/onebase/internal/dsl/lexer"
	"github.com/boffik/onebase/internal/dsl/parser"
	"github.com/boffik/onebase/internal/metadata"
	"github.com/boffik/onebase/internal/runtime"
)

func evalJSON(t *testing.T, src string) any {
	t.Helper()
	l := lexer.New(src, "test.os")
	p := parser.New(l)
	prog, err := p.ParseProgram()
	require.NoError(t, err)
	require.NotEmpty(t, prog.Procedures)

	interp := interpreter.New()
	obj := runtime.NewObject("Test", metadata.KindDocument)
	var result any
	err = interp.RunWithResult(prog.Procedures[0], obj, &result)
	require.NoError(t, err)
	return result
}

func TestReadJSON_Object(t *testing.T) {
	src := `Процедура Тест()
		м = ПрочитатьJSON("{""a"":1,""b"":""two""}");
		Возврат м.Получить("a") = 1 И м.Получить("b") = "two";
	КонецПроцедуры`
	assert.Equal(t, true, evalJSON(t, src))
}

func TestReadJSON_Array(t *testing.T) {
	src := `Процедура Тест()
		arr = ПрочитатьJSON("[10,20,30]");
		Возврат arr.Количество() = 3 И arr[0] + arr[2] = 40;
	КонецПроцедуры`
	assert.Equal(t, true, evalJSON(t, src))
}

func TestReadJSON_Nested(t *testing.T) {
	src := `Процедура Тест()
		с = ПрочитатьJSON("{""товары"":[{""имя"":""A""}]}");
		товар = с.Получить("товары")[0];
		Возврат товар.Получить("имя");
	КонецПроцедуры`
	assert.Equal(t, "A", evalJSON(t, src))
}

func TestWriteJSON_Roundtrip(t *testing.T) {
	src := `Процедура Тест()
		исх = "{""a"":1,""b"":[2,3]}";
		м = ПрочитатьJSON(исх);
		Возврат ЗаписатьJSON(м);
	КонецПроцедуры`
	out, ok := evalJSON(t, src).(string)
	require.True(t, ok)

	var v1, v2 any
	require.NoError(t, json.Unmarshal([]byte(`{"a":1,"b":[2,3]}`), &v1))
	require.NoError(t, json.Unmarshal([]byte(out), &v2))
	assert.Equal(t, v1, v2)
}

func TestReadJSON_InvalidJSON_Error(t *testing.T) {
	src := `Процедура Тест()
		Попытка
			ПрочитатьJSON("{bad json}");
			Возврат "no error";
		Исключение
			Возврат "caught: " + ОписаниеОшибки();
		КонецПопытки;
	КонецПроцедуры`
	result, ok := evalJSON(t, src).(string)
	require.True(t, ok)
	assert.Contains(t, result, "caught:")
}

func TestWriteJSON_Struct(t *testing.T) {
	src := `Процедура Тест()
		с = Новый Структура("имя, возраст", "Иван", 30);
		Возврат ЗаписатьJSON(с);
	КонецПроцедуры`
	out, ok := evalJSON(t, src).(string)
	require.True(t, ok)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "Иван", got["имя"])
	assert.Equal(t, float64(30), got["возраст"])
}

func TestReadJSON_Aliases(t *testing.T) {
	src := `Процедура Тест()
		м = ReadJSON("{""x"":5}");
		Возврат WriteJSON(м);
	КонецПроцедуры`
	out, ok := evalJSON(t, src).(string)
	require.True(t, ok)
	assert.JSONEq(t, `{"x":5}`, out)
}
