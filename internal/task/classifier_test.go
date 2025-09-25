package task

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/0xef53/kvmrun/internal/task/classifiers"
	test_utils "github.com/0xef53/kvmrun/internal/task/internal/testing"
)

func TestClassifier_Register_EmptyClassifier(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	if _, err := rootCls.Register(nil); !errors.Is(err, ErrRegistrationFailed) {
		t.Fatal(format(ErrRegistrationFailed, err))
	}
}

func TestClassifier_Register_WithDefinedNames(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	clsNames := []string{"unique-labels", "single-labels", "special-labels"}

	if names, err := rootCls.Register(cls, clsNames...); err == nil {
		if !reflect.DeepEqual(clsNames, names) {
			t.Fatal(format(clsNames, names, "Registered names"))
		}
	} else {
		t.Fatal(format(nil, err))
	}
}

func TestClassifier_Register_WithDefaultName(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	if names, err := rootCls.Register(cls); err == nil {
		if len(names) != 1 {
			t.Fatal(format(1, len(names)))
		}
	} else {
		t.Fatal(format(nil, err))
	}
}

func TestClassifier_Deregister(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	for i := 1; i <= 5; i++ {
		if _, err := rootCls.Register(cls, "unique-labels"); err != nil {
			t.Fatal(format(nil, err, fmt.Sprintf("Before deregister (attempt %d)", i)))
		}

		if err := rootCls.Deregister("unique-labels"); err != nil {
			t.Fatal(format(nil, err, fmt.Sprintf("Deregister (attempt %d)", i)))
		}

		for k := 1; k <= 5; k++ {
			if err := rootCls.Deregister("unique-labels"); err != nil {
				t.Fatal(format(nil, err, fmt.Sprintf("Repeated deregister (attempt %d)", k)))
			}
		}
	}
}

func TestClassifier_Assign_WithEmptyDefinition(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	if _, err := rootCls.Register(cls, "unique-labels"); err != nil {
		t.Fatal(format(nil, err, "Before assign"))
	}

	if err := rootCls.Assign(context.Background(), nil, "id-aabbcc123"); !errors.Is(err, ErrAssignmentFailed) {
		t.Fatal(format(ErrAssignmentFailed, err))
	}
}

func TestClassifier_Assign_WithInvalidDefinition(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	if _, err := rootCls.Register(cls, "unique-labels"); err != nil {
		t.Fatal(format(nil, err, "Before assign"))
	}

	def := &TaskClassifierDefinition{
		Name: "",
	}

	if err := rootCls.Assign(context.Background(), def, "id-aabbcc123"); !errors.Is(err, ErrAssignmentFailed) {
		t.Fatal(format(ErrAssignmentFailed, err))
	}

	def = &TaskClassifierDefinition{
		Name: "unique-labels",
	}

	if err := rootCls.Assign(context.Background(), def, "id-aabbcc123"); !errors.Is(err, ErrAssignmentFailed) {
		t.Fatal(format(ErrAssignmentFailed, err))
	}
}

func TestClassifier_Assign_WithUnknownClassifier(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewUniqueLabelClassifier()

	if _, err := rootCls.Register(cls, "unique-labels"); err != nil {
		t.Fatal(format(nil, err, "Before assign"))
	}

	def := &TaskClassifierDefinition{
		Name: "non-existent-classifier-name",
		Opts: &classifiers.UniqueLabelOptions{Label: "task"},
	}

	if err := rootCls.Assign(context.Background(), def, "id-aabbcc123"); !errors.Is(err, ErrAssignmentFailed) {
		t.Fatal(format(ErrAssignmentFailed, err))
	}
}

func TestClassifier_Unassign(t *testing.T) {
	format := test_utils.FormatResultString

	rootCls := newRootClassifier()

	cls := classifiers.NewGroupLabelClassifier()

	if _, err := rootCls.Register(cls, "group-labels"); err != nil {
		t.Fatal(format(nil, err, "Before unassign"))
	}

	def := &TaskClassifierDefinition{
		Name: "group-labels",
		Opts: &classifiers.UniqueLabelOptions{Label: "group-1"},
	}

	for i := range 5001 {
		if err := rootCls.Assign(context.Background(), def, fmt.Sprintf("id-%d", i)); err != nil {
			t.Fatal(format(nil, err, "Before unassign"))
		}
	}

	for i := range 5000 {
		rootCls.Unassign(fmt.Sprintf("id-%d", i))
	}

	if n := len(rootCls.Get("group-1")); n > 1 {
		t.Fatal(format(1, n))
	}
}
