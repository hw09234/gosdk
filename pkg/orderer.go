/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type OrderersInfo struct {
	current  uint64
	orderers []*Orderer
}

// getNextOrderer 获取下一个可用orderer
func (oi *OrderersInfo) getNextOrderer() *Orderer {
	logger.Debug("enter get an available orderer progress")
	defer logger.Debug("exit get an available orderer progress")

	preCurrent := int(oi.current)
	next := int(oi.current)
	l := len(oi.orderers)
	// 如果当前游标对应节点可用，则直接返回
	if oi.orderers[next].isAlive() {
		logger.Debugf("the current orderer %s is available, no need to choose the next one", oi.orderers[next].uri)
		return oi.orderers[next]
	}
	for {
		next = (next + 1) % l
		// 所有节点已被循环一遍，无可用节点，直接返回空
		if next == preCurrent {
			logger.Error("all orderers are not alive, return nil")
			return nil
		}
		// 如果节点可用，则修改current游标，并返回对应的节点
		if oi.orderers[next].isAlive() {
			logger.Debugf("currently using orerer %s", oi.orderers[next].uri)
			atomic.StoreUint64(&oi.current, uint64(next))
			return oi.orderers[next]
		}
		logger.Warnf("%s is unavailable, choose next", oi.orderers[next].uri)
	}
}

// Orderer expose API's to communicate with orderers.
type Orderer struct {
	once    *sync.Once
	mux     sync.RWMutex
	conn    *grpc.ClientConn
	oClient orderer.AtomicBroadcastClient
	node
}

// setAlive 更改orderer状态
func (o *Orderer) setAlive(alive bool) {
	o.mux.Lock()
	logger.Debugf("orderer %s's alive set to be %t", o.uri, alive)
	o.alive = alive
	o.mux.Unlock()
}

// isAlive 判断orderer是否可用
func (o *Orderer) isAlive() (alive bool) {
	o.mux.RLock()
	alive = o.alive
	o.mux.RUnlock()
	return
}

// waitRecovery 监听连接状态变化，更改orderer状态
func (o *Orderer) waitRecovery() {
	o.once.Do(func() {
		logger.Infof("orderer %s enter wait recovery", o.uri)
		for {
			if o.conn == nil {
				return
			}
			preState := o.conn.GetState()
			if o.conn.WaitForStateChange(context.Background(), preState) {
				if o.conn.GetState() == connectivity.Ready {
					logger.Infof("orderer %s's state change to be ready", o.uri)
					o.setAlive(true)
					o.once = new(sync.Once)
					return
				}
			}
		}
	})
}

const timeout = 5

// newOrderer 根据配置信息构建与orderer的连接
func newOrderer(nodeConfig nodeConfig, crypto CryptoSuite) (*Orderer, error) {
	n, err := newNode(nodeConfig, crypto)
	if err != nil {
		logger.Errorf("new node for orderer failed: %s", err.Error())
		return nil, errors.Errorf("new node for orderer failed: %s", err.Error())
	}

	ctxDial, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()
	c, err := grpc.DialContext(ctxDial, n.uri, n.opts...)
	if err != nil {
		return nil, errors.Errorf("connect orderer %s failed: %s", n.uri, err.Error())
	}

	o := orderer.NewAtomicBroadcastClient(c)
	return &Orderer{conn: c, oClient: o, node: *n, once: new(sync.Once)}, nil
}

// Broadcast Broadcast envelope to orderer for execution.
func (o *Orderer) broadcast(envelope *common.Envelope) (*orderer.BroadcastResponse, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bcc, err := o.oClient.Broadcast(ctx)
	if err != nil {
		logger.Errorf("broadcast failed: %v", err)
		return nil, err
	}
	defer bcc.CloseSend()
	bcc.Send(envelope)

	response, err := bcc.Recv()
	if err != nil {
		logger.Errorf("broadcast receive failed: %v", err)
		return nil, err
	}
	// TODO 查找何种情况下status不为success，以及此时err的状态
	if response.Status != common.Status_SUCCESS {
		logger.Errorf("response status with %d", response.Status)
		return nil, ErrResponse
	}

	return response, err
}

