package store

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Пределы справочника устройств.
const (
	MaxDevices       = 256
	MaxDeviceNameLen = 40
)

// Типы устройств: только для значка в интерфейсе.
var DeviceKinds = []string{"pc", "laptop", "phone", "tablet", "tv", "console", "speaker", "nas", "printer", "camera", "router", "wifi", "iot", "other"}

var reDeviceID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// Device — устройство LAN: на него ссылаются правила (src=@id) и мониторинг.
// Addr — один IP или сеть (для группы адресов).
type Device struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Addr string `json:"addr"`
	Kind string `json:"kind,omitempty"`
}

// Prefix возвращает адрес устройства как сеть (одиночный IP — /32 или /128).
func (d Device) Prefix() (netip.Prefix, bool) {
	s := strings.TrimSpace(d.Addr)
	if p, err := netip.ParsePrefix(s); err == nil && p.Bits() > 0 {
		return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()).Masked(), true
	}
	if a, err := netip.ParseAddr(s); err == nil && !a.Is4In6() && !a.IsUnspecified() {
		return netip.PrefixFrom(a, a.BitLen()), true
	}
	return netip.Prefix{}, false
}

// ValidDeviceID проверяет идентификатор устройства.
func ValidDeviceID(id string) bool { return reDeviceID.MatchString(id) }

// NormalizeDevices выдаёт пустым идентификаторам свободные d1, d2, …, чистит имена и
// типы и проверяет список целиком.
func NormalizeDevices(in []Device) ([]Device, error) {
	if len(in) > MaxDevices {
		return nil, fmt.Errorf("устройств не больше %d", MaxDevices)
	}
	out := make([]Device, len(in))
	used := map[string]bool{}
	for i, d := range in {
		d.ID = strings.ToLower(strings.TrimSpace(d.ID))
		d.Name = strings.Join(strings.Fields(d.Name), " ")
		d.Addr = strings.TrimSpace(d.Addr)
		if d.Kind == "" {
			d.Kind = "other"
		}
		out[i] = d
		if d.ID != "" {
			used[d.ID] = true
		}
	}
	next := 1
	for i := range out {
		if out[i].ID != "" {
			continue
		}
		for used[fmt.Sprintf("d%d", next)] {
			next++
		}
		out[i].ID = fmt.Sprintf("d%d", next)
		used[out[i].ID] = true
	}
	ids, addrs := map[string]bool{}, map[netip.Prefix]string{}
	kinds := map[string]bool{}
	for _, k := range DeviceKinds {
		kinds[k] = true
	}
	for _, d := range out {
		label := d.Name
		if label == "" {
			label = d.Addr
		}
		switch {
		case !ValidDeviceID(d.ID):
			return nil, fmt.Errorf("устройство «%s»: идентификатор — латиница, цифры, _ и -, до 32 символов", label)
		case ids[d.ID]:
			return nil, fmt.Errorf("идентификатор устройства «%s» повторяется", d.ID)
		case d.Name == "":
			return nil, fmt.Errorf("у устройства %s нет имени", d.Addr)
		case utf8.RuneCountInString(d.Name) > MaxDeviceNameLen:
			return nil, fmt.Errorf("имя устройства «%s» длиннее %d символов", d.Name, MaxDeviceNameLen)
		case strings.ContainsAny(d.Name, "|#\r\n"):
			return nil, fmt.Errorf("имя устройства «%s»: символы | и # не допускаются", d.Name)
		case !kinds[d.Kind]:
			return nil, fmt.Errorf("устройство «%s»: неизвестный тип «%s»", d.Name, d.Kind)
		}
		p, ok := d.Prefix()
		if !ok {
			return nil, fmt.Errorf("устройство «%s»: адрес — IP или сеть, например 192.168.1.10 или 192.168.1.0/28", d.Name)
		}
		if other, dup := addrs[p]; dup {
			return nil, fmt.Errorf("адрес %s уже у устройства «%s»", p, other)
		}
		ids[d.ID], addrs[p] = true, d.Name
	}
	return out, nil
}

// DeviceByID возвращает устройство по идентификатору.
func (s State) DeviceByID(id string) (Device, bool) {
	for _, d := range s.Devices {
		if d.ID == id {
			return d, true
		}
	}
	return Device{}, false
}
