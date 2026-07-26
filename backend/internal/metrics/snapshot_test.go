package metrics

import (
	"strings"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{500, "500 B"},
		{2048, "2 KB"},
		{5 * 1024 * 1024, "5.0 MB"},
		{2 * 1024 * 1024 * 1024, "2.00 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Fatalf("FormatBytes(%d)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestFormatTelegramEmpty(t *testing.T) {
	got := FormatTelegram(nil)
	if !strings.Contains(got, "Nenhum container") && !strings.Contains(got, "0</b> containers") {
		t.Fatalf("got %q", got)
	}
}
