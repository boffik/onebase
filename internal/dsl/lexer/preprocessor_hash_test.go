package lexer_test

import (
	"testing"

	"github.com/ivantit66/onebase/internal/dsl/lexer"
	"github.com/ivantit66/onebase/internal/dsl/token"
)

// #19: директива препроцессора 1С пропускается только в начале строки (в т.ч.
// с отступом). '#' в середине строки даёт ILLEGAL, а не молча съедает остаток
// строки (раньше «а = 5 #хвост;» терял «;», а обе ветви #Если исполнялись).
func TestPreprocessorHash_OnlyAtLineStart(t *testing.T) {
	for _, src := range []string{
		"#Если Сервер Тогда\nПроцедура П()\nКонецПроцедуры\n#КонецЕсли\n",
		"   #Область X\nПроцедура П()\nКонецПроцедуры\n",
	} {
		l := lexer.New(src, "t.os")
		for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
			if tok.Type == token.ILLEGAL {
				t.Fatalf("директива в начале строки дала ILLEGAL (%q): %+v", src, tok)
			}
		}
	}

	l := lexer.New("а = 5 # хвост\nб = 7;", "t.os")
	sawIllegal := false
	for tok := l.NextToken(); tok.Type != token.EOF; tok = l.NextToken() {
		if tok.Type == token.ILLEGAL {
			sawIllegal = true
		}
	}
	if !sawIllegal {
		t.Error("'#' в середине строки должен давать ILLEGAL, а не молча съедаться")
	}
}
