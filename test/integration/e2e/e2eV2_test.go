package e2e

import (
	"io/ioutil"
	"testing"
	"time"

	gosdk "github.com/hw09234/gosdk/pkg"
	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/stretchr/testify/assert"
)

// TestE2E 使用转账业务进行集成测试
// 在测试过程中，对于需要写块的操作，调用返回后需要执行sleep以等待交易落块
func TestV2E2ENonGM(t *testing.T) {
	t.Log("Ready to new fabric client")
	org1ClientConfig := newOrg1ClientConfig(t, "v2")
	c1, err := gosdk.NewFabricClient(&org1ClientConfig, "")
	assert.Nil(t, err, "new org1 client failed")

	org2ClientConfig := newOrg2ClientConfig(t, "v2")
	c2, err := gosdk.NewFabricClient(&org2ClientConfig, "")
	assert.Nil(t, err, "new org2 client failed")

	t.Run("deploy Fabric network and operate org1's node by fabric client", func(t *testing.T) {
		// 等待fabric容器启动，该fabric client只针对org1进行配置，所以只有org1的peer加入通道、安装cc
		// org2的peer未执行此操作
		time.Sleep(time.Second * 10)

		// 打包chaincode
		t.Log("Ready to package chaincode")
		info := gosdk.ChaincodeInfo{
			OutputFile: "mycc.tar.gz",
			Path:       chaincodePathV2,
			Type:       gosdk.ChaincodeSpec_GOLANG,
			Label:      "myccv1",
		}
		err = c1.PackageChaincode(info)
		assert.Nil(t, err)

		// 创建通道
		t.Log("Ready to create channel")
		err = c1.CreateUpdateChannel(channelName, channelTXPathV2)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 加入通道
		t.Log("Peer01 ready to join channel for peers")
		res, err := c1.JoinChannel(channelName, "peer01")
		assert.Nil(t, err, "org1's peer failed to join channel")
		assert.NotNil(t, res)
		time.Sleep(time.Second * 3)

		t.Log("Peer02 ready to join channel for peers")
		res, err = c2.JoinChannel(channelName, "peer02")
		assert.Nil(t, err, "org2's peer failed to join channel")
		assert.NotNil(t, res)
		time.Sleep(time.Second * 3)

		// 更新锚节点
		t.Log("Ready to update org1's anchor peer")
		err = c1.UpdateAnchorPeer(channelName, org1AnchorPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		t.Log("Ready to update org2's anchor peer")
		err = c2.UpdateAnchorPeer(channelName, org2AnchorPath)
		assert.Nil(t, err)
		time.Sleep(time.Second * 5)

		// 安装chaincode
		t.Log("Ready to install chaincode on org1's peer")
		pkg, err := ioutil.ReadFile("mycc.tar.gz")
		assert.Nil(t, err)
		installResult, err := c1.InstallChainCodeV2(pkg, "peer01")
		assert.Nil(t, err)
		assert.NotNil(t, installResult)
		assert.Equal(t, "myccv1", installResult.Label)
		time.Sleep(time.Second * 5)

		t.Log("Ready to install chaincode on org2's peer")
		installResult, err = c2.InstallChainCodeV2(pkg, "peer02")
		assert.Nil(t, err)
		assert.NotNil(t, installResult)
		assert.Equal(t, "myccv1", installResult.Label)
		time.Sleep(time.Second * 5)

		t.Log("Query peer01 chaincode installed")
		results, err := c1.QueryInstalledV2("peer01")
		assert.Nil(t, err)
		assert.Equal(t, "myccv1", results[0].Label)

		// 组织批准
		t.Log("Ready to approve chaincode")
		acreq := gosdk.ApproveCommitRequest{
			ChannelName:         channelName,
			ChaincodeName:       chaincodeName,
			ChaincodeVserison:   "1.0",
			PackageID:           installResult.PackageId,
			SignaturePolicy:     policy,
			ChannelConfigPolicy: "",
			OrgName:             "org1",
			Sequence:            1,
			InitReqired:         true,
		}
		ordRes, err := c1.ApproveForMyOrg(acreq)
		assert.Nil(t, err)
		assert.Equal(t, common.Status_SUCCESS, ordRes.Status)
		time.Sleep(time.Second * 5)

		block, err := c1.GetNewestBlock(channelName)
		assert.Nil(t, err)
		appChaincode, err := pBlock.GetApproveChaincodeInfo(block)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), appChaincode.Sequence)
		assert.Equal(t, "mycc", appChaincode.Name)

		acreq = gosdk.ApproveCommitRequest{
			ChannelName:         channelName,
			ChaincodeName:       chaincodeName,
			ChaincodeVserison:   "1.0",
			PackageID:           installResult.PackageId,
			SignaturePolicy:     policy,
			ChannelConfigPolicy: "",
			OrgName:             "org2",
			Sequence:            1,
			InitReqired:         true,
		}
		ordRes, err = c2.ApproveForMyOrg(acreq)
		assert.Nil(t, err)
		assert.Equal(t, common.Status_SUCCESS, ordRes.Status)
		time.Sleep(time.Second * 5)

		t.Log("Query org1 chaincode approved")
		acc, err := c1.QueryApproved(channelName, chaincodeName, "org1", 1)
		assert.Nil(t, err)
		assert.Equal(t, "1.0", acc.Version)
		assert.Equal(t, int64(1), acc.Sequence)
		assert.Equal(t, true, acc.InitRequired)

		t.Log("Query org2 chaincode approved")
		acc, err = c2.QueryApproved(channelName, chaincodeName, "org2", 1)
		assert.Nil(t, err)
		assert.Equal(t, "1.0", acc.Version)
		assert.Equal(t, int64(1), acc.Sequence)
		assert.Equal(t, true, acc.InitRequired)

		t.Log("Check org's commitreadiness")
		req := gosdk.CheckCommitreadinessRequest{
			ChannelName:       channelName,
			ChaincodeName:     chaincodeName,
			ChaincodeVserison: "1.0",
			SignaturePolicy:   policy,
			InitRequired:      true,
			Sequence:          1,
		}
		checkResult, err := c1.CheckCommitreadiness(req)
		assert.Nil(t, err)
		for _, flag := range checkResult {
			assert.Equal(t, true, flag)
		}

		// 进行提交
		t.Log("Ready to commit chaincode")
		acreq = gosdk.ApproveCommitRequest{
			ChannelName:       channelName,
			ChaincodeName:     chaincodeName,
			ChaincodeVserison: "1.0",
			SignaturePolicy:   policy,
			Sequence:          1,
			InitReqired:       true,
		}
		ordRes, err = c1.LifecycleCommit(acreq)
		assert.Nil(t, err)
		assert.Equal(t, common.Status_SUCCESS, ordRes.Status)
		time.Sleep(time.Second * 5)

		block, err = c1.GetNewestBlock(channelName)
		assert.Nil(t, err)
		commChaincode, err := pBlock.GetCommitChaincodeInfo(block)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), commChaincode.Sequence)
		assert.Equal(t, "mycc", commChaincode.Name)

		t.Log("Ready to query committed chaincode")
		input := gosdk.CommittedQueryInput{
			ChannelID: channelName,
		}
		_, queryResult, err := c1.QueryCommitted(input)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(queryResult.ChaincodeDefinitions))
		assert.Equal(t, chaincodeName, queryResult.ChaincodeDefinitions[0].Name)

		// 等待生成合约容器，此时fabric网络已完全创建，区块数目为4
		time.Sleep(time.Second * 40)

		t.Logf("Ready to discovery channel %s peers by fabric client", channelName)
		peers, err := c1.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 2, len(peers))
		assert.Equal(t, uint64(6), peers[0].LedgerHeight)
		assert.Equal(t, []string{chaincodeName, lifecycleName}, peers[0].Chaincodes)
		assert.NotEmpty(t, peers[0].Identity)

		t.Log("Ready to discovery local peers by fabric client")
		lPeers, err := c1.DiscoveryLocalPeers()
		assert.Nil(t, err, "discovery local peers failed")
		assert.Equal(t, 2, len(lPeers))
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
		assert.Equal(t, 2, len(p[0].EndorsersByGroups))
		for _, v := range p[0].EndorsersByGroups {
			assert.Equal(t, 1, len(v))
			assert.Equal(t, uint64(6), v[0].LedgerHeight)
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
		dConfig := newDiscoveryConfig(t, "v2")

		t.Log("Ready to new discovery client")
		dc, err := gosdk.NewDiscoveryClient(dConfig)
		assert.Nil(t, err)

		t.Logf("Ready to discovery channel %s peers", channelName)
		peers, err := dc.DiscoveryChannelPeers(channelName)
		assert.Nil(t, err, "discovery channel peers failed")
		assert.Equal(t, 2, len(peers))
		assert.Equal(t, uint64(6), peers[0].LedgerHeight)
		assert.Equal(t, []string{chaincodeName, "_lifecycle"}, peers[0].Chaincodes)
		assert.NotEmpty(t, peers[0].Identity)

		t.Logf("Ready to discovery channel %s local peers", channelName)
		lPeers, err := dc.DiscoveryLocalPeers()
		assert.Nil(t, err, "discovery local peers failed")
		assert.Equal(t, 2, len(lPeers))
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
		assert.Equal(t, 2, len(p[0].EndorsersByGroups))
		for _, v := range p[0].EndorsersByGroups {
			assert.Equal(t, 1, len(v))
			assert.Equal(t, uint64(6), v[0].LedgerHeight)
		}
	})

	// 此时fabric网络已部署完成，通道为mychannel，合约名称为mycc，版本为1.0
	// 当前通道中有4个块：0-初始区块；1-更新org1锚节点；2-实例化合约；3-更新org2锚节点；

	t.Run("test fabric client", func(t *testing.T) {
		t.Log("Ready to add users")
		user := newUser(t, "v2")
		err = c1.AddUser("user2", user)
		assert.Nil(t, err)

		t.Log("Ready to init chaincode")
		invokeRes, err := c1.Init([]string{"Init", "a", "100", "b", "100"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, invokeRes)
		assert.Equal(t, common.Status_SUCCESS, invokeRes.Status)
		txID = invokeRes.TxID
		time.Sleep(time.Second * 5)

		t.Log("Ready to query chaincode")
		queryRes, err := c1.Query([]string{"query", "a"}, nil, channelName, chaincodeName, "user2")
		assert.Nil(t, err)
		assert.NotNil(t, queryRes)
		assert.Equal(t, "100", string(queryRes.Response.Payload))

		// 执行转账交易，会生成第四个块：4-txID对应的交易
		t.Log("Ready to invoke chaincode")
		invokeRes, err = c1.Invoke([]string{"invoke", "a", "b", "20"}, nil, channelName, chaincodeName, "user2")
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
		assert.Equal(t, uint64(8), height)

		t.Log("Ready to get block by number")
		block4, err := c1.GetBlockByNumber(channelName, 4)
		assert.Nil(t, err)
		assert.NotNil(t, block4)
		assert.Equal(t, uint64(4), block4.Header.Number)

		// 当前不存在第13个区块，返回错误
		t.Log("Ready to get block by wrong number")
		nonBlock, err := c1.GetBlockByNumber(channelName, 13)
		assert.NotNil(t, err)
		assert.Nil(t, nonBlock)

		t.Log("Ready to get block by txID")
		block, err := c1.GetBlockByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, uint64(7), block.Header.Number)

		t.Log("Ready to get last block")
		lastBlock, err := c1.GetNewestBlock(channelName)
		assert.Nil(t, err)
		assert.NotNil(t, lastBlock)
		assert.Equal(t, uint64(7), lastBlock.Header.Number)

		t.Log("Ready to get tx by txID")
		tx, err := c1.GetTransactionByTxID(channelName, invokeRes.TxID)
		assert.Nil(t, err)
		assert.NotNil(t, tx)
	})

	t.Run("test ledger client ", func(t *testing.T) {
		lConfig := newLedgerConfig(t, "v2")

		t.Log("Ready to new ledger client")
		lc, err := gosdk.NewLedgerClient(lConfig)
		assert.Nil(t, err)

		t.Log("Ready to get block height")
		height, err := lc.GetBlockHeight(channelName)
		assert.Nil(t, err)
		assert.Equal(t, uint64(8), height)

		t.Log("Ready to get block by number")
		block4, err := lc.GetBlockByNumber(channelName, 4)
		assert.Nil(t, err)
		assert.NotNil(t, block4)
		assert.Equal(t, uint64(4), block4.Header.Number)

		// 当前不存在第13个区块，返回错误
		t.Log("Ready to get block by wrong number")
		nonBlock, err := lc.GetBlockByNumber(channelName, 13)
		assert.NotNil(t, err)
		assert.Nil(t, nonBlock)

		t.Log("Ready to get block by txID")
		block, err := lc.GetBlockByTxID(channelName, txID)
		assert.Nil(t, err)
		assert.NotNil(t, block)
		assert.Equal(t, uint64(7), block.Header.Number)

		t.Log("Ready to get last block")
		lastBlock, err := lc.GetNewestBlock(channelName)
		assert.Nil(t, err)
		assert.NotNil(t, lastBlock)
		assert.Equal(t, uint64(7), lastBlock.Header.Number)

		t.Log("Ready to get tx by txID")
		tx, err := lc.GetTransactionByTxID(channelName, txID)
		assert.Nil(t, err)
		assert.NotNil(t, tx)

		err = lc.Close()
		assert.Nil(t, err, "failed to close ledger client")
	})

	t.Run("test chaincode client", func(t *testing.T) {
		cConfig := newChaincodeClinetConfig(t, "v2")

		t.Log("Ready to new chaincode client")
		cc, err := gosdk.NewChaincodeClient(cConfig)
		assert.Nil(t, err)

		t.Log("Ready to add users")
		user := newUser(t, "v2")
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
		assert.Equal(t, uint64(9), syncInvokeRes.BlockNumber)

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
		eConfig := newEventConfig(t, "v2")

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
