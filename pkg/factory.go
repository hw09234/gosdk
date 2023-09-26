/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"github.com/hw09234/gosdk/pkg/syncTxStatus"
	"github.com/hyperledger/fabric/common/flogging"
	"github.com/pkg/errors"
	"time"
)

var logger = flogging.MustGetLogger("gosdk")

// NewFabricClient 根据结构体或者配置文件初始化Fabric句柄，包含所有的功能
func NewFabricClient(clientConfig *ClientConfig, configFile string) (FabricClient, error) {
	var err error

	// 当存在配置文件时，根据配置文件初始化
	if configFile != "" {
		clientConfig, err = initConfigFile(configFile)
	}
	if err != nil {
		return nil, err
	}

	client, err := newFabricClientFromConfig(*clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "new fabric client failed")
	}

	ide := make(map[string]Identity)
	if err := newUsers(clientConfig.Users, ide); err != nil {
		return nil, errors.WithMessage(err, "create users failed")
	}

	cPeers, ccPeers, err := parsePolicy(clientConfig.Channels, client.orgPeers, client.Peers)
	if err != nil {
		return nil, errors.WithMessage(err, "parse policy failed")
	}

	return &FabricClientImpl{
		client:                client,
		identity:              ide,
		channelChaincodePeers: ccPeers,
		channelPeers:          cPeers,
	}, nil
}

// NewChaincodeClient 根据结构体初始化Chaincode句柄
func NewChaincodeClient(config ChaincodeConfig) (ChaincodeClient, error) {
	var ord *OrderersInfo

	if config.PConfigs == nil || config.Channels == nil {
		return nil, errors.New("peer config, channel name or chaincode name is nil")
	}

	c, err := newECCryptSuiteFromConfig(config.CryptoConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}

	orgPeers := make(map[string][]*Peer)
	peers := make(map[string]*Peer)
	for _, peerConfig := range config.PConfigs {
		p, err := newPeer(peerConfig, c)
		if err != nil {
			return nil, errors.WithMessage(err, "create Peer failed")
		}
		orgPeers[peerConfig.OrgName] = append(orgPeers[peerConfig.OrgName], p)
		peers[p.uri] = p
	}

	channelPeers, ccPeers, err := parsePolicy(config.Channels, orgPeers, peers)
	if err != nil {
		return nil, errors.WithMessage(err, "parse policy failed")
	}

	ide := make(map[string]Identity)
	if err := newUsers(config.Users, ide); err != nil {
		return nil, errors.WithMessage(err, "create users failed")
	}

	// 如果orderer的配置为空，则不建立与orderer的连接，该句柄只能执行Query方法，无法执行Invoke方法
	if config.OConfigs != nil {
		orderers := make([]*Orderer, 0)
		for _, o := range config.OConfigs {
			newOrderer, err := newOrderer(o, c)
			if err != nil {
				return nil, errors.Errorf("new orderer failed: %s", err.Error())
			}
			logger.Debugf("get orderer %s success", newOrderer.uri)
			orderers = append(orderers, newOrderer)
		}
		ord = &OrderersInfo{orderers: orderers}
	}
	var curSyncTxHandler syncTxStatus.SyncTxHandler
	// 开启同步交易功能
	if config.IsEnableSyncTx {
		// 实例化同步交易处理句柄
		curSyncTxHandler = syncTxStatus.NewHandleSyncTxStatus(config.SyncTxWaitTime)
		// 创建监听区块状态句柄
		if err := createSyncTxEventHandle(channelPeers, config, curSyncTxHandler); err != nil {
			return nil, err
		}
	} else {
		curSyncTxHandler = nil
	}
	return &ChaincodeClientImpl{
		CryptoSuite:           c,
		OrderersInfo:          ord,
		SyncTxHandler:         curSyncTxHandler,
		identity:              ide,
		orgPeers:              orgPeers,
		channelChaincodePeers: ccPeers,
		SyncTxWaitTime:        config.SyncTxWaitTime}, nil
}

// NewDiscoveryClient 根据结构体初始化discovery句柄
// 初始化过程中不会与节点建立连接，所有的连接在使用discovery功能时才建立，且是短连接，使用完成后释放。
// input: DiscoveryConfig: 配置信息
// output: Sdk: discovery实例，可以直接调用discovery服务。例如: sdk.DiscoveryChannelPeers("mychannel")
func NewDiscoveryClient(config DiscoveryConfig) (DiscoveryClient, error) {
	c, err := newECCryptSuiteFromConfig(config.CryptoConfig)
	if err != nil {
		return nil, err
	}

	peers := make([]*node, 0)
	for _, peerConfig := range config.PeerConfigs {
		n, err := newNode(peerConfig, c)
		if err != nil {
			return nil, errors.Errorf("new node failed: %s", err.Error())
		}
		peers = append(peers, n)
	}

	id, err := newIdentityFromUserConfig(config.UserConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	return &DiscoveryClientImpl{
		CryptoSuite: c,
		Identity:    id,
		Peers:       peers,
	}, nil
}

// NewEventClient 根据结构体初始化event句柄
func NewEventClient(config EventConfig) (EventClient, error) {
	if config.CName == "" {
		return nil, errors.New("channel name is nil")
	}
	crypto, err := newECCryptSuiteFromConfig(config.CryptoConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}

	id, err := newIdentityFromUserConfig(config.UserConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	listeners := make([]*EventListener, 0)
	if len(config.Peers) == 0 {
		for _, peerConfig := range config.PeerConfigs {
			peer, err := newEventPeer(peerConfig, crypto)
			if err != nil {
				return nil, errors.WithMessage(err, "create Peer failed")
			}
			listener, err := newEventListener(crypto, peer)
			if err != nil {
				return nil, errors.Errorf("new event listener failed: %s", err.Error())
			}
			listeners = append(listeners, listener)
		}
	} else {
		// 此处操作为了兼容chaincodeClient同步发交易使用
		for _, peer := range config.Peers {
			listener, err := newEventListener(crypto, peer)
			if err != nil {
				return nil, errors.Errorf("new event listener failed: %s", err.Error())
			}
			listeners = append(listeners, listener)
		}
	}

	if config.RetryTimeout == 0 {
		config.RetryTimeout = time.Minute * 30
	}
	if config.RetryInterval == 0 {
		config.RetryTimeout = time.Second * 2
	}

	return &EventClientImpl{
		crypto:        crypto,
		identity:      id,
		cName:         config.CName,
		listeners:     listeners,
		retryTimeout:  config.RetryTimeout,
		retryInterval: config.RetryInterval,
	}, nil
}

// NewLedgerClinet 根据结构体初始化ledger句柄，用于对账本的查询
// 只与peer节点建立连接，调用系统链码进行查询，且只与一个peer建立连接
func NewLedgerClient(config LedgerConfig) (LedgerClient, error) {
	c, err := newECCryptSuiteFromConfig(config.CryptoConfig)
	if err != nil {
		return nil, err
	}

	peers := make([]*Peer, 0)
	for _, peerConfig := range config.PeersConfig {
		p, err := newPeer(peerConfig, c)
		if err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	peersInfo := &PeersInfo{peers: peers}

	id, err := newIdentityFromUserConfig(config.UserConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	return &LedgerClientImpl{
		crypto:    c,
		peersInfo: peersInfo,
		identity:  id}, nil
}
