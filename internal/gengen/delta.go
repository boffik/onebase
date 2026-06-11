package gengen

import "fmt"

// ComputeDelta computes the difference between the requested manifest and the
// existing project manifest. It returns a DeltaManifest describing what needs
// to be created, extended, or skipped.
func ComputeDelta(requested *ResolvedManifest, existing *ExistingManifest) *DeltaManifest {
	delta := &DeltaManifest{
		NewFields:     make(map[string][]FieldSpec),
		NewTableParts: make(map[string][]TablePartSpec),
		NewDSLFiles:   make(map[string]string),
	}

	// 1. New catalogs + field diffs for existing
	for _, cat := range requested.Catalogs {
		if _, exists := existing.Catalogs[cat.Name]; !exists {
			delta.NewCatalogs = append(delta.NewCatalogs, cat)
		} else {
			// Existing catalog: check for new fields and table parts
			checkEntityDelta(cat, existing.Catalogs[cat.Name].Fields, existing.Catalogs[cat.Name].TableParts, delta)
		}
	}

	// 2. New documents + field diffs for existing
	for _, doc := range requested.Documents {
		if _, exists := existing.Documents[doc.Name]; !exists {
			delta.NewDocuments = append(delta.NewDocuments, doc)
		} else {
			checkEntityDelta(doc, existing.Documents[doc.Name].Fields, existing.Documents[doc.Name].TableParts, delta)
		}
	}

	// 3. New registers
	for _, reg := range requested.Registers {
		if _, exists := existing.Registers[reg.Name]; !exists {
			delta.NewRegisters = append(delta.NewRegisters, reg)
		} else {
			// Check dimensions and resources
			existingReg := existing.Registers[reg.Name]
			existingFields := make(map[string]bool)
			for _, f := range existingReg.Dimensions {
				existingFields[f.Name] = true
			}
			for _, f := range existingReg.Resources {
				existingFields[f.Name] = true
			}
			for _, f := range reg.Fields {
				if !existingFields[f.Name] {
					delta.NewFields[reg.Name] = append(delta.NewFields[reg.Name], f)
				}
			}
		}
	}

	// 4. New enums
	for _, enum := range requested.Enums {
		if _, exists := existing.Enums[enum.Name]; !exists {
			delta.NewEnums = append(delta.NewEnums, enum)
		}
		// Note: enum value changes are not tracked in MVP
	}

	// 5. DSL files: only new ones
	for path, content := range requested.DSLFiles {
		if _, exists := existing.DSLFiles[path]; !exists {
			delta.NewDSLFiles[path] = content
		}
	}

	// 6. Detect conflicts (requested entities with same name but different kind)
	detectConflicts(requested, existing, delta)

	return delta
}

// checkEntityDelta compares an entity spec with existing fields/table parts
// and populates delta.NewFields and delta.NewTableParts.
func checkEntityDelta(spec EntitySpec, existingFields []FieldInfo, existingTPs []TablePartInfo, delta *DeltaManifest) {
	// Build set of existing field names
	existingFieldNames := make(map[string]bool)
	for _, f := range existingFields {
		existingFieldNames[f.Name] = true
	}

	// Find new fields
	for _, f := range spec.Fields {
		if !existingFieldNames[f.Name] {
			delta.NewFields[spec.Name] = append(delta.NewFields[spec.Name], f)
		}
	}

	// Build set of existing table part names
	existingTPNames := make(map[string]bool)
	for _, tp := range existingTPs {
		existingTPNames[tp.Name] = true
	}

	// Find new table parts
	for _, tp := range spec.TableParts {
		if !existingTPNames[tp.Name] {
			delta.NewTableParts[spec.Name] = append(delta.NewTableParts[spec.Name], tp)
		} else {
			// Existing TP: check for new fields inside it
			for _, existingTP := range existingTPs {
				if existingTP.Name == tp.Name {
					existingTPFieldNames := make(map[string]bool)
					for _, f := range existingTP.Fields {
						existingTPFieldNames[f.Name] = true
					}
					for _, f := range tp.Fields {
						if !existingTPFieldNames[f.Name] {
							// Mark as new field in the TP
							tpWithNewFields := TablePartSpec{
								Name:   tp.Name,
								Fields: []FieldSpec{f},
							}
							delta.NewTableParts[spec.Name] = append(delta.NewTableParts[spec.Name], tpWithNewFields)
						}
					}
					break
				}
			}
		}
	}
}

