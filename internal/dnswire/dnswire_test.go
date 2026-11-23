package dnswire

import (
	"encoding/binary"
	"math/rand"
	"net/netip"
	"testing"
)

// response собирает ответ: вопрос + записи A/AAAA со сжатием имени (указатель на вопрос).
func response(t *testing.T, name string, addrs ...string) []byte {
	t.Helper()
	q, ok := Query(0x1234, name, TypeA)
	if !ok {
		t.Fatal("query")
	}
	q[2] |= 0x80
	binary.BigEndian.PutUint16(q[6:8], uint16(len(addrs)))
	for _, a := range addrs {
		ip := netip.MustParseAddr(a)
		typ, raw := TypeA, ip.AsSlice()
		if ip.Is6() {
			typ = TypeAAAA
		}
		q = append(q, 0xC0, 12, byte(typ>>8), byte(typ), 0, 1, 0, 0, 0x0E, 0x10, 0, byte(len(raw)))
		q = append(q, raw...)
	}
	return q
}

func TestQuestionAndAnswers(t *testing.T) {
	msg := response(t, "WWW.Example.COM.", "93.184.216.34", "2606:2800:220:1::248")
	name, qt, end, ok := Question(msg)
	if !ok || name != "www.example.com" || qt != TypeA || end != 12+17+4 {
		t.Fatalf("вопрос: %q %d %d %v", name, qt, end, ok)
	}
	got := Answers(msg)
	if len(got) != 2 || got[0].Addr.String() != "93.184.216.34" || got[1].Addr.String() != "2606:2800:220:1::248" || got[0].TTL != 3600 {
		t.Fatalf("ответы: %+v", got)
	}
}

func TestErrorReplyKeepsQuestion(t *testing.T) {
	q, _ := Query(0xBEEF, "blocked.example", TypeA)
	r := ErrorReply(q, RcodeNXDomain)
	if ID(r) != 0xBEEF || r[2]&0x80 == 0 || r[3]&0x0F != RcodeNXDomain || r[2]&0x01 == 0 {
		t.Fatalf("заголовок ответа: % x", r[:4])
	}
	if name, _, _, ok := Question(r); !ok || name != "blocked.example" {
		t.Fatalf("вопрос должен сохраниться: %q", name)
	}
	if !IsResponseTo(r, 0xBEEF) || IsResponseTo(q, 0xBEEF) {
		t.Fatal("IsResponseTo")
	}
}

func TestRejectsLoopsAndOversize(t *testing.T) {
	// указатель сам на себя
	loop := []byte{0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0xC0, 12, 0, 1, 0, 1}
	if _, _, _, ok := Question(loop); ok {
		t.Fatal("цикл сжатия должен отвергаться")
	}
	long := make([]byte, 12)
	long[5] = 1
	for i := 0; i < 5; i++ {
		long = append(long, 63)
		long = append(long, make([]byte, 63)...)
	}
	long = append(long, 0, 0, 1, 0, 1)
	if _, _, _, ok := Question(long); ok {
		t.Fatal("имя длиннее 255 байт должно отвергаться")
	}
	// указатель вперёд (RFC 1035 разрешает только ссылки на уже встреченное имя)
	fwd := []byte{0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0xC0, 18, 0, 1, 0, 1, 1, 'a', 0}
	if _, _, _, ok := Question(fwd); ok {
		t.Fatal("указатель вперёд должен отвергаться")
	}
	if _, ok := Query(1, "a..b", TypeA); ok {
		t.Fatal("пустая метка")
	}
}

func TestGarbageNeverPanics(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	base := response(t, "example.org", "192.0.2.1", "192.0.2.2")
	for i := 0; i < 20000; i++ {
		var b []byte
		if i%2 == 0 {
			b = make([]byte, r.Intn(80))
			r.Read(b)
		} else {
			b = append([]byte(nil), base...)
			for j := 0; j < 1+r.Intn(4); j++ {
				b[r.Intn(len(b))] = byte(r.Intn(256))
			}
			b = b[:r.Intn(len(b)+1)]
		}
		Question(b)
		Answers(b)
		ErrorReply(b, RcodeServFail)
	}
}
