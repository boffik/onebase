package metadata

import (
	"fmt"
	"regexp"
	"strings"
)

// identRe — допустимые имена объектов и реквизитов: буква или подчёркивание,
// затем буквы/цифры/подчёркивания. Буквы — любые юникодные (кириллица и
// латиница): PostgreSQL допускает их в неэкранированных идентификаторах.
//
// Имена попадают в SQL БЕЗ кавычек (см. TableName/ColumnName/RegisterTableName
// и сборку запросов через fmt.Sprintf в internal/storage, internal/query),
// поэтому пробелы, пунктуация, кавычки и точки с запятой недопустимы. Это не
// только корректность (имя «Дата платежа» сломало бы DDL), но и защита от
// инъекции в режиме config_source: database, где имена задаются через
// веб-конфигуратор, а не проходят ревью YAML в git.
var identRe = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

// systemColumns — служебные колонки, которые платформа добавляет к таблицам
// сущностей. Реквизит с совпадающим именем (после ToLower) затёр бы системную
// колонку и привёл бы к неоднозначному/битому DDL.
var systemColumns = map[string]bool{
	"id":               true,
	"posted":           true,
	"deletion_mark":    true,
	"parent_id":        true,
	"is_folder":        true,
	"period":           true,
	"recorder":         true,
	"_is_predefined":   true,
	"_predefined_name": true,
}

// ValidIdent сообщает, пригодно ли имя как идентификатор объекта/реквизита
// OneBase (без проверки на коллизию со служебными колонками).
func ValidIdent(name string) bool {
	return identRe.MatchString(name)
}

// checkIdent возвращает ошибку, если имя нельзя безопасно использовать как
// SQL-идентификатор. role — человекочитаемая роль для текста ошибки
// (например, «справочник» или «реквизит документа Поступление»).
func checkIdent(role, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s: пустое имя", role)
	}
	if !identRe.MatchString(name) {
		return fmt.Errorf("%s %q: имя должно начинаться с буквы или _ и содержать "+
			"только буквы, цифры и _ (без пробелов, кавычек, точек и знаков пунктуации)", role, name)
	}
	return nil
}

// checkField проверяет имя реквизита как идентификатор и на коллизию со
// служебными колонками платформы.
func checkField(role string, f Field) error {
	if err := checkIdent(role, f.Name); err != nil {
		return err
	}
	if systemColumns[strings.ToLower(f.Name)] {
		return fmt.Errorf("%s %q: имя совпадает со служебной колонкой платформы — выберите другое", role, f.Name)
	}
	return nil
}

func checkFields(role string, fields []Field) error {
	for _, f := range fields {
		if err := checkField(role, f); err != nil {
			return err
		}
	}
	return nil
}

// ValidateIdentifiers проверяет, что имена всех объектов конфигурации и их
// реквизитов пригодны для использования как неэкранированные SQL-идентификаторы.
// Вызывается при загрузке проекта (см. internal/project) сразу после Validate.
// Любой аргумент может быть nil.
func ValidateIdentifiers(
	entities []*Entity,
	registers []*Register,
	inforegs []*InfoRegister,
	accountRegs []*AccountRegister,
	enums []*Enum,
	constants []*Constant,
) error {
	for _, e := range entities {
		role := "справочник"
		if e.Kind == KindDocument {
			role = "документ"
		}
		if err := checkIdent(role, e.Name); err != nil {
			return err
		}
		if err := checkFields(fmt.Sprintf("реквизит %s %q", role, e.Name), e.Fields); err != nil {
			return err
		}
		for _, tp := range e.TableParts {
			if err := checkIdent(fmt.Sprintf("табличная часть %s %q", role, e.Name), tp.Name); err != nil {
				return err
			}
			if err := checkFields(fmt.Sprintf("реквизит ТЧ %q.%q", e.Name, tp.Name), tp.Fields); err != nil {
				return err
			}
		}
	}

	for _, r := range registers {
		if err := checkIdent("регистр накопления", r.Name); err != nil {
			return err
		}
		role := fmt.Sprintf("поле регистра %q", r.Name)
		if err := checkFields(role, r.Dimensions); err != nil {
			return err
		}
		if err := checkFields(role, r.Resources); err != nil {
			return err
		}
		if err := checkFields(role, r.Attributes); err != nil {
			return err
		}
	}

	for _, ir := range inforegs {
		if err := checkIdent("регистр сведений", ir.Name); err != nil {
			return err
		}
		role := fmt.Sprintf("поле регистра сведений %q", ir.Name)
		if err := checkFields(role, ir.Dimensions); err != nil {
			return err
		}
		if err := checkFields(role, ir.Resources); err != nil {
			return err
		}
	}

	for _, ar := range accountRegs {
		if err := checkIdent("регистр бухгалтерии", ar.Name); err != nil {
			return err
		}
		if err := checkFields(fmt.Sprintf("ресурс регистра бухгалтерии %q", ar.Name), ar.Resources); err != nil {
			return err
		}
	}

	for _, en := range enums {
		if err := checkIdent("перечисление", en.Name); err != nil {
			return err
		}
	}

	for _, c := range constants {
		if err := checkIdent("константа", c.Name); err != nil {
			return err
		}
	}

	return nil
}
