package e2e

import (
	"io/ioutil"
	"testing"
	"time"

	gohfc "github.com/hw09234/gohfc/pkg"
	pBlock "github.com/hw09234/gohfc/pkg/parseBlock"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/stretchr/testify/assert"
)

const (
	gmChannelTXPath  = "../../fixtures/gm-channel-artifacts/channel.tx"
	org1AnchorGMPath = "../../fixtures/gm-channel-artifacts/Org1MSPanchors.tx"
	org2AnchorGMPath = "../../fixtures/gm-channel-artifacts/Org2MSPanchors.tx"

	ordererGMTLSPath  = "../../fixtures/gm-crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt"
	org1PeerGMTLSPath = "../../fixtures/gm-crypto-config/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt"
	org2PeerGMTLSPath = "../../fixtures/gm-crypto-config/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/server.crt"
	org1AdminGMPath   = "../../fixtures/gm-crypto-config/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp"
	org2AdminGMPath   = "../../fixtures/gm-crypto-config/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp"
)

var gmCryptoConfig = gohfc.CryptoConfig{
	Family:    "gm",
	Algorithm: "P256SM2",
	Hash:      "SM3",
}

func TestE2EGM(t *testing.T) {
	t.Log("Ready to new fabric gm client")
	clientConfig := newOrg1ClientConfigGM(t, "v1")
	c, err := gohfc.NewFabricClient(&clientConfig, "")
	assert.Nil(t, err, "new fabric client failed")

	t.Run("deploy Fabric network and operate org1's node by fabric client", func(t *testing.T) {
		// 等待fabric容器启动，该fabric client只针对org1进行配置，所以只有org1的peer加入通道、安装cc
		// org2的peer未执行此操作
		time.Sleep(time.Second * 10)

		// 创建通道
		t.Log("Ready to create channel")
		err = c.CreateUpdateChannel(channelName, gmChannelTXPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 加入通道
		t.Log("Ready to join channel for peers")
		res, err := c.JoinChannel(channelName, "peer01")
		assert.Nil(t, err, "org1's peer failed to join channel")
		assert.NotNil(t, res)
		time.Sleep(time.Second * 3)

		// 更新锚节点
		t.Log("Ready to update org1's anchor peer")
		err = c.UpdateAnchorPeer(channelName, org1AnchorGMPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 安装chaincode
		t.Log("Ready to install chaincode on org1's peer")
		installRequest := &gohfc.InstallRequest{
			ChannelId:        channelName,
			ChainCodeName:    chaincodeName,
			ChainCodeVersion: "1.0",
			ChainCodeType:    gohfc.ChaincodeSpec_GOLANG,
			Namespace:        "github.com/PeerFintech/gohfc/test/fixtures/chaincode",
			SrcPath:          chaincodePath,
		}
		res, err = c.InstallChainCode("peer01", installRequest)
		assert.Nil(t, err, "failed to install chaincode for org1's peer")
		time.Sleep(time.Second * 5)

		// 实例化chaincode
		// 未指定policy策略，默认为
		t.Log("Ready to instantiate chaincode")
		instantiateRequest := &gohfc.ChainCode{
			ChannelId: channelName,
			Name:      chaincodeName,
			Version:   "1.0",
			Type:      gohfc.ChaincodeSpec_GOLANG,
			Args:      []string{"init", "a", "100", "b", "200"},
		}
		instantiateRes, err := c.InstantiateChainCode(policy, instantiateRequest)
		assert.Nil(t, err, "InstantiateChainCode failed")
		assert.NotNil(t, instantiateRes)
		assert.Equal(t, common.Status_SUCCESS, instantiateRes.Status)

		// 等待生成合约容器，此时fabric网络已完全创建，区块数目为4
		time.Sleep(time.Second * 40)

		t.Logf("Ready to discovery channel %s peers by fabric client", channelName)
		peers, err := c.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 1, len(peers))
		assert.Equal(t, "peer0.org1.example.com:7051", peers[0].Endpoint)
		assert.Equal(t, "Org1MSP", peers[0].MSPID)
		assert.Equal(t, uint64(3), peers[0].LedgerHeight)
		assert.Equal(t, []string{chaincodeName}, peers[0].Chaincodes)
		assert.NotEmpty(t, peers[0].Identity)

		t.Log("Ready to discovery local peers by fabric client")
		lPeers, err := c.DiscoveryLocalPeers()
		assert.Nil(t, err, "discovery local peers failed")
		assert.Equal(t, 1, len(lPeers))
		assert.Equal(t, "peer0.org1.example.com:7051", lPeers[0].Endpoint)
		assert.Equal(t, "Org1MSP", lPeers[0].MSPID)
		assert.NotEmpty(t, lPeers[0].Identity)

		t.Logf("Entering Discovery channel %s config by fabric client", channelName)
		cconfig, err := c.DiscoveryChannelConfig(channelName)
		assert.Nil(t, err, "discovery channel config failed")
		assert.Equal(t, "OrdererMSP", cconfig.Msps["OrdererMSP"].Name)
		assert.Equal(t, "Org1MSP", cconfig.Msps["Org1MSP"].Name)
		assert.Equal(t, "Org2MSP", cconfig.Msps["Org2MSP"].Name)
		assert.Equal(t, &discovery.Endpoint{
			Host: "orderer.example.com",
			Port: 7050,
		}, cconfig.Orderers["OrdererMSP"].Endpoint[0])

		t.Log("Entering Discovery chaincodes endorse policy by fabric client")
		p, err := c.DiscoveryEndorsePolicy(channelName, []string{chaincodeName}, nil)
		assert.Nil(t, err, "discovery config failed")
		assert.Equal(t, 1, len(p))
		assert.Equal(t, chaincodeName, p[0].Chaincode)
		t.Logf("endorses by group is %v", p[0].EndorsersByGroups)
		assert.Equal(t, 1, len(p[0].EndorsersByGroups))
		for _, v := range p[0].EndorsersByGroups {
			assert.Equal(t, 1, len(v))
			assert.Equal(t, uint64(3), v[0].LedgerHeight)
		}
	})

	t.Run("test discovery client", func(t *testing.T) {
		dConfig := newDiscoveryConfigGM(t, "v1")

		t.Log("Ready to new discovery client")
		dc, err := gohfc.NewDiscoveryClient(dConfig)
		assert.Nil(t, err)

		t.Logf("Ready to discovery channel %s peers", channelName)
		peers, err := dc.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 1, len(peers))
		assert.Equal(t, "peer0.org1.example.com:7051", peers[0].Endpoint)
		assert.Equal(t, "Org1MSP", peers[0].MSPID)
		assert.Equal(t, uint64(3), peers[0].LedgerHeight)
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
			assert.Equal(t, uint64(3), v[0].LedgerHeight)
		}
	})

	t.Run("test admin operation", func(t *testing.T) {
		org1Config := newOrg1AdminConfigGM(t)
		t.Log("Ready to new org1's admin client")

		ccs, err := gohfc.GetInstalledCCs(org1Config.cryptoC, org1Config.userC, org1Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		ccs, err = gohfc.GetInstantiatedCCs(org1Config.cryptoC, org1Config.userC, org1Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		t.Log("admin operate org2's fabric node")
		org2Config := newOrg2AdminConfigGM(t)

		t.Log("Ready to get genesis block for mychannel")
		genesisBlock, err := gohfc.GetOldestBlock(org2Config.cryptoC, org2Config.userC, channelName, org2Config.orderersC)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), genesisBlock.Header.Number)

		genesisBlockBytes, err := proto.Marshal(genesisBlock)
		assert.Nil(t, err)

		t.Log("Ready to join org2's peer to channel")
		err = gohfc.JoinChannel(org2Config.cryptoC, org2Config.userC, org2Config.peerC, genesisBlockBytes)
		assert.Nil(t, err, "org2's peer failed to join channel")

		t.Log("Ready to update org2's anchor peer")
		anchorEnv, err := ioutil.ReadFile(org2AnchorGMPath)
		assert.Nil(t, err, "read org2's anchor peer tx")
		err = gohfc.UpdateChannel(org2Config.cryptoC, org2Config.userC, org2Config.orderersC, channelName, anchorEnv)
		assert.Nil(t, err, "failed to update org2's anchor")
		time.Sleep(time.Second * 60)

		ccs, err = gohfc.GetInstalledCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(ccs))

		// TODO 需等待peer同步到所有的区块，获取到的已实例化合约数目即为1
		ccs, err = gohfc.GetInstantiatedCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))

		t.Log("Ready to install chaincode on org2's peer")
		ccPack, err := ioutil.ReadFile(ccoutPath)
		assert.Nil(t, err, "read cc.out failed")
		err = gohfc.InstallCCByPack(org2Config.cryptoC, org2Config.userC, org2Config.peerC, ccPack, gohfc.ChaincodeSpec_GOLANG)
		assert.Nil(t, err)

		ccs, err = gohfc.GetInstalledCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)

		ccs, err = gohfc.GetInstantiatedCCs(org2Config.cryptoC, org2Config.userC, org2Config.peerC, channelName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(ccs))
		assert.Equal(t, chaincodeName, ccs[0].Name)
	})

	// 此时fabric网络已部署完成，通道为mychannel，合约名称为mycc，版本为1.0
	// 当前通道中有4个块：0-初始区块；1-更新org1锚节点；2-实例化合约；3-更新org2锚节点；

	t.Run("test fabric client", func(t *testing.T) {
		t.Log("Ready to add users")
		user := newUserGM(t, "v1")
		err = c.AddUser("user2", user)
		assert.Nil(t, err)

		t.Log("Ready to query chaincode")
		queryRes, err := c.Query([]string{"query", "a"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, queryRes)
		assert.Equal(t, "100", string(queryRes.Response.Payload))

		// 执行转账交易，会生成第四个块：4-txID对应的交易
		t.Log("Ready to invoke chaincode")
		invokeRes, err := c.Invoke([]string{"invoke", "a", "b", "20"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, invokeRes)
		assert.Equal(t, common.Status_SUCCESS, invokeRes.Status)
		txID = invokeRes.TxID
		time.Sleep(time.Second * 5)

		t.Log("Ready to query chaincode again")
		queryRes, err = c.Query([]string{"query", "a"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, queryRes)
		assert.Equal(t, "80", string(queryRes.Response.Payload))

		t.Log("Ready to get block height")
		height, err := c.GetBlockHeight(channelName)
		assert.Nil(t, err)
		assert.Equal(t, uint64(5), height)

		t.Log("Ready to get block by number")
		block4, err := c.GetBlockByNumber(channelName, 4)
		assert.Nil(t, err)
		assert.NotNil(t, block4)
		assert.Equal(t, uint64(4), block4.Header.Number)

		// 当前不存在第5个区块，返回错误
		t.Log("Ready to get block by wrong number")
		nonBlock, err := c.GetBlockByNumber(channelName, 5)
		assert.NotNil(t, err)
		assert.Nil(t, nonBlock)

		t.Log("Ready to get block by txID")
		block, err := c.GetBlockByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, uint64(4), block.Header.Number)

		t.Log("Ready to get last block")
		lastBlock, err := c.GetNewestBlock(channelName)
		assert.Nil(t, err)
		assert.NotNil(t, lastBlock)
		assert.Equal(t, uint64(4), lastBlock.Header.Number)

		t.Log("Ready to get tx by txID")
		tx, err := c.GetTransactionByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("test ledger client ", func(t *testing.T) {
		lConfig := newLedgerConfigGM(t, "v1")

		t.Log("Ready to new ledger client")
		lc, err := gohfc.NewLedgerClient(lConfig)
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
		cConfig := newChaincodeClinetConfigGM(t, "v1")

		t.Log("Ready to new chaincode client")
		cc, err := gohfc.NewChaincodeClient(cConfig)
		assert.Nil(t, err)

		t.Log("Ready to add users")
		user := newUserGM(t, "v1")
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
		ccPeer, err := gohfc.NewChaincodeClient(cConfig)
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
		eConfig := newEventConfigGM(t, "v1")

		t.Log("Ready to new event client")
		ec, err := gohfc.NewEventClient(eConfig)
		assert.Nil(t, err)
		cb := make(chan pBlock.Block)
		fullErr := ec.ListenEventFullBlock(0, cb)
		assert.Nil(t, err)
		assert.NotNil(t, cb)
		fcb := make(chan gohfc.FilteredBlockResponse)
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
