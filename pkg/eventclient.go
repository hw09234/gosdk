/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"math"
	"sync/atomic"
	"time"

	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/pkg/errors"
)

type EventClientImpl struct {
	current         uint64
	cName           string           // 通道名
	crypto          CryptoSuite      // 加密套件
	identity        *Identity        // 身份信息
	listeners       []*EventListener // 监听器
	retryTimeout    time.Duration    // 尝试重连超时时间
	retryInterval   time.Duration    // 尝试重连的时间间隔
	initiativeClose bool             // 是否主动关闭
}

// getNextListener 获取下一个可用listener
func (ec *EventClientImpl) getNextListener() *EventListener {
	logger.Debug("enter get an available peer progress")
	defer logger.Debug("exit get an available peer progress")

	preCurrent := int(ec.current)
	next := int(ec.current)
	l := len(ec.listeners)
	// 如果当前游标对应节点可用，则直接返回
	if ec.listeners[next].peer.isAlive() {
		logger.Debugf("the current peer %s is available, no need to choose the next one", ec.listeners[next].peer.uri)
		return ec.listeners[next]
	}
	startTime := time.Now()
	count := 0
	for {
		currentTime := time.Now()
		next = (next + 1) % l
		if ec.initiativeClose {
			return nil
		}
		// 所有节点已被循环一遍，无可用节点，直接返回空
		if next == preCurrent && currentTime.Sub(startTime) > ec.retryTimeout {
			logger.Errorf("retry find the available peer time out %v, all peers are not alive, return nil", ec.retryTimeout)
			return nil
		}

		// 所有节点循环一遍后，进行一定时间的等待，防止长时间进入无限死循环
		if next == preCurrent {
			count++
			logger.Debugf("retry find the available peer times : %v\n", count)
			time.Sleep(ec.retryInterval)
		}

		// 如果节点可用，则修改current游标，并返回对应的节点
		if ec.listeners[next].peer.isAlive() {
			logger.Debugf("currently using peer %s", ec.listeners[next].peer.uri)
			atomic.StoreUint64(&ec.current, uint64(next))
			return ec.listeners[next]
		}
		logger.Warnf("%s is unavailable, choose next", ec.listeners[next].peer.uri)
	}
}

// ListenEventFullBlock 从指定的区块号对通道进行监听
// 如果startNum等于math.MaxUint64，则从最新的区块开始监听，否则从当前区块开始一直监听
func (ec *EventClientImpl) ListenEventFullBlock(startNum uint64, fullBlockCh chan pBlock.Block) chan error {
	logger.Debug("enter listen event full block progress")
	defer logger.Debug("exit listen event full block progress")

	var (
		err      error
		listener *EventListener
		blockNum uint64
		current  = ec.current
		errRes   = make(chan error, 1)
	)

	listener = ec.listeners[current]
	go func() {
		defer func() {
			close(errRes)
		}()

		for {
			if listener == nil {
				errRes <- errors.New("all listeners are unavailable")
				return
			}

			if err = listener.newClient(EventTypeFullBlock); err != nil {
				logger.Warningf("create full block client failed: %v", err)
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = ec.getNextListener()
				continue
			}

			if blockNum != 0 {
				startNum = blockNum + 1
			}

			if startNum == math.MaxUint64 {
				err = listener.SeekNewest(EventTypeFullBlock, ec.cName, *ec.identity, listener.peer, ec.crypto)
			} else {
				err = listener.SeekRange(startNum, math.MaxUint64, EventTypeFullBlock, ec.cName, *ec.identity, listener.peer, ec.crypto)
			}

			if err != nil {
				logger.Warningf("seek block number failed: %s", err.Error())
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = ec.getNextListener()
				continue
			}

			blockNum, err = listener.listenEventFullBlock(fullBlockCh, ec.crypto)
			if err == ErrCloseEvent {
				// 用户主动退出event
				return
			}
			if err == ErrAbnormalCloseEvent {
				errRes <- err
				// 异常退出event
				return
			}

			if listener.peer.conn == nil {
				return
			}

			go listener.peer.waitRecovery()
			listener.peer.setAlive(false)
			listener = ec.getNextListener()
		}
	}()

	return errRes
}

