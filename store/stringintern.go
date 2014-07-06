// This code is under BSD license. See license-bsd.txt
package store

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

// returns existing id of the string and false if this string has been interned
// before. returns a newly allocated id for the string and true if this is
// a newly interned string
func (i *StringInterner) Intern(s string) (id int, isNew bool) {
	if id, exists := i.strToId[s]; exists {
		return id, false
	} else {
		id = len(i.strings)
		i.strToId[s] = id
		i.strings = append(i.strings, s)
		return id, true
	}
}

func (i *StringInterner) GetById(id int) (string, bool) {
	if id < 0 || id >= len(i.strings) {
		//fmt.Printf("no id %d in i.strings of len %d\n", id, len(i.strings))
		return "", false
	}
	return i.strings[id], true
}

func (i *StringInterner) IdByStrMust(s string) int {
	if id, exists := i.strToId[s]; !exists {
		panic("s not in i.strToId")
	} else {
		return id
	}
}

func (i *StringInterner) Count() int {
	return len(i.strings)
}
