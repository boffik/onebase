package ui

import (
	"testing"

	"github.com/ivantit66/onebase/internal/auth"
	"github.com/ivantit66/onebase/internal/metadata"
)

func TestRLSDiagnosticsShowsAutoFillAndReferencePolicy(t *testing.T) {
	owner := &metadata.Entity{
		Name: "Клиент", Kind: metadata.KindCatalog,
		Fields: []metadata.Field{
			{Name: "Наименование", Type: metadata.FieldTypeString},
			{Name: "Owner", Type: metadata.FieldTypeString},
		},
	}
	order := &metadata.Entity{
		Name: "Заказ", Kind: metadata.KindDocument,
		Fields: []metadata.Field{
			{Name: "Номер", Type: metadata.FieldTypeString},
			{Name: "Клиент", Type: metadata.FieldTypeString, RefEntity: owner.Name},
		},
	}
	s, _ := newSubmitTestServer(t, []*metadata.Entity{owner, order})
	role := &auth.Role{Name: "Менеджер", Permissions: auth.Permission{
		Catalogs:  map[string][]string{owner.Name: {"write"}},
		Documents: map[string][]string{order.Name: {"read"}},
		RowAccess: auth.RowAccess{
			Catalogs: map[string]auth.RowPolicies{
				owner.Name: {"write": {Field: "Owner", Op: "eq", Value: auth.RowValue{User: "login"}}},
			},
			Documents: map[string]auth.RowPolicies{
				order.Name: {"read": {Field: "Клиент.Owner", Op: "eq", Value: auth.RowValue{User: "login"}}},
			},
		},
	}}

	rows := s.buildRLSDiagnosticRows([]*auth.Role{role})
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want 2 diagnostics", rows)
	}
	var sawAutoFill, sawReference bool
	for _, row := range rows {
		if row.Object == owner.Name && row.Op == "write" {
			sawAutoFill = row.Status == "active" && row.AutoFill == "Owner"
		}
		if row.Object == order.Name && row.Field == "Клиент.Owner" {
			sawReference = row.Status == "active"
		}
	}
	if !sawAutoFill {
		t.Fatalf("diagnostics must report Owner auto-fill: %#v", rows)
	}
	if !sawReference {
		t.Fatalf("diagnostics must report active reference policy: %#v", rows)
	}
}
