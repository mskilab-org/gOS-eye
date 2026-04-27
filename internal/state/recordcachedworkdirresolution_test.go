package state

import (
	"reflect"
	"testing"
)

func TestRecordCachedWorkdirResolution_IncrementsCounterForTrackedProvenance(t *testing.T) {
	cases := []struct {
		name             string
		provenance       WorkdirProvenance
		wantExplicit     int
		wantHashResolved int
		wantUnresolved   int
		wantAmbiguous    int
	}{
		{
			name:             "explicit trace workdir",
			provenance:       WorkdirProvenanceTraceWorkdir,
			wantExplicit:     11,
			wantHashResolved: 20,
			wantUnresolved:   30,
			wantAmbiguous:    40,
		},
		{
			name:             "hash resolved",
			provenance:       WorkdirProvenanceHashResolved,
			wantExplicit:     10,
			wantHashResolved: 21,
			wantUnresolved:   30,
			wantAmbiguous:    40,
		},
		{
			name:             "unresolved",
			provenance:       WorkdirProvenanceUnresolved,
			wantExplicit:     10,
			wantHashResolved: 20,
			wantUnresolved:   31,
			wantAmbiguous:    40,
		},
		{
			name:             "ambiguous hash",
			provenance:       WorkdirProvenanceAmbiguousHash,
			wantExplicit:     10,
			wantHashResolved: 20,
			wantUnresolved:   30,
			wantAmbiguous:    41,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			traceImport := TraceImportState{
				Warning:                         "legacy trace warning",
				CachedWorkdirsExplicit:          10,
				CachedWorkdirsHashResolved:      20,
				CachedWorkdirsUnresolved:        30,
				CachedWorkdirsAmbiguous:         40,
				CachedWorkdirResolutionWarnings: []string{"existing cached warning"},
			}

			recordCachedWorkdirResolution(&traceImport, CachedWorkdirResolution{Provenance: tc.provenance})

			if traceImport.CachedWorkdirsExplicit != tc.wantExplicit {
				t.Fatalf("CachedWorkdirsExplicit = %d, want %d", traceImport.CachedWorkdirsExplicit, tc.wantExplicit)
			}
			if traceImport.CachedWorkdirsHashResolved != tc.wantHashResolved {
				t.Fatalf("CachedWorkdirsHashResolved = %d, want %d", traceImport.CachedWorkdirsHashResolved, tc.wantHashResolved)
			}
			if traceImport.CachedWorkdirsUnresolved != tc.wantUnresolved {
				t.Fatalf("CachedWorkdirsUnresolved = %d, want %d", traceImport.CachedWorkdirsUnresolved, tc.wantUnresolved)
			}
			if traceImport.CachedWorkdirsAmbiguous != tc.wantAmbiguous {
				t.Fatalf("CachedWorkdirsAmbiguous = %d, want %d", traceImport.CachedWorkdirsAmbiguous, tc.wantAmbiguous)
			}
			if traceImport.Warning != "legacy trace warning" {
				t.Fatalf("Warning = %q, want legacy warning unchanged", traceImport.Warning)
			}
			wantWarnings := []string{"existing cached warning"}
			if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, wantWarnings) {
				t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, wantWarnings)
			}
		})
	}
}

func TestRecordCachedWorkdirResolution_AppendsNonEmptyWarningWithoutMutatingLegacyWarning(t *testing.T) {
	traceImport := TraceImportState{
		Warning:                         "trace path warning",
		CachedWorkdirResolutionWarnings: []string{"existing cached warning"},
	}

	recordCachedWorkdirResolution(&traceImport, CachedWorkdirResolution{
		Provenance: WorkdirProvenanceUnresolved,
		Warning:    "cached workdir unresolved: blank task hash",
	})

	if traceImport.CachedWorkdirsUnresolved != 1 {
		t.Fatalf("CachedWorkdirsUnresolved = %d, want 1", traceImport.CachedWorkdirsUnresolved)
	}
	if traceImport.Warning != "trace path warning" {
		t.Fatalf("Warning = %q, want legacy trace warning unchanged", traceImport.Warning)
	}
	wantWarnings := []string{"existing cached warning", "cached workdir unresolved: blank task hash"}
	if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, wantWarnings) {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, wantWarnings)
	}
}

func TestRecordCachedWorkdirResolution_UntrackedProvenanceDoesNotIncrementCountersButStillRecordsWarning(t *testing.T) {
	traceImport := TraceImportState{}

	recordCachedWorkdirResolution(&traceImport, CachedWorkdirResolution{
		Provenance: WorkdirProvenanceLiveWebhook,
		Warning:    "cached workdir skipped: live webhook provenance",
	})

	if traceImport.CachedWorkdirsExplicit != 0 ||
		traceImport.CachedWorkdirsHashResolved != 0 ||
		traceImport.CachedWorkdirsUnresolved != 0 ||
		traceImport.CachedWorkdirsAmbiguous != 0 {
		t.Fatalf("counters = explicit %d, hash-resolved %d, unresolved %d, ambiguous %d; want all zero",
			traceImport.CachedWorkdirsExplicit,
			traceImport.CachedWorkdirsHashResolved,
			traceImport.CachedWorkdirsUnresolved,
			traceImport.CachedWorkdirsAmbiguous)
	}
	wantWarnings := []string{"cached workdir skipped: live webhook provenance"}
	if !reflect.DeepEqual(traceImport.CachedWorkdirResolutionWarnings, wantWarnings) {
		t.Fatalf("CachedWorkdirResolutionWarnings = %#v, want %#v", traceImport.CachedWorkdirResolutionWarnings, wantWarnings)
	}
}

func TestRecordCachedWorkdirResolution_NilTraceImportIsNoOp(t *testing.T) {
	recordCachedWorkdirResolution(nil, CachedWorkdirResolution{
		Provenance: WorkdirProvenanceAmbiguousHash,
		Warning:    "cached workdir ambiguous",
	})
}
