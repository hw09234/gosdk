package syncTxStatus

import (
	"sync"
)

type TxSyncChan chan *TxStatusEvent

//监听交易同步Map(协程并发安全)
type TxSyncMap struct {
	data map[string]TxSyncChan
	*sync.RWMutex
}

func NewTxSyncMap(data map[string]TxSyncChan) *TxSyncMap {
	return &TxSyncMap{data, &sync.RWMutex{}}
}

func (d *TxSyncMap) Len() int {
	d.RLock()
	defer d.RUnlock()
	return len(d.data)
}

func (d *TxSyncMap) Put(key string, value TxSyncChan) bool {
	d.Lock()
	defer d.Unlock()
	_, ok := d.data[key]
	if !ok {
		d.data[key] = value
	}
	return ok
}

func (d *TxSyncMap) Get(key string) (TxSyncChan, bool) {
	d.RLock()
	defer d.RUnlock()
	oldValue, ok := d.data[key]
	return oldValue, ok
}

func (d *TxSyncMap) Delete(key string) {
	d.Lock()
	defer d.Unlock()
	curChan, ok := d.data[key]
	if ok {
		delete(d.data, key)
		close(curChan)
	}
}
