package store

import (
	"fmt"
	"strings"
)

// Режимы группы туннелей.
const (
	GroupBalance = "balance" // новые соединения распределяются между исправными участниками
	GroupOrdered = "ordered" // трафик идёт через первого исправного участника по порядку списка
)

// Пределы: группа живёт в таблице маршрутов (150–169), а ECMP-маршрут с
// десятками веток ядро обрабатывает плохо.
const (
	MaxGroups       = 16
	MaxGroupMembers = 16
	MaxGroupWeight  = 100
)

// GroupMember — участник группы. Weight нужен только режиму balance (0 = 1).
type GroupMember struct {
	Name   string `json:"name"`
	Weight int    `json:"weight,omitempty"`
}

// W возвращает действующий вес участника.
func (m GroupMember) W() int {
	if m.Weight < 1 {
		return 1
	}
	return m.Weight
}

// Group — группа туннелей: на неё можно указывать в правилах (PROXY@имя) и
// назначать её основной. Порядок участников важен для режима ordered.
type Group struct {
	Name    string        `json:"name"`
	Mode    string        `json:"mode"` // balance | ordered
	Members []GroupMember `json:"members"`
}

// Group возвращает группу по имени.
func (s State) Group(name string) (Group, bool) {
	for _, g := range s.Groups {
		if g.Name == name {
			return g, true
		}
	}
	return Group{}, false
}

// HasGroup сообщает, что группа с таким именем задана.
func (s State) HasGroup(name string) bool { _, ok := s.Group(name); return ok }

// ValidateGroups проверяет список групп целиком. Имя группы не должно совпадать
// с именем профиля: оно стоит там же, где имя туннеля (PROXY@имя).
func ValidateGroups(groups []Group, profiles []string) error {
	if len(groups) > MaxGroups {
		return fmt.Errorf("групп не больше %d", MaxGroups)
	}
	isProfile := map[string]bool{}
	for _, p := range profiles {
		isProfile[p] = true
	}
	names := map[string]bool{}
	for _, g := range groups {
		switch {
		case !ValidName(g.Name):
			return fmt.Errorf("имя группы «%s»: буквы, цифры, _ и -, до %d символов", g.Name, MaxNameLen)
		case isProfile[g.Name]:
			return fmt.Errorf("имя группы «%s» совпадает с профилем: правила PROXY@%s были бы неоднозначны", g.Name, g.Name)
		case names[strings.ToLower(g.Name)]:
			return fmt.Errorf("группа «%s» повторяется", g.Name)
		case g.Mode != GroupBalance && g.Mode != GroupOrdered:
			return fmt.Errorf("группа «%s»: режим balance или ordered", g.Name)
		case len(g.Members) > MaxGroupMembers:
			return fmt.Errorf("группа «%s»: не больше %d участников", g.Name, MaxGroupMembers)
		}
		names[strings.ToLower(g.Name)] = true
		seen := map[string]bool{}
		for _, m := range g.Members {
			switch {
			case !ValidName(m.Name):
				return fmt.Errorf("группа «%s»: недопустимое имя участника «%s»", g.Name, m.Name)
			case seen[m.Name]:
				return fmt.Errorf("группа «%s»: участник «%s» повторяется", g.Name, m.Name)
			case profiles != nil && !isProfile[m.Name]:
				return fmt.Errorf("группа «%s»: профиля «%s» нет", g.Name, m.Name)
			case m.Weight < 0 || m.Weight > MaxGroupWeight:
				return fmt.Errorf("группа «%s»: вес участника «%s» — от 1 до %d", g.Name, m.Name, MaxGroupWeight)
			}
			seen[m.Name] = true
		}
	}
	return nil
}

func cloneGroups(in []Group) []Group {
	if in == nil {
		return nil
	}
	out := make([]Group, len(in))
	for i, g := range in {
		out[i] = Group{Name: g.Name, Mode: g.Mode, Members: append([]GroupMember(nil), g.Members...)}
	}
	return out
}
