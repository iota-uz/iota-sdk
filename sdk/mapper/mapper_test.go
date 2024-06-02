package mapper

import "testing"

func TestStrictMapping(t *testing.T) {
	type Source struct {
		Field1 string
		Field2 int
	}
	type Target struct {
		Field1 string
		Field2 int
	}

	source := &Source{
		Field1: "test",
		Field2: 1,
	}
	target := &Target{}

	err := StrictMapping(source, target)
	if err != nil {
		t.Error(err)
	}
	if target.Field1 != "test" {
		t.Errorf("expected %s, got %s", "test", target.Field1)
	}
	if target.Field2 != 1 {
		t.Errorf("expected %d, got %d", 1, target.Field2)
	}
}

func TestStrictMappingWithPointers(t *testing.T) {
	type Source struct {
		Field1 *string
		Field2 *int
	}

	type Target struct {
		Field1 *string
		Field2 *int
	}

	source := &Source{
		Field1: Pointer("test"),
		Field2: Pointer(1),
	}

	target := &Target{}

	err := StrictMapping(source, target)
	if err != nil {
		t.Error(err)
	}

	if *target.Field1 != "test" {
		t.Errorf("expected %s, got %s", "test", *target.Field1)
	}

	if *target.Field2 != 1 {
		t.Errorf("expected %d, got %d", 1, *target.Field2)
	}
}

func TestStrictMappingWithPointers2(t *testing.T) {
	type Source struct {
		Field1 *string
		Field2 *int
	}

	type Target struct {
		Field1 string
		Field2 int
	}

	source := &Source{
		Field1: Pointer("test"),
		Field2: Pointer(1),
	}
	target := &Target{}

	err := StrictMapping(source, target)
	if err != nil {
		t.Error(err)
	}
	if target.Field1 != "test" {
		t.Errorf("expected %s, got %s", "test", target.Field1)
	}
	if target.Field2 != 1 {
		t.Errorf("expected %d, got %d", 1, target.Field2)
	}
}

func TestStrictMappingWithDifferentTypes(t *testing.T) {
	type Source struct {
		Field1 string
		Field2 int
	}
	type Target struct {
		Field1 int
		Field2 string
	}

	source := &Source{
		Field1: "test",
		Field2: 1,
	}
	target := &Target{}

	err := StrictMapping(source, target)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
