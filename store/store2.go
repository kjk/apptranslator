// This code is under BSD license. See license-bsd.txt
package store

import "sync"

type StoreCsv struct {
	sync.Mutex
}

const (
	TransRecId = "tr"
)