// detectConflicts finds cases where a requested entity name collides with an
// existing entity of a different kind.
func detectConflicts(requested *ResolvedManifest, existing *ExistingManifest, delta *DeltaManifest) {
	// Check catalogs
	for _, cat := range requested.Catalogs {
		if _, ok := existing.Documents[cat.Name]; ok {
			delta.Conflicts = append(delta.Conflicts, Conflict{
				Kind:    "catalog",
				Name:    cat.Name,
				Message: fmt.Sprintf("«%s» уже существует как документ, нельзя создать справочник с тем же именем", cat.Name),
			})
		}
		if _, ok := existing.Registers[cat.Name]; ok {
			delta.Conflicts = append(delta.Conflicts, Conflict{
				Kind:    "catalog",
				Name:    cat.Name,
				Message: fmt.Sprintf("«%s» уже существует как регистр, нельзя создать справочник с тем же именем", cat.Name),
			})
		}
	}

	// Check documents
	for _, doc := range requested.Documents {
		if _, ok := existing.Catalogs[doc.Name]; ok {
			delta.Conflicts = append(delta.Conflicts, Conflict{
				Kind:    "document",
				Name:    doc.Name,
				Message: fmt.Sprintf("«%s» уже существует как справочник, нельзя создать документ с тем же именем", doc.Name),
			})
		}
		if _, ok := existing.Registers[doc.Name]; ok {
			delta.Conflicts = append(delta.Conflicts, Conflict{
				Kind:    "document",
				Name:    doc.Name,
				Message: fmt.Sprintf("«%s» уже существует как регистр, нельзя создать документ с тем же именем", doc.Name),
			})
		}
	}

	// Check enums
	for _, enum := range requested.Enums {
		if _, ok := existing.Catalogs[enum.Name]; ok {
			delta.Conflicts = append(delta.Conflicts, Conflict{
				Kind:    "enum",
				Name:    enum.Name,
				Message: fmt.Sprintf("«%s» уже существует как справочник, нельзя создать перечисление с тем же именем", enum.Name),
			})
		}
	}
}

// HasChanges returns true if the delta contains any new entities, fields, or files.
func (d *DeltaManifest) HasChanges() bool {
	return len(d.NewCatalogs) > 0 ||
		len(d.NewDocuments) > 0 ||
		len(d.NewRegisters) > 0 ||
		len(d.NewEnums) > 0 ||
		len(d.NewFields) > 0 ||
		len(d.NewTableParts) > 0 ||
		len(d.NewDSLFiles) > 0
}

// Summary returns a human-readable summary of the delta.
func (d *DeltaManifest) Summary() string {
	parts := []string{}
	if len(d.NewCatalogs) > 0 {
		names := make([]string, len(d.NewCatalogs))
		for i, c := range d.NewCatalogs {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("+%d справочник(ов): %v", len(d.NewCatalogs), names))
	}
	if len(d.NewDocuments) > 0 {
		names := make([]string, len(d.NewDocuments))
		for i, c := range d.NewDocuments {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("+%d документ(ов): %v", len(d.NewDocuments), names))
	}
	if len(d.NewRegisters) > 0 {
		names := make([]string, len(d.NewRegisters))
		for i, c := range d.NewRegisters {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("+%d регистр(ов): %v", len(d.NewRegisters), names))
	}
	if len(d.NewEnums) > 0 {
		names := make([]string, len(d.NewEnums))
		for i, c := range d.NewEnums {
			names[i] = c.Name
		}
		parts = append(parts, fmt.Sprintf("+%d перечисление(ий): %v", len(d.NewEnums), names))
	}
	if len(d.NewFields) > 0 {
		parts = append(parts, fmt.Sprintf("+%d сущностей с новыми полями", len(d.NewFields)))
	}
	if len(d.NewTableParts) > 0 {
		parts = append(parts, fmt.Sprintf("+%d сущностей с новыми ТЧ", len(d.NewTableParts)))
	}
	if len(d.NewDSLFiles) > 0 {
		names := make([]string, 0, len(d.NewDSLFiles))
		for k := range d.NewDSLFiles {
			names = append(names, k)
		}
		parts = append(parts, fmt.Sprintf("+%d DSL-файл(ов): %v", len(d.NewDSLFiles), names))
	}
	if len(d.Conflicts) > 0 {
		parts = append(parts, fmt.Sprintf("⚠ %d конфликт(ов)", len(d.Conflicts)))
	}
	if len(parts) == 0 {
		return "Нет изменений — всё уже существует"
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n"
		}
		result += p
	}
	return result
}