// Deliver delivers envelope to orderer. Please note that new connection will be created on every call of Deliver.
func (o *Orderer) deliver(envelope *common.Envelope) (*common.Block, error) {
	logger.Debug("enter orderer deliver progress")
	defer logger.Debug("exit orderer deliver progress")

	var block *common.Block
	var ctx = context.Background()

	ctxDial, cancelDial := context.WithTimeout(ctx, o.timeout)
	defer cancelDial()
	connection, err := grpc.DialContext(ctxDial, o.uri, o.opts...)
	if err != nil {
		logger.Errorf("connect to orderer %s failed: %s", o.uri, err.Error())
		return nil, errors.Errorf("connect to orderer %s failed: %s", o.uri, err.Error())
	}
	defer connection.Close()

	dk, err := orderer.NewAtomicBroadcastClient(connection).Deliver(context.Background())
	if err != nil {
		logger.Errorf("generate deliver client failed: %v", err)
		return nil, err
	}
	defer dk.CloseSend()

	if err := dk.Send(envelope); err != nil {
		logger.Errorf("send envelope failed: %v", err)
		return nil, err
	}

	timer := time.NewTimer(time.Second * time.Duration(timeout))
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			logger.Errorf("receive timeout")
			return nil, ErrOrdererTimeout
		default:
			response, err := dk.Recv()
			if err != nil {
				logger.Errorf("receive failed: %v", err)
				return nil, err
			}

			// orderer分发区块时会进行两次发送，第一次发送DeliverResponse_Block，
			// 第二次发送DeliverResponse_Status以判断是否分发区块成功
			switch t := response.Type.(type) {
			case *orderer.DeliverResponse_Status:
				if t.Status == common.Status_SUCCESS {
					logger.Debugf("get block success with %d", block.Header.Number)
					return block, nil
				} else {
					logger.Errorf("deliver failed with response status %d", t.Status)
					return nil, ErrResponse
				}
			case *orderer.DeliverResponse_Block:
				block = response.GetBlock()
				logger.Debugf("get block %d", block.Header.Number)
			default:
				logger.Errorf("unknown response type from orderer: %v", t)
				return nil, errors.Errorf("unknown response type from orderer: %v", t)
			}
		}
	}
}

func (oi *OrderersInfo) fabricManagerGenesisBlock(identity Identity, crypto CryptoSuite, channelId string) (*common.Block, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := createEnvWithSeekInfo(seekInfo, identity, crypto, channelId)
	if err != nil {
		logger.Errorf("create envelop with orderer.SeekInfo failed: %v", err)
		return nil, errors.WithMessage(err, "create envelop with orderer.SeekInfo failed")
	}

	block, err := sendToOrderersDeliver(oi, env)
	return block, err
}

func (oi *OrderersInfo) getLastConfigBlock(identity Identity, crypto CryptoSuite, channelID string) (*common.Block, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
	if err != nil {
		return nil, errors.WithMessage(err, "create envelop with orderer.SeekInfo failed")
	}
	newestBlock, err := sendToOrderersDeliver(oi, env)
	if err != nil {
		return nil, errors.WithMessage(err, "get newest block failede")
	}
	lci, err := utils.GetLastConfigIndexFromBlock(newestBlock)
	if err != nil {
		return nil, errors.WithMessage(err, "get last config index from block failed")
	}

	seekInfo = &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: lci}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: lci}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err = createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
	if err != nil {
		return nil, err
	}

	block, err := sendToOrderersDeliver(oi, env)
	return block, err
}

func getGenesisBlock(orderers *OrderersInfo, identity Identity, crypto CryptoSuite, channelId string) (*common.Block, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: 0}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	env, err := createEnvWithSeekInfo(seekInfo, identity, crypto, channelId)
	if err != nil {
		return nil, errors.WithMessage(err, "create envelop with orderer.SeekInfo failed")
	}
	return sendToOrderersDeliver(orderers, env)
}

