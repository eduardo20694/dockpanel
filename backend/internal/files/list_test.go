package files

import "testing"

func TestParseLS(t *testing.T) {
	raw := "total 4\ndrwxr-xr-x 2 0 0 4096 Jan 1 00:00 app\n-rw-r--r-- 1 0 0 12 Jan 1 00:00 config.yml\n"
	got := ParseLS("/var/lib", raw)
	if len(got) != 2 || !got[0].Dir || got[0].Path != "/var/lib/app" || got[1].Size != 12 {
		t.Fatalf("%+v", got)
	}
}
