package store

import (
	"strings"
	"testing"
)

func TestNormalizeDevices(t *testing.T) {
	out, err := NormalizeDevices([]Device{
		{Name: "  PC  ", Addr: "192.168.51.150", Kind: "pc"},
		{ID: "d1", Name: "TV", Addr: "192.168.51.10"},
		{Name: "Гостевые", Addr: "192.168.51.64/28", Kind: "wifi"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].ID != "d2" || out[0].Name != "PC" || out[1].ID != "d1" || out[1].Kind != "other" || out[2].ID != "d3" {
		t.Fatalf("%+v", out)
	}
	if p, ok := out[2].Prefix(); !ok || p.String() != "192.168.51.64/28" {
		t.Fatalf("сеть: %v", p)
	}
	for _, c := range []struct {
		in   []Device
		want string
	}{
		{[]Device{{Name: "a", Addr: "1.2.3.4"}, {Name: "b", Addr: "1.2.3.4/32"}}, "уже у устройства"},
		{[]Device{{Name: "", Addr: "1.2.3.4"}}, "нет имени"},
		{[]Device{{Name: "a|b", Addr: "1.2.3.4"}}, "не допускаются"},
		{[]Device{{Name: "a", Addr: "host.lan"}}, "адрес"},
		{[]Device{{Name: "a", Addr: "0.0.0.0/0"}}, "адрес"},
		{[]Device{{Name: "a", Addr: "1.2.3.4", Kind: "toaster"}}, "тип"},
		{[]Device{{ID: "X Y", Name: "a", Addr: "1.2.3.4"}}, "идентификатор"},
		{[]Device{{ID: "a", Name: "a", Addr: "1.2.3.4"}, {ID: "a", Name: "b", Addr: "1.2.3.5"}}, "повторяется"},
	} {
		if _, err := NormalizeDevices(c.in); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: ждали «%s», вышло %v", c.in, c.want, err)
		}
	}
}

func TestDevicesPersist(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(st *State) error {
		st.Devices = []Device{{ID: "pc", Name: "PC", Addr: "10.0.0.2", Kind: "pc"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := s2.Snapshot().DeviceByID("pc"); !ok || d.Addr != "10.0.0.2" {
		t.Fatalf("%+v", s2.Snapshot().Devices)
	}
}
