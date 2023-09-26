/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"time"

	"github.com/hw09234/gohfc/pkg/syncTxStatus"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
)

type ChaincodeClientImpl struct {
	CryptoSuite
	*OrderersInfo
	syncTxStatus.SyncTxHandler               // 同步交易操作句柄
	SyncTxWaitTime             time.Duration // 同步交易等待时间
	identity                   map[string]Identity
	orgPeers                   map[string][]*Peer
	channelChaincodePeers      map[string]map[string]*endorsePeers // 第一维为通道，第二维为智能合约，对应的peer信息
}

type ChaincodeConfig struct {
	CryptoConfig
	SyncTxWaitTime time.Duration // 同步交易等待时间
	IsEnableSyncTx bool          // 是否开启同步交易功能
	PConfigs       []PeerConfig
	OConfigs       []OrdererConfig
	Users          map[string]UserConfig
	Channels       []ChannelChaincodeConfig
	RetryTimeout   time.Duration
	RetryInterval  time.Duration
}

// getEndorsePeers 根据通道和合约名称获取对应的peer
func (cc *ChaincodeClientImpl) getEndorsePeers(cName, ccName string) (*endorsePeers, error) {
	if cc.channelChaincodePeers == nil {
		return nil, errors.New("no peer configured")
	}
	ccPeers, ok := cc.channelChaincodePeers[cName]
	if !ok {
		return nil, errors.Errorf("channel %s is not configured", cName)
	}
	peers, ok := ccPeers[ccName]
	if !ok {
		return nil, errors.Errorf("chaincode %s is not configured", ccName)
	}

	return peers, nil
}

// getBasicInfo 获取交易基本信息：identity，peer，chaincode信息
func (cc *ChaincodeClientImpl) getBasicInfo(args []string, transientMap map[string][]byte, cName, ccName, userName string, isInit bool) (*Identity, *endorsePeers, *ChainCode, error) {
	if userName == "" {
		userName = defaultUser
	}
	identity, ok := cc.identity[userName]
	if !ok {
		return nil, nil, nil, errors.Errorf("user %s is not configured", userName)
	}

	peers, err := cc.getEndorsePeers(cName, ccName)
	if err != nil {
		return nil, nil, nil, errors.WithMessage(err, "get endorse peers failed")
	}
	chaincode, err := getChainCodeObj(args, transientMap, cName, ccName, peers.chanincodeType, isInit)
	if err != nil {
		return nil, nil, nil, errors.WithMessage(err, "get chaincode object failed")
	}

	return &identity, peers, chaincode, nil
}

func (cc *ChaincodeClientImpl) Init(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error) {
	logger.Debug("enter chaincode client invoke progress")
	defer logger.Debug("exit chaincode client invoke progress")

	if cc.OrderersInfo == nil {
		return nil, errors.New("orderer is not configured, can not execute invoke operation")
	}

	identity, peers, chaincode, err := cc.getBasicInfo(args, transientMap, cName, ccName, userName, true)
	if err != nil {
		return nil, errors.WithMessage(err, "get init basic info failed")
	}

	proposal, sProposal, err := createSignedProposal(*identity, *chaincode, cc.CryptoSuite)
	if err != nil {
		logger.Errorf("create signed proposal failed: %v", err)
		return nil, err
	}

	propRes := sendToPeers(peers.invokePeers, sProposal, other)

	env, err := createEnvelope(proposal.proposal, propRes, *identity, cc.CryptoSuite)
	if err != nil {
		return nil, errors.Errorf("create envelope failed: %s", err.Error())
	}

	broadRes, err := sendToOrderersBroadcast(cc.OrderersInfo, env)
	if err != nil {
		return nil, errors.Errorf("broadcast to orderer failed: %s", err.Error())
	}

	return &InvokeResponse{Status: broadRes.Status, TxID: proposal.transactionId, Payload: propRes[0].Response.Response.Payload}, nil
}

