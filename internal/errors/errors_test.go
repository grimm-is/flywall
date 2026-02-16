// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package errors

import (
	"errors"
	"testing"
)

func TestError(t *testing.T) {
	err := New(KindValidation, "invalid input")
	if err.Error() != "invalid input" {
		t.Errorf("expected 'invalid input', got '%s'", err.Error())
	}

	wrapped := Wrap(err, KindInternal, "failed to validate")
	if wrapped.Error() != "failed to validate: invalid input" {
		t.Errorf("expected 'failed to validate: invalid input', got '%s'", wrapped.Error())
	}
}

func TestGetKind(t *testing.T) {
	err := New(KindValidation, "invalid input")
	if GetKind(err) != KindValidation {
		t.Errorf("expected KindValidation, got %v", GetKind(err))
	}

	wrapped := Wrap(err, KindInternal, "failed")
	if GetKind(wrapped) != KindInternal {
		t.Errorf("expected KindInternal, got %v", GetKind(wrapped))
	}

	if GetKind(errors.New("std error")) != KindUnknown {
		t.Errorf("expected KindUnknown, got %v", GetKind(errors.New("std error")))
	}
}

func TestAttributes(t *testing.T) {
	err := New(KindValidation, "invalid input")
	err = Attr(err, "field", "port")
	err = Attr(err, "value", 80)

	attrs := GetAttributes(err)
	if attrs["field"] != "port" {
		t.Errorf("expected port, got %v", attrs["field"])
	}
	if attrs["value"] != 80 {
		t.Errorf("expected 80, got %v", attrs["value"])
	}

	wrapped := Wrap(err, KindInternal, "failed")
	wrapped = Attr(wrapped, "operation", "start")

	allAttrs := GetAttributes(wrapped)
	if allAttrs["field"] != "port" || allAttrs["operation"] != "start" {
		t.Errorf("missing attributes: %v", allAttrs)
	}
}