// ListenEventFilterBlock 从指定的区块号开始监听过滤的区块
func (ec *EventClientImpl) ListenEventFilterBlock(startNum uint64, filterBlockCh chan FilteredBlockResponse) chan error {
	logger.Debug("enter listen event filter block progress")
	defer logger.Debug("exit listen event filter block progress")

	var (
		err      error
		listener *EventListener
		blockNum uint64
		current  = ec.current
		errRes   = make(chan error, 1)
	)

	listener = ec.listeners[current]
	go func() {
		defer func() {
			close(errRes)
		}()

		for {
			if listener == nil {
				errRes <- errors.New("all listeners are unavailable")
				return
			}

			if err = listener.newClient(EventTypeFiltered); err != nil {
				logger.Warningf("create filter block client failed: %v", err)
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = ec.getNextListener()
				continue
			}

			if blockNum != 0 {
				startNum = blockNum + 1
			}

			if startNum == math.MaxUint64 {
				err = listener.SeekNewest(EventTypeFiltered, ec.cName, *ec.identity, listener.peer, ec.crypto)
			} else {
				err = listener.SeekRange(startNum, math.MaxUint64, EventTypeFiltered, ec.cName, *ec.identity, listener.peer, ec.crypto)
			}
			if err != nil {
				logger.Warningf("seek block number failed: %s", err.Error())
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = ec.getNextListener()
				continue
			}

			blockNum, err = listener.listenEventFilterBlock(filterBlockCh)
			if err == ErrCloseEvent {
				// 用户主动退出event
				return
			}
			if err == ErrAbnormalCloseEvent {
				// 异常退出event
				return
			}

			if listener.peer.conn == nil {
				return
			}

			go listener.peer.waitRecovery()
			listener.peer.setAlive(false)
			listener = ec.getNextListener()
		}
	}()

	return errRes
}

// CloseFullBlockListen 关闭监听
func (ec *EventClientImpl) CloseFullBlockListen() {
	for _, listener := range ec.listeners {
		logger.Debugf("start CloseFullBlockListen \n")
		if listener.fullClient != nil && listener.fullClient.client != nil {
			logger.Debugf("CloseFullBlockListen listener peer name : %#v\n", listener.peer.name)
			listener.fullClient.client.CloseSend()
			listener.fullClient.cancel()
			listener.fullClient.initiativeClose = true
			//if _, ok := <-listener.fullClient.stopCh; ok {
			//close(listener.fullClient.stopCh)
			//}
		}
	}
	ec.initiativeClose = true
}

// CloseFilteredBlockListen 关闭监听
func (ec *EventClientImpl) CloseFilteredBlockListen() {
	for _, listener := range ec.listeners {
		logger.Debugf("start CloseFilteredBlockListen \n")
		if listener.filteredClient != nil && listener.filteredClient.client != nil {
			logger.Debugf("CloseFilteredBlockListen listener peer name : %#v\n", listener.peer.name)
			listener.filteredClient.client.CloseSend()
			listener.filteredClient.cancel()
			listener.filteredClient.initiativeClose = true
			//close(listener.filteredClient.stopCh)
		}
	}
	ec.initiativeClose = true
}

// Disconnect 关闭与peer的连接
func (ec *EventClientImpl) Disconnect() {
	for _, listener := range ec.listeners {
		listener.peer.mux.Lock()
		if err := listener.DisConnect(); err != nil && listener.peer.conn != nil {
			listener.peer.mux.Unlock()
			logger.Errorf("disconnect failed: %s", err.Error())
			continue
		}
		listener.peer.conn = nil
		listener.peer.mux.Unlock()
	}
}

func (ec *EventClientImpl) GetCrypto() (CryptoSuite, error) {
	if ec.crypto == nil {
		return nil, errors.New("crypto suite is nil")
	}

	return ec.crypto, nil
}
