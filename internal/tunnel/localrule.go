package tunnel

import (
	"context"
	"strings"
)

// EnsureLocalRule возвращает правило «0: from all lookup local», если его нет.
// Без него ядро не считает свои адреса локальными: пакеты к 127.0.0.1 и к адресу
// контейнера уходят по правилам маршрутизации в туннель или blackhole, WebUI и
// ARP перестают отвечать. На CHR (RouterOS 7.25) правило пропадало после
// обновления контейнера; кто его удаляет, не выяснено, поэтому проверяем сами.
// Возвращает семейства, для которых правило пришлось вернуть.
func EnsureLocalRule(ctx context.Context, r Runner, ipv6 bool) []string {
	fams := []string{"-4"}
	if ipv6 {
		fams = append(fams, "-6")
	}
	var fixed []string
	for _, fam := range fams {
		out, err := r.Run(ctx, "ip", fam, "rule", "show")
		if err != nil || HasLocalRule(string(out)) {
			continue
		}
		if _, err := r.Run(ctx, "ip", fam, "rule", "add", "priority", "0", "lookup", "local"); err == nil {
			fixed = append(fixed, fam)
		}
	}
	return fixed
}

// HasLocalRule сообщает, есть ли в выводе «ip rule show» правило lookup local.
func HasLocalRule(show string) bool {
	for _, line := range strings.Split(show, "\n") {
		f := strings.Fields(line)
		for i := 0; i+1 < len(f); i++ {
			if f[i] == "lookup" && (f[i+1] == "local" || f[i+1] == "255") {
				return true
			}
		}
	}
	return false
}
