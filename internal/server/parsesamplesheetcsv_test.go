package server

import (
	"reflect"
	"testing"
)

func TestParseSamplesheetCSV_EmptyString(t *testing.T) {
	_, _, err := parseSamplesheetCSV("")
	if err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestParseSamplesheetCSV_WhitespaceOnly(t *testing.T) {
	_, _, err := parseSamplesheetCSV("   \n\t\n  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only content, got nil")
	}
}

func TestParseSamplesheetCSV_HeadersOnly(t *testing.T) {
	headers, rows, err := parseSamplesheetCSV("sample,fastq_1,fastq_2\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"sample", "fastq_1", "fastq_2"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	if len(rows) != 0 {
		t.Fatalf("rows: got %d rows, want 0", len(rows))
	}
}

func TestParseSamplesheetCSV_HeadersOnlyNoTrailingNewline(t *testing.T) {
	headers, rows, err := parseSamplesheetCSV("sample,fastq_1,fastq_2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"sample", "fastq_1", "fastq_2"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	if len(rows) != 0 {
		t.Fatalf("rows: got %d rows, want 0", len(rows))
	}
}

func TestParseSamplesheetCSV_NormalCSV(t *testing.T) {
	content := "sample,fastq_1,fastq_2\nA,a1.fq,a2.fq\nB,b1.fq,b2.fq\n"
	headers, rows, err := parseSamplesheetCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"sample", "fastq_1", "fastq_2"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	wantRows := [][]string{
		{"A", "a1.fq", "a2.fq"},
		{"B", "b1.fq", "b2.fq"},
	}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows: got %v, want %v", rows, wantRows)
	}
}

func TestParseSamplesheetCSV_QuotedFields(t *testing.T) {
	content := "name,description\n\"Smith, John\",\"Has a \"\"nickname\"\"\"\n"
	headers, rows, err := parseSamplesheetCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"name", "description"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	wantRows := [][]string{
		{"Smith, John", `Has a "nickname"`},
	}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows: got %v, want %v", rows, wantRows)
	}
}

func TestParseSamplesheetCSV_SingleColumn(t *testing.T) {
	content := "id\n1\n2\n3\n"
	headers, rows, err := parseSamplesheetCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"id"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	if len(rows) != 3 {
		t.Fatalf("rows: got %d, want 3", len(rows))
	}
}

func TestParseSamplesheetCSV_FieldWithNewline(t *testing.T) {
	content := "name,bio\nAlice,\"line1\nline2\"\n"
	headers, rows, err := parseSamplesheetCSV(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantHeaders := []string{"name", "bio"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: got %v, want %v", headers, wantHeaders)
	}
	wantRows := [][]string{
		{"Alice", "line1\nline2"},
	}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("rows: got %v, want %v", rows, wantRows)
	}
}