func (cc *ChaincodeClientImpl) Invoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error) {
	logger.Debug("enter chaincode client invoke progress")
	defer logger.Debug("exit chaincode client invoke progress")

	if cc.OrderersInfo == nil {
		return nil, errors.New("orderer is not configured, can not execute invoke operation")
	}

	identity, peers, chaincode, err := cc.getBasicInfo(args, transientMap, cName, ccName, userName, false)
	if err != nil {
		return nil, errors.WithMessage(err, "get invoke basic info failed")
	}

	proposal, sProposal, err := createSignedProposal(*identity, *chaincode, cc.CryptoSuite)
	if err != nil {
		logger.Errorf("create signed proposal failed: %v", err)
		return nil, err
	}

	propRes := sendToPeers(peers.invokePeers, sProposal, other)

	env, err := createEnvelope(proposal.proposal, propRes, *identity, cc.CryptoSuite)
	if err != nil {
		return nil, errors.Errorf("create envelope failed: %s", err.Error())
	}

	broadRes, err := sendToOrderersBroadcast(cc.OrderersInfo, env)
	if err != nil {
		return nil, errors.Errorf("broadcast to orderer failed: %s", err.Error())
	}

	return &InvokeResponse{Status: broadRes.Status, TxID: proposal.transactionId, Payload: propRes[0].Response.Response.Payload}, nil
}

// 上链交易接口落账后返回
// 返回值： 同步交易响应结构,其他错误
// PS: TxStatus == peer.TxValidationCode_VALID 表示落账成功
func (cc *ChaincodeClientImpl) SyncInvoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*SyncInvokeResponse, error) {
	logger.Debug("enter chaincode client sync invoke progress")
	defer logger.Debug("exit chaincode client sync invoke progress")

	if cc.SyncTxHandler == nil {
		return nil, errors.New("SyncTxHandler Not initialized, Forbidden syncInvoke!")
	}
	invokeResponse, err := cc.Invoke(args, transientMap, cName, ccName, userName)
	if err != nil {
		logger.Errorf("invoke failed: %v", err)
		return nil, err
	}

	//listen tx status
	TxStatusChan, err := cc.RegisterTxStatusEvent(invokeResponse.TxID)
	if err != nil {
		logger.Errorf("register event failed: %v", err)
		return nil, err
	}
	defer cc.UnRegisterTxStatusEvent(invokeResponse.TxID)
	select {
	case txStatus := <-TxStatusChan:
		return &SyncInvokeResponse{Status: invokeResponse.Status, TxID: invokeResponse.TxID, BlockNumber: txStatus.BlockNumber, TxStatus: txStatus.TxValidationCode, Payload: invokeResponse.Payload}, nil
	case <-time.After(cc.GetSyncTxWaitTime()):
		return nil, errors.Errorf("txId %s which has`t been listened event for more than wait time %s", invokeResponse.TxID, cc.GetSyncTxWaitTime().String())
	}
}

func (cc *ChaincodeClientImpl) Query(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*peer.ProposalResponse, error) {
	logger.Debug("enter chaincode client query progress")
	defer logger.Debug("exit chaincode client query progress")

	identity, peers, chaincode, err := cc.getBasicInfo(args, transientMap, cName, ccName, userName, false)
	if err != nil {
		return nil, errors.WithMessage(err, "get query basic info failed")
	}

	_, sProposal, err := createSignedProposal(*identity, *chaincode, cc.CryptoSuite)
	if err != nil {
		return nil, err
	}

	return querySinglePeer(sProposal, peers.queryPeers)
}

func (cc *ChaincodeClientImpl) AddUser(userName string, userConfig UserConfig) error {
	user, err := newIdentityFromUserConfig(userConfig)
	if err != nil {
		return errors.WithMessagef(err, "create user %s failed", userName)
	}
	cc.identity[userName] = *user
	return nil
}

func (cc *ChaincodeClientImpl) Close() error {
	logger.Debug("enter close peer and orderer connection progress")
	defer logger.Debug("exit close peer and orderer connection progress")

	for _, peers := range cc.orgPeers {
		for _, peer := range peers {
			if peer != nil && peer.conn != nil {
				peer.mux.Lock()
				defer peer.mux.Unlock()
				if err := peer.conn.Close(); err != nil {
					logger.Errorf("close connection to peer %s failed: %s", peer.uri, err.Error())
					continue
				}
				peer.conn = nil
			}
		}
	}
	cc.channelChaincodePeers = nil

	for _, ord := range cc.orderers {
		if ord != nil && ord.conn != nil {
			if err := ord.conn.Close(); err != nil {
				logger.Errorf("close connection to orderer %s failed: %s", ord.uri, err.Error())
				continue
			}
			ord.conn = nil
		}
	}

	return nil
}

func (cc *ChaincodeClientImpl) GetCrypto() (CryptoSuite, error) {
	logger.Debug("enter get chaincode client crypto progress")
	defer logger.Debug("exit get chaincode client crypto progress")

	if cc.CryptoSuite == nil {
		return nil, errors.New("crypto suite is nil")
	}

	return cc.CryptoSuite, nil
}
