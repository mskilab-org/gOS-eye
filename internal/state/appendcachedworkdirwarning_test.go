package state

import (
	"reflect"
	"testing"
)

func TestAppendCachedWorkdirWarning_IgnoresEmptyWarning(t *testing.T) {
	traceImport := TraceImportState{
		Warning:                         "trace path warning",
		CachedWorkdirResolutionWarnings: []string{"existing cached warning"},
	}

	traceImport.appendCachedWorkdirWarning("")

	if traceImport.Warning != "trace path warning" {
		t.Fatalf("Warning = %q, want legacy trace warning unchanged", traceImport.Warning)
	}
	want := []string{"existing cached warning"}
	if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, want) {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, want)
	}
}

func TestAppendCachedWorkdirWarning_AppendsNonEmptyWarning(t *testing.T) {
	traceImport := TraceImportState{Warning: "trace path warning"}

	traceImport.appendCachedWorkdirWarning("cached workdir unresolved")

	if traceImport.Warning != "trace path warning" {
		t.Fatalf("Warning = %q, want legacy trace warning unchanged", traceImport.Warning)
	}
	want := []string{"cached workdir unresolved"}
	if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, want) {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, want)
	}
}

func TestAppendCachedWorkdirWarning_PreservesExistingOrder(t *testing.T) {
	traceImport := TraceImportState{
		Warning:                         "trace path was not explicit",
		CachedWorkdirResolutionWarnings: []string{"first cached warning"},
	}

	traceImport.appendCachedWorkdirWarning("second cached warning")
	traceImport.appendWarning("parse trace: bad task_id")

	if traceImport.Warning != "trace path was not explicit; parse trace: bad task_id" {
		t.Fatalf("Warning = %q, want legacy trace warnings appended with semicolon", traceImport.Warning)
	}
	want := []string{"first cached warning", "second cached warning"}
	if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, want) {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, want)
	}
}
