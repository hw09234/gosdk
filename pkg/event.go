/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"io"
	"math"
	"sync"
	"time"

	pBlock "github.com/hw09234/gohfc/pkg/parseBlock"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const (
	EventTypeFullBlock = iota
	EventTypeFiltered
)

const (
	maxRecvMsgSize = 100 * 1024 * 1024
	maxSendMsgSize = 100 * 1024 * 1024
)

var (
	oldest  = &orderer.SeekPosition{Type: &orderer.SeekPosition_Oldest{Oldest: &orderer.SeekOldest{}}}
	newest  = &orderer.SeekPosition{Type: &orderer.SeekPosition_Newest{Newest: &orderer.SeekNewest{}}}
	maxStop = &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}}
)

//go:generate mockery --dir . --name deliveryClient --case underscore  --output mocks/

type deliveryClient interface {
	Send(*common.Envelope) error
	Recv() (*peer.DeliverResponse, error)
	CloseSend() error
}

type eventclient struct {
	client          deliveryClient
	cancel          context.CancelFunc // 关闭stream
	stopCh          chan struct{}
	initiativeClose bool
}

type EventListener struct {
	peer           *Peer
	fullBlock      bool
	fullClient     *eventclient
	filteredClient *eventclient
}

type FilteredBlockResponse struct {
	Error        error
	ChannelId    string
	BlockNumber  uint64
	Transactions []EventBlockResponseTransaction
	RawBlock     []byte
}

type EventBlockResponseTransaction struct {
	Id          string
	Type        common.HeaderType
	Status      peer.TxValidationCode
	ChainCodeId string
	Events      []EventBlockResponseTransactionEvent
}

type EventBlockResponseTransactionEvent struct {
	Name  string
	Value []byte
}

// newEventPeer 根据配置信息构建与event peer的连接参数
// 此函数并未与event peer建立连接，只是构建参数
func newEventPeer(nodeConfig nodeConfig, crypto CryptoSuite) (*Peer, error) {
	n, err := newNode(nodeConfig, crypto)
	if err != nil {
		return nil, errors.Errorf("new node for event peer fialed: %s", err.Error())
	}

	return &Peer{node: *n, once: new(sync.Once)}, nil
}

func (e *EventListener) newConnection(peer *Peer) error {
	if e.peer.conn != nil {
		return nil
	}

	ctxDial, cancel := context.WithTimeout(context.Background(), peer.timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctxDial, peer.uri, peer.opts...)
	if err != nil {
		return errors.Errorf("cannot make new connection to: %s err: %v", peer.uri, err)
	}
	e.peer.conn = conn

	return nil
}

func (e *EventListener) newClient(listenerType int) error {
	var err error

	ctxClient, cancel := context.WithCancel(context.Background())
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	switch listenerType {
	case EventTypeFiltered:
		var client peer.Deliver_DeliverFilteredClient
		//if e.filteredClient != nil {
		//	return nil
		//}
		client, err = peer.NewDeliverClient(e.peer.conn).DeliverFiltered(ctxClient)
		if err != nil {
			logger.Errorf("create filtered client failed: %v", err)
			return err
		}
		e.filteredClient = &eventclient{
			client: client,
			cancel: cancel,
			stopCh: make(chan struct{}),
		}
	case EventTypeFullBlock:
		var client peer.Deliver_DeliverClient
		//if e.fullClient != nil {
		//	return nil
		//}
		logger.Debugf("start event peer.NewDeliverClient \n")
		client, err = peer.NewDeliverClient(e.peer.conn).Deliver(ctxClient)
		if err != nil {
			logger.Errorf("create full client failed: %v", err)
			return err
		}
		logger.Debugf("end event peer.NewDeliverClient \n")
		e.fullClient = &eventclient{
			client: client,
			cancel: cancel,
			stopCh: make(chan struct{}),
		}
	default:
		err = errors.Errorf("invalid listener type provided")
	}

	return err
}

func (e *EventListener) DisConnect() error {
	if e.peer != nil && e.peer.conn != nil {
		return e.peer.conn.Close()
	}
	return nil
}

func (e *EventListener) SeekNewest(listenerType int, channelID string, identity Identity, peer *Peer, crypto CryptoSuite) error {
	seek, err := e.createSeekEnvelope(newest, maxStop, channelID, identity, peer, crypto)
	if err != nil {
		logger.Errorf("create seek envelope failed: %v", err)
		return err
	}

	switch listenerType {
	case EventTypeFiltered:
		if e.peer.conn == nil || e.filteredClient == nil {
			return errors.Errorf("cannot seek no connection or client")
		}
		return e.filteredClient.client.Send(seek)
	case EventTypeFullBlock:
		if e.peer.conn == nil || e.fullClient == nil {
			return errors.Errorf("cannot seek no connection or client")
		}
		return e.fullClient.client.Send(seek)
	}
	return nil
}

