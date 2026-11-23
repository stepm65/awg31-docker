// Package dnswire — минимальный разбор DNS-сообщений (RFC 1035), ровно то, что
// нужно форвардеру: вопрос, адреса из ответа, ответ-ошибка и простой запрос.
// Все функции терпимы к мусору на входе: при любой неожиданности возвращают
// ok=false, а не паникуют.
package dnswire

import (
	"encoding/binary"
	"net/netip"
	"strings"
)

// Типы записей.
const (
	TypeA    uint16 = 1
	TypeAAAA uint16 = 28
)

// Коды ответа.
const (
	RcodeServFail = 2
	RcodeNXDomain = 3
	RcodeRefused  = 5
)

const headerLen = 12

// Question разбирает первый вопрос запроса: имя в нижнем регистре без точки в
// конце, тип и смещение конца вопроса.
func Question(msg []byte) (name string, qtype uint16, end int, ok bool) {
	if len(msg) < headerLen || binary.BigEndian.Uint16(msg[4:6]) < 1 {
		return "", 0, 0, false
	}
	name, off, ok := readName(msg, headerLen, 0)
	if !ok || off+4 > len(msg) {
		return "", 0, 0, false
	}
	return name, binary.BigEndian.Uint16(msg[off : off+2]), off + 4, true
}

// readName читает имя с учётом сжатия (не больше 10 переходов, не длиннее 255).
func readName(msg []byte, off, depth int) (string, int, bool) {
	var labels []string
	total := 0
	end := -1
	for jumps := 0; ; {
		if off >= len(msg) {
			return "", 0, false
		}
		l := int(msg[off])
		switch {
		case l == 0:
			if end < 0 {
				end = off + 1
			}
			return strings.ToLower(strings.Join(labels, ".")), end, true
		case l&0xC0 == 0xC0:
			if off+1 >= len(msg) || jumps >= 10 || depth > 0 {
				return "", 0, false
			}
			if end < 0 {
				end = off + 2
			}
			ptr := int(binary.BigEndian.Uint16(msg[off:off+2]) & 0x3FFF)
			if ptr >= off {
				return "", 0, false // ссылки только назад: без циклов
			}
			off = ptr
			jumps++
		case l&0xC0 != 0:
			return "", 0, false
		default:
			if off+1+l > len(msg) {
				return "", 0, false
			}
			total += l + 1
			if total > 255 {
				return "", 0, false
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}

func skipName(msg []byte, off int) (int, bool) {
	_, end, ok := readName(msg, off, 0)
	return end, ok
}

// Answer — адрес из ответа с его TTL.
type Answer struct {
	Addr netip.Addr
	TTL  uint32
}

// Answers возвращает A и AAAA записи из секции ответов.
func Answers(msg []byte) []Answer {
	if len(msg) < headerLen {
		return nil
	}
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	off := headerLen
	for i := 0; i < qd; i++ {
		var ok bool
		if off, ok = skipName(msg, off); !ok || off+4 > len(msg) {
			return nil
		}
		off += 4
	}
	var out []Answer
	for i := 0; i < an && i < 512; i++ {
		var ok bool
		if off, ok = skipName(msg, off); !ok || off+10 > len(msg) {
			return out
		}
		typ := binary.BigEndian.Uint16(msg[off : off+2])
		class := binary.BigEndian.Uint16(msg[off+2 : off+4])
		ttl := binary.BigEndian.Uint32(msg[off+4 : off+8])
		rdlen := int(binary.BigEndian.Uint16(msg[off+8 : off+10]))
		off += 10
		if off+rdlen > len(msg) {
			return out
		}
		if class == 1 {
			switch {
			case typ == TypeA && rdlen == 4:
				out = append(out, Answer{netip.AddrFrom4([4]byte(msg[off : off+4])), ttl})
			case typ == TypeAAAA && rdlen == 16:
				out = append(out, Answer{netip.AddrFrom16([16]byte(msg[off : off+16])), ttl})
			}
		}
		off += rdlen
	}
	return out
}

// ErrorReply строит ответ с кодом rcode на запрос (только заголовок и вопрос).
func ErrorReply(query []byte, rcode byte) []byte {
	end := headerLen
	if _, _, e, ok := Question(query); ok {
		end = e
	} else if len(query) < headerLen {
		return nil
	}
	resp := make([]byte, end)
	copy(resp, query[:end])
	resp[2] = 0x80 | (query[2] & 0x79) // QR=1, сохранить opcode и RD
	resp[3] = 0x80 | (rcode & 0x0F)    // RA=1
	if end == headerLen {
		binary.BigEndian.PutUint16(resp[4:6], 0)
	} else {
		binary.BigEndian.PutUint16(resp[4:6], 1)
	}
	binary.BigEndian.PutUint16(resp[6:8], 0)
	binary.BigEndian.PutUint16(resp[8:10], 0)
	binary.BigEndian.PutUint16(resp[10:12], 0)
	return resp
}

// Query строит запрос с одним вопросом (для разрешения имени DoH-сервера).
func Query(id uint16, name string, qtype uint16) ([]byte, bool) {
	name = strings.TrimSuffix(strings.ToLower(name), ".")
	if name == "" || len(name) > 253 {
		return nil, false
	}
	b := make([]byte, headerLen, headerLen+len(name)+6)
	binary.BigEndian.PutUint16(b[0:2], id)
	b[2] = 0x01 // RD
	binary.BigEndian.PutUint16(b[4:6], 1)
	for _, l := range strings.Split(name, ".") {
		if l == "" || len(l) > 63 {
			return nil, false
		}
		b = append(b, byte(len(l)))
		b = append(b, l...)
	}
	b = append(b, 0, byte(qtype>>8), byte(qtype), 0, 1)
	return b, true
}

// ID возвращает идентификатор сообщения.
func ID(msg []byte) uint16 {
	if len(msg) < 2 {
		return 0
	}
	return binary.BigEndian.Uint16(msg[0:2])
}

// IsResponseTo — msg ответ на запрос с этим ID (QR=1).
func IsResponseTo(msg []byte, id uint16) bool {
	return len(msg) >= headerLen && ID(msg) == id && msg[2]&0x80 != 0
}