// createEnvpForOldestBlock 创建获取最早区块的envp结构体
func createEnvpForOldestBlock(identity Identity, crypto CryptoSuite, channelID string) (*common.Envelope, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Oldest{Oldest: &orderer.SeekOldest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Oldest{Oldest: &orderer.SeekOldest{}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	return createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
}

// createEnvpForNewestBlock 创建获取最新区块的envp结构体
func createEnvpForNewestBlock(identity Identity, crypto CryptoSuite, channelID string) (*common.Envelope, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	return createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
}

// createEnvpForBlockNum 创建获取指定区块号的envp结构体
func createEnvpForBlockNum(identity Identity, crypto CryptoSuite, channelID string, num uint64) (*common.Envelope, error) {
	seekInfo := &orderer.SeekInfo{
		Start:    &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: num}}},
		Stop:     &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: num}}},
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	}
	return createEnvWithSeekInfo(seekInfo, identity, crypto, channelID)
}

// createEnvWithSeekInfo 创建获取对应区块请求的envp结构体
func createEnvWithSeekInfo(seekInfo *orderer.SeekInfo, identity Identity, crypto CryptoSuite, channelID string) (*common.Envelope, error) {
	seekInfoBytes, err := proto.Marshal(seekInfo)
	if err != nil {
		logger.Errorf("proto marshal seek info failed: %v", err)
		return nil, err
	}

	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		logger.Errorf("get creator bytes failed: %v", err)
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		logger.Errorf("get transaction id failed: %v", err)
		return nil, err
	}

	headerBytes, err := channelHeader(common.HeaderType_DELIVER_SEEK_INFO, txId, channelID, 0, nil)
	if err != nil {
		logger.Errorf("get channel header bytes failed: %v", err)
		return nil, err
	}
	signatureHeaderBytes, err := signatureHeader(creator, txId)
	if err != nil {
		logger.Errorf("get channel header sinature bytes failed: %v", err)
		return nil, err
	}
	header := header(signatureHeaderBytes, headerBytes)
	payloadBytes, err := payload(header, seekInfoBytes)
	if err != nil {
		logger.Errorf("proto marshal header failed: %v", err)
		return nil, err
	}
	payloadSignedBytes, err := crypto.Sign(payloadBytes, identity.PrivateKey)
	if err != nil {
		logger.Errorf("get signature failed: %v", err)
		return nil, err
	}
	return &common.Envelope{Payload: payloadBytes, Signature: payloadSignedBytes}, nil
}

// sendToOrderersBroadcast 选择orderer发送envelope，当orderer不可连接时重选择orderer
func sendToOrderersBroadcast(orderers *OrderersInfo, envelope *common.Envelope) (reply *orderer.BroadcastResponse, err error) {
	logger.Debug("enter orderer broadcast progress")
	defer logger.Debug("exit orderer broadcast progress")

	var ord *Orderer
	var current = orderers.current

	ord = orderers.orderers[current]
	for {
		logger.Debugf("orderer %s try to broadcast", ord.uri)
		reply, err = ord.broadcast(envelope)
		// 发送给orderer处理成功，直接返回
		// 或orderer成功接收，但内部处理失败，直接返回
		if err == nil || err == ErrResponse {
			logger.Debugf("orderer %s complete broadcast", ord.uri)
			return
		}
		logger.Errorf("orderer %s broadcast failed: %s", ord.uri, err.Error())

		// 发送给orderer失败，选取下一个orderer继续发送
		ord.setAlive(false)
		go ord.waitRecovery()
		if ord = orderers.getNextOrderer(); ord == nil {
			return nil, errors.New("all orderers are unavailable to broadcast")
		}
	}
}

// sendToOrderersDeliver 选择orderer发送envelope，当orderer不可连接时重选择orderer
func sendToOrderersDeliver(orderers *OrderersInfo, envelope *common.Envelope) (block *common.Block, err error) {
	logger.Debug("enter orderer deliver progress")
	defer logger.Debug("exit orderer deliver progress")

	var ord *Orderer
	var current = orderers.current

	ord = orderers.orderers[current]
	for {
		logger.Debugf("orderer %s try to deliver", ord.uri)
		block, err = ord.deliver(envelope)
		// 发送给orderer处理成功，直接返回
		// 或orderer成功接收，但内部处理失败，直接返回
		if err == nil || err == ErrResponse {
			return
		}
		logger.Errorf("orderer %s deliver failed: %s", ord.uri, err.Error())

		// 发送给orderer失败，选取下一个orderer继续发送
		ord.setAlive(false)
		go ord.waitRecovery()
		if ord = orderers.getNextOrderer(); ord == nil {
			return nil, errors.New("all orderers are unavailable to deliver")
		}
	}
}