func (e *EventListener) SeekRange(start, end uint64, listenerType int, channelID string, identity Identity, peer *Peer, crypto CryptoSuite) error {
	if start > end {
		return errors.Errorf("start: %d cannot be bigger than end: %d", start, end)
	}
	startPos := &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: start}}}
	endPos := &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: end}}}
	seek, err := e.createSeekEnvelope(startPos, endPos, channelID, identity, peer, crypto)
	if err != nil {
		return err
	}

	switch listenerType {
	case EventTypeFiltered:
		if e.peer.conn == nil || e.filteredClient == nil {
			return errors.Errorf("cannot seek no connection or client")
		}
		return e.filteredClient.client.Send(seek)
	case EventTypeFullBlock:
		if e.peer.conn == nil || e.fullClient == nil {
			return errors.Errorf("cannot seek no connection or client")
		}
		return e.fullClient.client.Send(seek)
	}

	return nil
}

func (e *EventListener) listenEventFullBlock(response chan<- pBlock.Block, crypto CryptoSuite) (uint64, error) {
	var blockNum uint64
	msgCh := make(chan *peer.DeliverResponse)
	errCh := make(chan error)
	flag := false

	//defer func() {
	//	e.fullClient = nil
	//}()

	go func() {
		for {
			originMsg, err := e.fullClient.client.Recv()
			if err != nil {
				logger.Errorf("full client errCh : %v\n", err)
				errCh <- err
				if err == io.EOF {
					continue
				}
			}
			if originMsg != nil {
				logger.Debugf("full client msgCh \n")
				msgCh <- originMsg
			}
			if flag {
				return
			}
		}
	}()

	for {
		select {
		//case stopch := <-e.fullClient.stopCh: // TODO: 用户主动退出event。由于没有新区块时，client.Recv()会进行阻塞，所以只有在来新区块之前关闭。
		//	logger.Debugf("full client stopCh : %v\n", stopch)
		//	flag = true
		//	e.fullClient = nil
		//	return blockNum, ErrCloseEvent
		case msg := <-msgCh:
			logger.Debugf("full client block %#v\n", msg)
			switch msg.Type.(type) {
			case *peer.DeliverResponse_Block:
				b := msg.GetBlock()
				if b != nil {
					size := uint64(len(msg.String()))
					block := pBlock.ParseBlock(msg.GetBlock(), size, crypto)
					blockNum = block.Header.Number
					response <- block
				} else {
					flag = true
					return blockNum, ErrAbnormalCloseEvent
				}
			case *peer.DeliverResponse_Status:
				tmpData := msg.Type.(*peer.DeliverResponse_Status)
				logger.Warningf("full client receive block msg peer.DeliverResponse_Status : %+v", *tmpData)
				flag = true
				if tmpData.Status == common.Status_FORBIDDEN {
					return blockNum, ErrAbnormalCloseEvent
				}
				return blockNum, errors.Errorf("full client receive msg peer.DeliverResponse_Status : %v", *tmpData)
			}
		case err := <-errCh:
			logger.Warningf("full client receive block failed: %v", err)
			flag = true
			if e.fullClient.initiativeClose {
				logger.Debugf("full client initiativeClose : %v\n", e.fullClient.initiativeClose)
				flag = true
				e.fullClient = nil
				return blockNum, ErrCloseEvent
			}
			return blockNum, errors.Errorf("full client receive block failed: %v", err)
		}
	}
}

func (e *EventListener) listenEventFilterBlock(filterResponse chan<- FilteredBlockResponse) (uint64, error) {
	var blockNum uint64
	msgCh := make(chan *peer.DeliverResponse)
	errCh := make(chan error)
	flag := false

	//defer func() {
	//	e.filteredClient = nil
	//}()

	go func() {
		for {
			originMsg, err := e.filteredClient.client.Recv()
			if err != nil {
				logger.Errorf("filter client errCh : %v\n", err)
				errCh <- err
				if err == io.EOF {
					continue
				}
			}
			if originMsg != nil {
				logger.Debugf("filter client msgCh\n")
				msgCh <- originMsg
			}
			if flag {
				return
			}
		}
	}()

	for {
		select {
		//case stopch := <-e.filteredClient.stopCh: // TODO: 用户主动退出event。由于没有新区块时，client.Recv()会进行阻塞，所以只有在来新区块之前关闭。
		//	logger.Debugf("filter client stopCh : %v\n", stopch)
		//	flag = true
		//	return blockNum, ErrCloseEvent
		case msg := <-msgCh:
			logger.Debugf("filter client block %#v\n", msg)
			switch msg.Type.(type) {
			case *peer.DeliverResponse_FilteredBlock:
				b := msg.Type.(*peer.DeliverResponse_FilteredBlock)
				if b != nil {
					block := *e.parseFilteredBlock(msg.Type.(*peer.DeliverResponse_FilteredBlock), e.fullBlock)
					blockNum = block.BlockNumber
					filterResponse <- block
				} else {
					flag = true
					return blockNum, ErrAbnormalCloseEvent
				}
			case *peer.DeliverResponse_Status:
				tmpData := msg.Type.(*peer.DeliverResponse_Status)
				logger.Warningf("filter client receive block msg peer.DeliverResponse_Status : %+v", *tmpData)
				flag = true
				if tmpData.Status == common.Status_FORBIDDEN {
					return blockNum, ErrAbnormalCloseEvent
				}
				return blockNum, errors.Errorf("filter client receive msg peer.DeliverResponse_Status : %v", *tmpData)
			}
		case err := <-errCh:
			logger.Warningf("filter client receive block failed: %v", err)
			flag = true
			if e.filteredClient.initiativeClose {
				logger.Debugf("filter client initiativeClose : %v\n", e.filteredClient.initiativeClose)
				flag = true
				return blockNum, ErrCloseEvent
			}

			return blockNum, errors.Errorf("filter client receive block failed: %v", err)
		}
	}
}

