// This code is under BSD license. See license-bsd.txt
package store

import "sync"

type StoreCsv struct {
	sync.Mutex
}

const (
	transRecId = "tr"
)

type StringInterner struct {
	strings []string
	strToId map[string]int
}

func NewStringInterner() *StringInterner {
	return &StringInterner{
		strings: make([]string, 0),
		strToId: make(map[string]int),
	}
}

// returns
func (i *StringInterner) GetId(s string) (id int, newString bool) {
	if id, exists := i.strToId[s]; exists {
		return id, false
	} else {
		id = len(i.strings)
		i.strToId[s] = id
		i.strings = append(i.strings, s)
		return id, true
	}
}
