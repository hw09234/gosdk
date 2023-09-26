/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
)

//go:generate mockery --dir . --name FabricClient --case underscore  --output mocks/

type FabricClient interface {
	Init(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error)
	Invoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error)
	Query(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*peer.ProposalResponse, error)

	ListenEventFullBlock(channelName string, startNum uint64, fullBlockCh chan pBlock.Block) chan error
	ListenEventFilterBlock(channelName string, startNum uint64, filterBlockCh chan FilteredBlockResponse) chan error

	CreateUpdateChannel(channelId, path string) error
	JoinChannel(channelId, peerName string) (*peer.ProposalResponse, error)
	UpdateAnchorPeer(channelId, path string) error
	InstallChainCode(peerName string, req *InstallRequest) (*peer.ProposalResponse, error)
	InstantiateChainCode(policy string, req *ChainCode) (*orderer.BroadcastResponse, error)

	PackageChaincode(info ChaincodeInfo) error
	InstallChainCodeV2(chaincodePkg []byte, peerName string) (*lifecycle.InstallChaincodeResult, error)
	QueryInstalledV2(peerName string) ([]*lifecycle.QueryInstalledChaincodesResult_InstalledChaincode, error)
	GetInstalledPackageV2(PackageID, OutputDirectory, peerName string) error
	ApproveForMyOrg(acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error)
	QueryApproved(channelName, chaincodeName, orgName string, sequence int64) (*ApproveChaincode, error)
	CheckCommitreadiness(request CheckCommitreadinessRequest) (map[string]bool, error)
	LifecycleCommit(acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error)
	QueryCommitted(input CommittedQueryInput) (*lifecycle.QueryChaincodeDefinitionResult, *lifecycle.QueryChaincodeDefinitionsResult, error)

	// GetBlockHeight 注意返回的uint64为区块数目，比最后一个区块中的Num大1
	GetBlockHeight(channelName string) (uint64, error)
	GetBlockByNumber(channelName string, blockNum uint64) (*common.Block, error)
	GetBlockByTxID(channelName string, txID string) (*common.Block, error)
	GetTransactionByTxID(channelName string, txID string) (*peer.ProcessedTransaction, error)
	GetNewestBlock(channelName string) (*common.Block, error)

	DiscoveryChannelPeers(channel string) ([]ChannelPeer, error)
	DiscoveryLocalPeers() ([]LocalPeer, error)
	DiscoveryChannelConfig(channel string) (*discovery.ConfigResult, error)
	DiscoveryEndorsePolicy(channel string, chaincodes []string, collections map[string]string) ([]EndorsermentDescriptor, error)

	// AddUser 添加用户
	AddUser(userName string, userConfig UserConfig) error

	// GetCrypto 获取加密套件
	GetCrypto() (CryptoSuite, error)
}

//go:generate mockery --dir . --name ChaincodeClient --case underscore  --output mocks/

type ChaincodeClient interface {
	// Init 执行chaincode实例化操作，在approve和commit时，采用参数InitReqired决定是否必要执行
	Init(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error)

	// Invoke 调用合约提供的写入账本方法
	Invoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error)

	// SyncInvoke 同步调用合约提供的写入账本方法
	SyncInvoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*SyncInvokeResponse, error)

	// Query 调用合约提供的查询方法
	Query(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*peer.ProposalResponse, error)

	// AddUser 添加用户
	AddUser(userName string, userConfig UserConfig) error

	// Close 释放所有的连接
	Close() error

	// GetCrypto 获取加密套件
	GetCrypto() (CryptoSuite, error)
}

//go:generate mockery --dir . --name LedgerClient --case underscore  --output mocks/

type LedgerClient interface {
	// GetBlockHeight 注意返回的uint64为区块数目，比最后一个区块中的Num大1
	GetBlockHeight(channelName string) (uint64, error)

	// GetBlockByNumber 根据区块编号查询区块
	GetBlockByNumber(channelName string, blockNum uint64) (*common.Block, error)

	// GetBlockByTxID 根据transaction ID查询区块
	GetBlockByTxID(channelName string, txID string) (*common.Block, error)

	// GetTransactionByTxID 根据交易ID查询交易
	GetTransactionByTxID(channelName string, txID string) (*peer.ProcessedTransaction, error)

	// GetLastBlock 获取最新的一个区块
	GetNewestBlock(channelName string) (*common.Block, error)

	// Close 释放所有的连接
	Close() error

	// GetCrypto 获取加密套件
	GetCrypto() (CryptoSuite, error)
}

//go:generate mockery --dir . --name EventClient --case underscore  --output mocks/

type EventClient interface {
	// ListenEventFullBlock 从指定的区块号开始监听区块，返回的区块结构已被解析
	ListenEventFullBlock(startNum uint64, fullBlockCh chan pBlock.Block) chan error

	// ListenEventFilterBlock 从指定的区块号开始监听过滤的区块
	ListenEventFilterBlock(startNum uint64, filterBlockCh chan FilteredBlockResponse) chan error

	// Disconnect 关闭与peer的连接
	Disconnect()

	//CloseFullBlockListen 关闭监听full block client
	CloseFullBlockListen()

	//CloseFilteredBlockListen 关闭监听filtered block client
	CloseFilteredBlockListen()

	// GetCrypto 获取加密套件
	GetCrypto() (CryptoSuite, error)
}

//go:generate mockery --dir . --name DiscoveryClient --case underscore  --output mocks/

// DiscoveryClient 用于服务发现，注意当前所有的函数只能支持一种查询（即函数名所示功能）
type DiscoveryClient interface {
	// DiscoveryChannelPeers 发现通道中所有的peer
	// output:
	//    []ChannelPeer: 已加入通道的所有peer信息，ChannelPeer为自定义的结构体
	DiscoveryChannelPeers(channel string) ([]ChannelPeer, error)

	// DiscoveryChannelPeers 发现本地peer
	// output:
	//    []LocalPeer: 当不指定通道时，得到本地的peer
	DiscoveryLocalPeers() ([]LocalPeer, error)

	// DiscoveryChannelPeers 发现通道配置
	// output:
	//    *discovery.ConfigResult: 返回通道的配置，ConfigResult为fabric定义的结构体
	DiscoveryChannelConfig(channel string) (*discovery.ConfigResult, error)

	// DiscoveryEndorsePolicy 针对某个通道，查询chaincode和私有数据的collection的背书策略
	// input:
	//    collections: 私有数据collection集合(可以为空)，key为chaincode name，value为collection name,
	//   多个collection以","分割，例如map["mycc"]="collection1,collection2"
	// output:
	//    []EndorsermentDescriptor: 背书策略信息，EndorsermentDescriptor为自定义结构体
	DiscoveryEndorsePolicy(channel string, chaincodes []string, collections map[string]string) ([]EndorsermentDescriptor, error)

	// GetCrypto 获取加密套件
	GetCrypto() (CryptoSuite, error)
}