func (e *EventListener) parseFilteredBlock(block *peer.DeliverResponse_FilteredBlock, fullBlock bool) *FilteredBlockResponse {
	response := &FilteredBlockResponse{
		ChannelId:    block.FilteredBlock.ChannelId,
		BlockNumber:  block.FilteredBlock.Number,
		Transactions: make([]EventBlockResponseTransaction, 0),
	}
	if fullBlock {
		m, err := proto.Marshal(block.FilteredBlock)
		if err != nil {
			response.Error = err
			return response
		}
		response.RawBlock = m
	}

	for _, t := range block.FilteredBlock.FilteredTransactions {
		transaction := EventBlockResponseTransaction{
			Type:   t.Type,
			Id:     t.Txid,
			Status: t.TxValidationCode,
		}

		if t.Type != common.HeaderType_ENDORSER_TRANSACTION {
			continue
		}
		switch data := t.Data.(type) {
		case *peer.FilteredTransaction_TransactionActions:
			if len(data.TransactionActions.ChaincodeActions) > 0 {
				transaction.ChainCodeId = data.TransactionActions.ChaincodeActions[0].ChaincodeEvent.ChaincodeId
				for _, e := range data.TransactionActions.ChaincodeActions {
					transaction.Events = append(transaction.Events, EventBlockResponseTransactionEvent{
						Name: e.ChaincodeEvent.EventName,
					})
				}
			}
			response.Transactions = append(response.Transactions, transaction)
		default:
			response.Error = errors.Errorf("filterd actions are with unknown type: %T", t.Data)
			return response
		}
	}
	return response
}

func (e *EventListener) createSeekEnvelope(start *orderer.SeekPosition, stop *orderer.SeekPosition, channelID string, identity Identity, peer *Peer, crypto CryptoSuite) (*common.Envelope, error) {
	marshaledIdentity, err := marshalProtoIdentity(identity)
	if err != nil {
		return nil, err
	}
	nonce, err := generateRandomBytes(24)
	if err != nil {
		return nil, err
	}

	channelHeader, err := proto.Marshal(&common.ChannelHeader{
		Type:    int32(common.HeaderType_DELIVER_SEEK_INFO),
		Version: 0,
		Timestamp: &timestamp.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   0,
		},
		ChannelId:   channelID,
		Epoch:       0,
		TlsCertHash: peer.tlsCertHash,
	})
	if err != nil {
		return nil, err
	}

	sigHeader, err := proto.Marshal(&common.SignatureHeader{
		Creator: marshaledIdentity,
		Nonce:   nonce,
	})
	if err != nil {
		return nil, err
	}

	data, err := proto.Marshal(&orderer.SeekInfo{
		Start:    start,
		Stop:     stop,
		Behavior: orderer.SeekInfo_BLOCK_UNTIL_READY,
	})
	if err != nil {
		return nil, err
	}

	payload, err := proto.Marshal(&common.Payload{
		Header: &common.Header{
			ChannelHeader:   channelHeader,
			SignatureHeader: sigHeader,
		},
		Data: data,
	})
	if err != nil {
		return nil, err
	}

	sig, err := crypto.Sign(payload, identity.PrivateKey)
	if err != nil {
		return nil, err
	}

	return &common.Envelope{Payload: payload, Signature: sig}, nil
}

func newEventListener(crypto CryptoSuite, peer *Peer) (*EventListener, error) {
	if crypto == nil {
		return nil, errors.Errorf("cryptoSuite cannot be nil")
	}

	listener := EventListener{
		fullBlock: false,
		peer:      peer,
	}

	if err := listener.newConnection(peer); err != nil {
		return nil, err
	}

	return &listener, nil
}
