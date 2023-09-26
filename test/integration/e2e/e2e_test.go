package e2e

import (
	"context"
	"errors"
	"io/ioutil"
	"testing"
	"time"

	gosdk "github.com/hw09234/gosdk/pkg"
	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/stretchr/testify/assert"
)

const (
	policy            = "OR('Org1MSP.member', 'Org2MSP.member')"
	channelName       = "mychannel"
	chaincodeName     = "mycc"
	lifecycleName     = "_lifecycle"
	channelTXPath     = "../../fixtures/channel-artifacts/channel.tx"
	channelTXPathV2   = "../../fixtures/channel-artifactsV2/channel.tx"
	chaincodePath     = "../../fixtures/chaincode"
	chaincodePathV2   = "../../fixtures/chaincodeV2"
	ccoutPath         = "../../fixtures/example.out"
	gmChaincodePathV2 = "../../fixtures/gm-chaincodeV2"
	codetgz           = "../../test/fixtures/chaincode.tar.gz"

	org1AnchorPath = "../../fixtures/channel-artifacts/Org1MSPanchors.tx"
	org2AnchorPath = "../../fixtures/channel-artifacts/Org2MSPanchors.tx"

	ordererTLSPath    = "../../fixtures/crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt"
	org1PeerTLSPath   = "../../fixtures/crypto-config/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt"
	org1PeerTLSPathV2 = "../../fixtures/crypto-configV2/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt"
	org2PeerTLSPath   = "../../fixtures/crypto-config/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/server.crt"
	org1AdminPath     = "../../fixtures/crypto-config/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
	org1AdminPathV2   = "../../fixtures/crypto-configV2/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
	org2AdminPath     = "../../fixtures/crypto-config/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp"
)

var txID string
var cryptoConfig = gosdk.CryptoConfig{
	Family:    "ecdsa",
	Algorithm: "P256-SHA256",
	Hash:      "SHA2-256",
}

