package server

import "testing"

func TestFormatBytes_Zero(t *testing.T) {
	got := formatBytes(0)
	want := "0 B"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_OneByte(t *testing.T) {
	got := formatBytes(1)
	want := "1 B"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_SmallBytes(t *testing.T) {
	got := formatBytes(512)
	want := "512 B"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_MaxBytes(t *testing.T) {
	got := formatBytes(1023)
	want := "1023 B"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_ExactlyOneKB(t *testing.T) {
	got := formatBytes(1024)
	want := "1.0 KB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_FractionalKB(t *testing.T) {
	// 1536 = 1.5 * 1024
	got := formatBytes(1536)
	want := "1.5 KB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_LargeKB(t *testing.T) {
	// 500 * 1024 = 512000
	got := formatBytes(512000)
	want := "500.0 KB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_ExactlyOneMB(t *testing.T) {
	got := formatBytes(1048576)
	want := "1.0 MB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_FractionalMB(t *testing.T) {
	// 2.5 * 1048576 = 2621440
	got := formatBytes(2621440)
	want := "2.5 MB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_ExactlyOneGB(t *testing.T) {
	got := formatBytes(1073741824)
	want := "1.0 GB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_LargeGB(t *testing.T) {
	// 10 * 1073741824 = 10737418240
	got := formatBytes(10737418240)
	want := "10.0 GB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatBytes_FractionalGB(t *testing.T) {
	// 1.5 * 1073741824 = 1610612736
	got := formatBytes(1610612736)
	want := "1.5 GB"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
