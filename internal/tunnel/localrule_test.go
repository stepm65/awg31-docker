package tunnel

import (
	"context"
	"strings"
	"testing"
)

type ruleRunner struct {
	show  map[string]string
	calls []string
}

func (r *ruleRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if len(args) == 3 && args[1] == "rule" && args[2] == "show" {
		return []byte(r.show[args[0]]), nil
	}
	return nil, nil
}

func TestEnsureLocalRule(t *testing.T) {
	r := &ruleRunner{show: map[string]string{
		"-4": "250:\tfrom all iif veth-awg31 lookup 210\n2147483646:\tfrom all lookup main\n",
		"-6": "0:\tfrom all lookup local\n32766:\tfrom all lookup main\n",
	}}
	got := EnsureLocalRule(context.Background(), r, true)
	if len(got) != 1 || got[0] != "-4" {
		t.Fatalf("fixed %v", got)
	}
	last := r.calls[len(r.calls)-2]
	if last != "ip -4 rule add priority 0 lookup local" {
		t.Fatalf("calls %q", r.calls)
	}
	if HasLocalRule("1103:\tfrom all fwmark 0x31000067 lookup 103") || !HasLocalRule("0:\tfrom all lookup 255") {
		t.Fatal("HasLocalRule")
	}
}