// TestE2E 使用转账业务进行集成测试
// 在测试过程中，对于需要写块的操作，调用返回后需要执行sleep以等待交易落块
func TestE2ENonGM(t *testing.T) {
	t.Log("Ready to new fabric client")
	org1ClientConfig := newOrg1ClientConfig(t, "v1")
	c1, err := gosdk.NewFabricClient(&org1ClientConfig, "")
	assert.Nil(t, err, "new org1 client failed")

	t.Run("deploy Fabric network and operate org1's node by fabric client", func(t *testing.T) {
		// 等待fabric容器启动，该fabric client只针对org1进行配置，所以只有org1的peer加入通道、安装cc
		// org2的peer未执行此操作
		time.Sleep(time.Second * 10)

		// 创建通道
		t.Log("Ready to create channel")
		err = c1.CreateUpdateChannel(channelName, channelTXPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 加入通道
		t.Log("Ready to join channel for peers")
		res, err := c1.JoinChannel(channelName, "peer01")
		assert.Nil(t, err, "org1's peer failed to join channel")
		assert.NotNil(t, res)
		time.Sleep(time.Second * 3)

		// 更新锚节点
		t.Log("Ready to update org1's anchor peer")
		err = c1.UpdateAnchorPeer(channelName, org1AnchorPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 安装chaincode
		t.Log("Ready to install chaincode on org1's peer")
		installRequest := &gosdk.InstallRequest{
			ChannelId:        channelName,
			ChainCodeName:    chaincodeName,
			ChainCodeVersion: "1.0",
			ChainCodeType:    gosdk.ChaincodeSpec_GOLANG,
			Namespace:        "github.com/PeerFintech/gosdk/test/fixtures/chaincode",
			SrcPath:          chaincodePath,
		}
		res, err = c1.InstallChainCode("peer01", installRequest)
		assert.Nil(t, err, "failed to install chaincode for org1's peer")
		time.Sleep(time.Second * 5)

		// 实例化chaincode
		// 未指定policy策略，默认为
		t.Log("Ready to instantiate chaincode")
		instantiateRequest := &gosdk.ChainCode{
			ChannelId: channelName,
			Name:      chaincodeName,
			Version:   "1.0",
			Type:      gosdk.ChaincodeSpec_GOLANG,
			Args:      []string{"init", "a", "100", "b", "200"},
		}
		instantiateRes, err := c1.InstantiateChainCode(policy, instantiateRequest)
		assert.Nil(t, err, "InstantiateChainCode failed")
		assert.NotNil(t, instantiateRes)
		assert.Equal(t, common.Status_SUCCESS, instantiateRes.Status)

		// 等待生成合约容器，此时fabric网络已完全创建，区块数目为4
		time.Sleep(time.Second * 40)

		t.Logf("Ready to discovery channel %s peers by fabric client", channelName)
		peers, err := c1.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 1, len(peers))
		assert.Equal(t, "peer0.org1.example.com:7051", peers[0].Endpoint)
		assert.Equal(t, "Org1MSP", peers[0].MSPID)
		assert.Equal(t, uint64(2), peers[0].LedgerHeight)
		assert.Equal(t, []string{chaincodeName}, peers[0].Chaincodes)
		assert.NotEmpty(t, peers[0].Identity)

		t.Log("Ready to discovery local peers by fabric client")
		lPeers, err := c1.DiscoveryLocalPeers()
		assert.Nil(t, err, "discovery local peers failed")
		assert.Equal(t, 1, len(lPeers))
		assert.Equal(t, "peer0.org1.example.com:7051", lPeers[0].Endpoint)
		assert.Equal(t, "Org1MSP", lPeers[0].MSPID)
		assert.NotEmpty(t, lPeers[0].Identity)

		t.Logf("Entering Discovery channel %s config by fabric client", channelName)
		cconfig, err := c1.DiscoveryChannelConfig(channelName)
		assert.Nil(t, err, "discovery channel config failed")
		assert.Equal(t, "OrdererMSP", cconfig.Msps["OrdererMSP"].Name)
		assert.Equal(t, "Org1MSP", cconfig.Msps["Org1MSP"].Name)
		assert.Equal(t, "Org2MSP", cconfig.Msps["Org2MSP"].Name)
		assert.Equal(t, &discovery.Endpoint{
			Host: "orderer.example.com",
			Port: 7050,
		}, cconfig.Orderers["OrdererMSP"].Endpoint[0])

		t.Log("Entering Discovery chaincodes endorse policy by fabric client")
		p, err := c1.DiscoveryEndorsePolicy(channelName, []string{chaincodeName}, nil)
		assert.Nil(t, err, "discovery config failed")
		assert.Equal(t, 1, len(p))
		assert.Equal(t, chaincodeName, p[0].Chaincode)
		t.Logf("endorses by group is %v", p[0].EndorsersByGroups)
		assert.Equal(t, 1, len(p[0].EndorsersByGroups))
		for _, v := range p[0].EndorsersByGroups {
			assert.Equal(t, 1, len(v))
			assert.Equal(t, uint64(2), v[0].LedgerHeight)
		}

		t.Log("Ready to get event block")
		cb := make(chan pBlock.Block)
		fullErr := c1.ListenEventFullBlock(channelName, 0, cb)
		blocksNum, err := recBlock(t, cb, nil, fullErr)
		assert.Nil(t, err)
		assert.Equal(t, 3, blocksNum)
		fcb := make(chan gosdk.FilteredBlockResponse)
		filterErr := c1.ListenEventFilterBlock(channelName, 0, fcb)
		blocksNum, err = recBlock(t, nil, fcb, filterErr)
		assert.Nil(t, err)
		assert.Equal(t, 3, blocksNum)
	})

	t.Run("test discovery client", func(t *testing.T) {
		dConfig := newDiscoveryConfig(t, "v1")

		t.Log("Ready to new discovery client")
		dc, err := gosdk.NewDiscoveryClient(dConfig)
		assert.Nil(t, err)

		t.Logf("Ready to discovery channel %s peers", channelName)
		peers, err := dc.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 1, len(peers))
		assert.Equal(t, "peer0.org1.example.com:7051", peers[0].Endpoint)
		assert.Equal(t, "Org1MSP", peers[0].MSPID)
		assert.Equal(t, uint64(2), peers[0].LedgerHeight)
		assert.Equal(t, []string{chaincodeName}, peers[0].Chaincodes)
		assert.NotEmpty(t, peers[0].Identity)

		t.Logf("Ready to discovery channel %s local peers", channelName)
		lPeers, err := dc.DiscoveryLocalPeers()
		assert.Nil(t, err, "discovery local peers failed")
		assert.Equal(t, 1, len(lPeers))
		assert.Equal(t, "peer0.org1.example.com:7051", lPeers[0].Endpoint)
		assert.Equal(t, "Org1MSP", lPeers[0].MSPID)
		assert.NotEmpty(t, lPeers[0].Identity)

		t.Logf("Entering Discovery channel %s config", channelName)
		cconfig, err := dc.DiscoveryChannelConfig(channelName)
		assert.Nil(t, err, "discovery channel config failed")
		assert.Equal(t, "OrdererMSP", cconfig.Msps["OrdererMSP"].Name)
		assert.Equal(t, "Org1MSP", cconfig.Msps["Org1MSP"].Name)
		assert.Equal(t, "Org2MSP", cconfig.Msps["Org2MSP"].Name)
		assert.Equal(t, &discovery.Endpoint{
			Host: "orderer.example.com",
			Port: 7050,
		}, cconfig.Orderers["OrdererMSP"].Endpoint[0])

		t.Log("Entering Discovery chaincodes endorse policy")
		p, err := dc.DiscoveryEndorsePolicy(channelName, []string{chaincodeName}, nil)
		assert.Nil(t, err, "discovery config failed")
		assert.Equal(t, 1, len(p))
		assert.Equal(t, chaincodeName, p[0].Chaincode)
		t.Logf("endorses by group is %v", p[0].EndorsersByGroups)
		assert.Equal(t, 1, len(p[0].EndorsersByGroups))
		for _, v := range p[0].EndorsersByGroups {
			assert.Equal(t, 1, len(v))
			assert.Equal(t, uint64(2), v[0].LedgerHeight)
		}
	})

	t.Run("test admin operation", func(t *testing.T) {
		org1Config := newOrg1AdminConfig(t)
		t.Log("Ready to new org1's admin client")

		ccs, err := gosdk.GetInstalledCCs(org1Config.cryptoC, org1Config.userC, org1Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		ccs, err = gosdk.GetInstantiatedCCs(org1Config.cryptoC, org1Config.userC, org1Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		t.Log("admin operate org2's fabric node")
		org2Config := newOrg2AdminConfig(t)

		t.Log("Ready to get genesis block for mychannel")
		genesisBlock, err := gosdk.GetOldestBlock(org2Config.cryptoC, org2Config.userC, channelName, org2Config.orderersC)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), genesisBlock.Header.Number)

		genesisBlockBytes, err := proto.Marshal(genesisBlock)
		assert.Nil(t, err)

		t.Log("Ready to join org2's peer to channel")
		err = gosdk.JoinChannel(org2Config.cryptoC, org2Config.userC, org2Config.peerC, genesisBlockBytes)
		assert.Nil(t, err, "org2's peer failed to join channel")

		t.Log("Ready to update org2's anchor peer")
		anchorEnv, err := ioutil.ReadFile(org2AnchorPath)
		assert.Nil(t, err, "read org2's anchor peer tx")
		err = gosdk.UpdateChannel(org2Config.cryptoC, org2Config.userC, org2Config.orderersC, channelName, anchorEnv)
		assert.Nil(t, err, "failed to update org2's anchor")
		time.Sleep(time.Second * 60)

		ccs, err = gosdk.GetInstalledCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(ccs))

		// TODO 需等待peer同步到所有的区块，获取到的已实例化合约数目即为1
		ccs, err = gosdk.GetInstantiatedCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))

		t.Log("Ready to install chaincode on org2's peer")
		ccPack, err := ioutil.ReadFile(ccoutPath)
		assert.Nil(t, err, "read cc.out failed")
		err = gosdk.InstallCCByPack(org2Config.cryptoC, org2Config.userC, org2Config.peerC, ccPack, gosdk.ChaincodeSpec_GOLANG)
		assert.Nil(t, err)

		ccs, err = gosdk.GetInstalledCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		// TODO 调研org2peer上的实例化数目
		ccs, err = gosdk.GetInstantiatedCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)
	})

	// 此时fabric网络已部署完成，通道为mychannel，合约名称为mycc，版本为1.0
	// 当前通道中有4个块：0-初始区块；1-更新org1锚节点；2-实例化合约；3-更新org2锚节点；

	t.Run("test fabric client", func(t *testing.T) {
		t.Log("Ready to add users")
		user := newUser(t, "v1")
		err = c1.AddUser("user2", user)
		assert.Nil(t, err)

		t.Log("Ready to query chaincode")
		queryRes, err := c1.Query([]string{"query", "a"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, queryRes)
		assert.Equal(t, "100", string(queryRes.Response.Payload))

		// 执行转账交易，会生成第四个块：4-txID对应的交易
		t.Log("Ready to invoke chaincode")
		invokeRes, err := c1.Invoke([]string{"invoke", "a", "b", "20"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, invokeRes)
		assert.Equal(t, common.Status_SUCCESS, invokeRes.Status)
		txID = invokeRes.TxID
		time.Sleep(time.Second * 5)

		t.Log("Ready to query chaincode again")
		queryRes, err = c1.Query([]string{"query", "a"}, nil, channelName, chaincodeName, "org1User")
		assert.Nil(t, err)
		assert.NotNil(t, queryRes)
		assert.Equal(t, "80", string(queryRes.Response.Payload))

		t.Log("Ready to get block height")
		height, err := c1.GetBlockHeight(channelName)
		assert.Nil(t, err)
		assert.Equal(t, uint64(5), height)

		t.Log("Ready to get block by number")
		block4, err := c1.GetBlockByNumber(channelName, 4)
		assert.Nil(t, err)
		assert.NotNil(t, block4)
		assert.Equal(t, uint64(4), block4.Header.Number)

		// 当前不存在第5个区块，返回错误
		t.Log("Ready to get block by wrong number")
		nonBlock, err := c1.GetBlockByNumber(channelName, 5)
		assert.NotNil(t, err)
		assert.Nil(t, nonBlock)

		t.Log("Ready to get block by txID")
		block, err := c1.GetBlockByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, uint64(4), block.Header.Number)

		t.Log("Ready to get last block")
		lastBlock, err := c1.GetNewestBlock(channelName)
		assert.Nil(t, err)
		assert.NotNil(t, lastBlock)
		assert.Equal(t, uint64(4), lastBlock.Header.Number)

		t.Log("Ready to get tx by txID")
		tx, err := c1.GetTransactionByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("test ledger client ", func(t *testing.T) {
		lConfig := newLedgerConfig(t, "v1")

		t.Log("Ready to new ledger client")
		lc, err := gosdk.NewLedgerClient(lConfig)
		assert.Nil(t, err)

		t.Log("Ready to get block height")
		height, err := lc.GetBlockHeight(channelName)
		assert.Nil(t, err)
		assert.Equal(t, uint64(5), height)

		t.Log("Ready to get block by number")
		block4, err := lc.GetBlockByNumber(channelName, 4)
		assert.Nil(t, err)
		assert.NotNil(t, block4)
		assert.Equal(t, uint64(4), block4.Header.Number)

		// 当前不存在第5个区块，返回错误
		t.Log("Ready to get block by wrong number")
		nonBlock, err := lc.GetBlockByNumber(channelName, 5)
		assert.NotNil(t, err)
		assert.Nil(t, nonBlock)

		t.Log("Ready to get block by txID")
		block, err := lc.GetBlockByTxID(channelName, txID)
		assert.Nil(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, uint64(4), block.Header.Number)

		t.Log("Ready to get last block")
		lastBlock, err := lc.GetNewestBlock(channelName)
		assert.Nil(t, err)
		assert.NotNil(t, lastBlock)
		assert.Equal(t, uint64(4), lastBlock.Header.Number)

		t.Log("Ready to get tx by txID")
		tx, err := lc.GetTransactionByTxID(channelName, txID)
		assert.Nil(t, err)
		assert.NotNil(t, tx)

		err = lc.Close()
		assert.Nil(t, err, "failed to close ledger client")
	})

	t.Run("test chaincode client", func(t *testing.T) {
		cConfig := newChaincodeClinetConfig(t, "v1")

		t.Log("Ready to new chaincode client")
		cc, err := gosdk.NewChaincodeClient(cConfig)
		assert.Nil(t, err)

		t.Log("Ready to add users")
		user := newUser(t, "v1")
		err = cc.AddUser("user2", user)
		assert.Nil(t, err)

		t.Log("Ready to query chaincode again")
		queryRes, err := cc.Query([]string{"query", "a"}, nil, "mychannel", "mycc", "user2")
		assert.Nil(t, err)
		assert.Equal(t, "80", string(queryRes.Response.Payload))

		// 执行转账交易，会生成区块5
		t.Log("Ready to invoke chaincode")
		invokeRes, err := cc.Invoke([]string{"invoke", "a", "b", "20"}, nil, "mychannel", "mycc", "user2")
		assert.Nil(t, err)
		assert.NotNil(t, invokeRes)
		assert.Equal(t, common.Status_SUCCESS, invokeRes.Status)
		time.Sleep(time.Second * 5)

		t.Log("Ready to query chaincode again")
		queryRes, err = cc.Query([]string{"query", "a"}, nil, "mychannel", "mycc", "user")
		assert.Nil(t, err)
		assert.Equal(t, "60", string(queryRes.Response.Payload))

		// 执行同步交易转账, 成功区块号是6
		t.Log("Ready to sync invoke chaincode ")
		syncInvokeRes, err := cc.SyncInvoke([]string{"invoke", "a", "b", "20"}, nil, "mychannel", "mycc", "user2")
		assert.Nil(t, err)
		assert.NotNil(t, syncInvokeRes)
		assert.Equal(t, common.Status_SUCCESS, invokeRes.Status)
		t.Log("Ready to query chaincode again")
		queryRes, err = cc.Query([]string{"query", "a"}, nil, "mychannel", "mycc", "user")
		assert.Nil(t, err)
		assert.Equal(t, "40", string(queryRes.Response.Payload))
		assert.Equal(t, uint64(6), syncInvokeRes.BlockNumber)

		cConfig.OConfigs = nil
		t.Log("Ready to new chaincode client just for peer")
		ccPeer, err := gosdk.NewChaincodeClient(cConfig)
		assert.Nil(t, err)

		t.Log("Ready to query chaincode by ccPeer again")
		queryRes, err = ccPeer.Query([]string{"query", "a"}, nil, "mychannel", "mycc", "user")
		assert.Nil(t, err)
		assert.Equal(t, "40", string(queryRes.Response.Payload))

		t.Log("Ready to invoke chaincode by ccPeer again")
		invokeRes, err = ccPeer.Invoke([]string{"invoke", "a", "b", "20"}, nil, "mychannel", "mycc", "user")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "orderer is not configured, can not execute invoke operation")
		assert.Nil(t, invokeRes)

		err = cc.Close()
		assert.Nil(t, err, "failed to close chaincode client")
	})

	t.Run("test event client", func(t *testing.T) {
		eConfig := newEventConfig(t, "v1")

		t.Log("Ready to new event client")
		ec, err := gosdk.NewEventClient(eConfig)
		assert.Nil(t, err)
		cb := make(chan pBlock.Block)
		fullErr := ec.ListenEventFullBlock(0, cb)
		assert.Nil(t, err)
		assert.NotNil(t, cb)
		fcb := make(chan gosdk.FilteredBlockResponse)
		filterErr := ec.ListenEventFilterBlock(0, fcb)
		assert.Nil(t, err)
		assert.NotNil(t, fcb)
		blocksNum, err := recBlock(t, cb, nil, fullErr)
		assert.Nil(t, err)
		assert.Equal(t, 3, blocksNum)
		ec.CloseFullBlockListen()
		blocksNum, err = recBlock(t, nil, fcb, filterErr)
		assert.Nil(t, err)
		assert.Equal(t, 3, blocksNum)
		ec.CloseFilteredBlockListen()
		time.Sleep(time.Second * 3)
		ec.Disconnect()
	})
}

func recBlock(t *testing.T, cb chan pBlock.Block, fcb chan gosdk.FilteredBlockResponse, errCh chan error) (int, error) {
	n := 0

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer func() {
		cancel()
	}()

	for {
		select {
		case block := <-cb:
			if block.Error != nil {
				t.Errorf("listen wrong block: %s", block.Error.Error())
				continue
			}

			n++

			if n == 3 {
				// 没有区块时event会进行阻塞，无法正常关闭event，所以在新区块接受之前关闭event
				time.Sleep(time.Second * 3)
				return n, nil
			}
		case block := <-fcb:
			if block.Error != nil {
				t.Errorf("listen wrong block: %s", block.Error.Error())
				continue
			}

			n++

			if n == 3 {
				// 没有区块时event会进行阻塞，无法正常关闭event，所以在新区块接受之前关闭event
				time.Sleep(time.Second * 3)
				return n, nil
			}
		case err := <-errCh:
			return -1, err
		case <-ctx.Done():
			t.Log("reach the dead time")
			return -1, errors.New("reach the dead time")
		}
	}
}
