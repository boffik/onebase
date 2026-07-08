package ui

import (
	"context"

	"github.com/google/uuid"
	"github.com/ivantit66/onebase/internal/dsl/interpreter"
	"github.com/ivantit66/onebase/internal/metadata"
	"github.com/ivantit66/onebase/internal/storage"
)

type dslRowAccessChecker struct {
	s *Server
}

type trustedDSLContextKey struct{}

func trustedDSLContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, trustedDSLContextKey{}, true)
}

func isTrustedDSLContext(ctx context.Context) bool {
	v, _ := ctx.Value(trustedDSLContextKey{}).(bool)
	return v
}

func (c dslRowAccessChecker) CheckRowAccess(ctx context.Context, entity *metadata.Entity, op string, id uuid.UUID, fields map[string]any) error {
	if c.s == nil {
		return nil
	}
	return c.s.checkDSLRowAccess(ctx, entity, op, id, fields)
}

func (c dslRowAccessChecker) IsRowAccessRestricted(ctx context.Context, entity *metadata.Entity, op string) bool {
	return c.s != nil && c.s.rowAccessRestricted(ctx, entity, op)
}

func (s *Server) dslRowAccessChecker() interpreter.RowAccessChecker {
	return dslRowAccessChecker{s: s}
}

func (s *Server) checkDSLRowAccess(ctx context.Context, entity *metadata.Entity, op string, id uuid.UUID, fields map[string]any) error {
	if isTrustedDSLContext(ctx) {
		return nil
	}
	dec, err := s.rowDecision(ctx, entity, op)
	if err != nil {
		return err
	}
	if !dec.Allowed {
		return interpreter.ErrRowAccessDenied
	}
	if dec.Unrestricted {
		return nil
	}
	if id == uuid.Nil {
		if storage.MatchPredicate(fields, dec.Predicate) {
			return nil
		}
		return interpreter.ErrRowAccessDenied
	}
	row, err := s.store.GetByID(ctx, entity.Name, id, entity)
	if err != nil {
		return err
	}
	if !storage.MatchPredicate(row, dec.Predicate) {
		return interpreter.ErrRowAccessDenied
	}
	if fields != nil && !storage.MatchPredicate(storage.MergeRowFields(row, fields), dec.Predicate) {
		return interpreter.ErrRowAccessDenied
	}
	return nil
}
