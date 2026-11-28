package journal

import (
	"fmt"
	"log"
	"testing"
)

func TestRingAndSince(t *testing.T) {
	j := New(3)
	for i := 1; i <= 5; i++ {
		j.Add(fmt.Sprintf("line %d", i))
	}
	all, last := j.Since(0, 0)
	if last != 5 || len(all) != 3 || all[0].Text != "line 3" || all[2].Seq != 5 {
		t.Fatalf("%v %d", all, last)
	}
	got, _ := j.Since(4, 0)
	if len(got) != 1 || got[0].Text != "line 5" {
		t.Fatalf("since 4: %v", got)
	}
	got, _ = j.Since(0, 2)
	if len(got) != 2 || got[0].Text != "line 4" {
		t.Fatalf("limit: %v", got)
	}
	if got, _ := j.Since(5, 0); len(got) != 0 {
		t.Fatalf("since last: %v", got)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ text, src, lvl string }{
		{"policy: geo databases: файл не найден", "policy", Info},
		{"WARNING: ALLOW_INSECURE_SUBSCRIPTIONS=on", "", Warn},
		{"tunnel LV: строка DNS не применяется в режиме шлюза", "tunnel", Warn},
		{"autostart: tunnel LV is up", "autostart", Info},
		{"ERROR: listen tcp :8443: address already in use", "", Error},
		{"[entrypoint] architecture: x86_64", "entrypoint", Info},
		{"switching to tunnel b failed: exit status 1", "", Error},
		{"ip rule 'lookup local' was missing (-4), restored", "", Warn},
		{"failover: main is unhealthy but there is no healthy replacement", "failover", Warn},
		{"BGP: connection refused, retry in 30s", "bgp", Warn},
		{"Open the WebUI and enter the setup token (SETUP TOKEN): abc", "", Info},
	}
	for _, c := range cases {
		src, lvl := Classify(c.text)
		if src != c.src || lvl != c.lvl {
			t.Errorf("%q: got %q/%q, want %q/%q", c.text, src, lvl, c.src, c.lvl)
		}
	}
}

func TestWriterSplitsLines(t *testing.T) {
	j := New(10)
	l := log.New(&Writer{J: j}, "", 0)
	l.Printf("one")
	l.Printf("two\nthree")
	log.New(&Writer{J: j}, "", log.LstdFlags).Printf("four")
	got, _ := j.Since(0, 0)
	if len(got) != 4 || got[2].Text != "three" || got[3].Text != "four" {
		t.Fatalf("%v", got)
	}
}

func TestSetupTokenHidden(t *testing.T) {
	j := New(2)
	j.Add("Open the WebUI and enter the setup token (SETUP TOKEN): 0123abcd")
	got, _ := j.Since(0, 0)
	if got[0].Text != "Open the WebUI and enter the setup token (SETUP TOKEN): ***" {
		t.Fatal(got[0].Text)
	}
}
