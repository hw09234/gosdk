package syncTxStatus

import (
	"time"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
)

const SYNCTXWAITTIME = 120 * time.Second // 默认同步交易等待时间

//go:generate mockery --dir . --name SyncTxHandler --case underscore  --output mocks/

type SyncTxHandler interface {
	RegisterTxStatusEvent(txID string) (chan *TxStatusEvent, error)
	UnRegisterTxStatusEvent(txID string)
	PublishTxStatus(blockNumber uint64, txID string, txStatus peer.TxValidationCode)
	GetSyncTxWaitTime() time.Duration
}

// 处理同步交易状态
type HandleSyncTxStatus struct {
	SyncTxStatusMap *TxSyncMap    // 等待交易同步Map
	SyncTxWaitTime  time.Duration // 同步交易等待时间,默认120s
}

func NewHandleSyncTxStatus(waitTime time.Duration) *HandleSyncTxStatus {
	var handler HandleSyncTxStatus
	handler.SyncTxStatusMap = NewTxSyncMap(make(map[string]TxSyncChan, 2000))
	if waitTime == 0 {
		waitTime = SYNCTXWAITTIME
	}
	handler.SyncTxWaitTime = waitTime
	return &handler
}

type TxStatusEvent struct {
	BlockNumber      uint64
	TxValidationCode peer.TxValidationCode
}

func (wt *HandleSyncTxStatus) RegisterTxStatusEvent(txID string) (chan *TxStatusEvent, error) {
	if txID == "" {
		return nil, errors.New("RegisterTxStatusEvent txID must be provided")
	}
	statusChan := make(chan *TxStatusEvent)
	exist := wt.SyncTxStatusMap.Put(txID, statusChan)
	if exist {
		return nil, errors.Errorf("RegisterTxStatusEvent txID %s was already exist", txID)
	}
	return statusChan, nil
}

func (wt *HandleSyncTxStatus) UnRegisterTxStatusEvent(txID string) {
	wt.SyncTxStatusMap.Delete(txID)
}

func (wt *HandleSyncTxStatus) PublishTxStatus(blockNumber uint64, txID string, txStatus peer.TxValidationCode) {
	if txID == "" {
		return
	}
	go func(b uint64, t string, s peer.TxValidationCode) {
		statusChan, exist := wt.SyncTxStatusMap.Get(t)
		if exist {
			statusChan <- &TxStatusEvent{BlockNumber: b, TxValidationCode: s}
		}
	}(blockNumber, txID, txStatus)
}

func (wt *HandleSyncTxStatus) GetSyncTxWaitTime() time.Duration {
	return wt.SyncTxWaitTime
}
