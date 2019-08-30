package util

import (
	"reflect"
	"testing"
)

func TestRemoveFromSlice(t *testing.T) {
	got := []string{"a", "b", "c"}
	got = RemoveFromSlice(got, 1).([]string)
	expected := []string{"a", "c"}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("\n\tGOT: %s \n\tEXPECTED: %s", got, expected)
	}
}
