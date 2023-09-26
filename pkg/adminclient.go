package gosdk

import (
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/bccsp/sw"
	"github.com/hyperledger/fabric/core/common/ccprovider"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// GetInstalledCCs 获取指定peer上已经安装的chaincode
func GetInstalledCCs(cryptoC CryptoConfig, userC UserConfig, peerC PeerConfig) ([]ChainCode, error) {
	var ccs []ChainCode

	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	p, err := newPeer(peerC, crypto)
	if err != nil {
		return nil, errors.WithMessage(err, "new peer failed")
	}
	defer p.conn.Close()

	chaincode := ChainCode{
		Type: ChaincodeSpec_GOLANG,
		Name: LSCC,
		Args: []string{"getinstalledchaincodes"},
	}
	_, signedProp, err := createSignedProposal(*ide, chaincode, crypto)
	if err != nil {
		return nil, errors.Errorf("failed to create proposal: %s", err.Error())
	}

	resp, err := p.Endorse(signedProp, other)
	if err != nil {
		return nil, errors.WithMessage(err, "peer endorse proposal failed")
	}

	cqr := &peer.ChaincodeQueryResponse{}
	if err = proto.Unmarshal(resp.Response.Payload, cqr); err != nil {
		return nil, errors.Errorf("unmarshal payload failed: %s", err.Error())
	}
	for _, cc := range cqr.Chaincodes {
		ccs = append(ccs, ChainCode{
			Name:    cc.Name,
			Version: cc.Version,
		})
	}

	return ccs, nil
}

// GetInstantiatedCCs 获取指定peer上对应通道中已实例化的chaincode
func GetInstantiatedCCs(cryptoC CryptoConfig, userC UserConfig, peerC PeerConfig, channel string) ([]ChainCode, error) {
	var ccs []ChainCode

	if channel == "" {
		return nil, errors.New("channel is nil")
	}

	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	p, err := newPeer(peerC, crypto)
	if err != nil {
		return nil, errors.WithMessage(err, "new peer failed")
	}
	defer p.conn.Close()

	chaincode := ChainCode{
		ChannelId: channel,
		Type:      ChaincodeSpec_GOLANG,
		Name:      LSCC,
		Args:      []string{"getchaincodes"},
	}
	_, signedProp, err := createSignedProposal(*ide, chaincode, crypto)
	if err != nil {
		return nil, errors.Errorf("failed to create proposal: %s", err.Error())
	}

	resp, err := p.Endorse(signedProp, other)
	if err != nil {
		return nil, errors.WithMessage(err, "peer endorse proposal failed")
	}

	cqr := &peer.ChaincodeQueryResponse{}
	if err = proto.Unmarshal(resp.Response.Payload, cqr); err != nil {
		return nil, errors.Errorf("unmarshal payload failed: %s", err.Error())
	}
	for _, cc := range cqr.Chaincodes {
		ccs = append(ccs, ChainCode{
			ChannelId: channel,
			Name:      cc.Name,
			Version:   cc.Version,
		})
	}

	return ccs, nil
}

// InstallCCByPack 在对应peer上安装已打包类型的chaincode包
func InstallCCByPack(cryptoC CryptoConfig, userC UserConfig, peerC PeerConfig, b []byte, codeType ChainCodeType) error {
	logger.Debug("enter install chaincode by package progress")
	defer logger.Debug("exit install chaincode by package progress")

	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return errors.WithMessage(err, "create Identity failed")
	}

	cryptoProvider, err := sw.NewDefaultSecurityLevelWithKeystore(sw.NewDummyKeyStore())
	if err != nil {
		return errors.WithMessage(err, "new default crypto provider failed")
	}
	ccpack, err := ccprovider.GetCCPackage(b, cryptoProvider)
	if err != nil {
		return errors.WithMessage(err, "get CDSPackage failed")
	}

	//either CDS or Envelope
	o := ccpack.GetPackageObject()

	cds, ok := o.(*peer.ChaincodeDeploymentSpec)
	if !ok || cds == nil {
		return errors.New("invalid message for creating chaincode proposal")
	}
	depSpec, err := proto.Marshal(cds)
	if err != nil {
		return errors.WithMessage(err, "proto marshal failed")
	}

	chaincode := ChainCode{
		Type:     codeType,
		Name:     LSCC,
		Args:     []string{"install"},
		argBytes: depSpec,
	}
	_, signedProp, err := createSignedProposal(*ide, chaincode, crypto)

	p, err := newPeer(peerC, crypto)
	if err != nil {
		return errors.WithMessage(err, "new peer failed")
	}
	defer p.conn.Close()

	resp, err := p.Endorse(signedProp, other)
	if err != nil {
		return errors.WithMessage(err, "install chaincode failed")
	}
	if resp.Response.Status != int32(common.Status_SUCCESS) {
		return errors.Errorf("install chaincode failed with status %d and message is %s", resp.Response.Status, resp.Response.Message)
	}

	return nil
}

// GetOldestBlock 获取最早的区块
// 最早的区块即区块号为0的区块，为一配置块
func GetOldestBlock(cryptoC CryptoConfig, userC UserConfig, channelID string, orderersC []OrdererConfig) (*common.Block, error) {
	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	signedEnvp, err := createEnvpForOldestBlock(*ide, crypto, channelID)
	if err != nil {
		return nil, errors.WithMessage(err, "create envelope for getting oldest block")
	}
	for _, ordererC := range orderersC {
		orderer, err := newOrderer(ordererC, crypto)
		if err != nil {
			logger.Errorf("create orderer %s failed, try next orderer", ordererC.Host)
			continue
		}
		// 在函数退出时关闭tcp连接
		defer orderer.conn.Close()

		block, err := orderer.deliver(signedEnvp)
		if err != nil {
			logger.Errorf("Failed to get oldest config block from orderer %s: %s", orderer.uri, err.Error())
			continue
		}
		return block, nil
	}

	return nil, errors.New("failed to get oldest config block from all orderers")
}

// GetNewestBlock 获取最新的区块
func GetNewestBlock(cryptoC CryptoConfig, userC UserConfig, channelID string, orderersC []OrdererConfig) (*common.Block, error) {
	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	signedEnvp, err := createEnvpForNewestBlock(*ide, crypto, channelID)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create envelope for getting newest block")
	}
	for _, ordererC := range orderersC {
		orderer, err := newOrderer(ordererC, crypto)
		if err != nil {
			logger.Errorf("create orderer %s failed, try next orderer", ordererC.Host)
			continue
		}
		// 在函数退出时关闭tcp连接
		defer orderer.conn.Close()

		block, err := orderer.deliver(signedEnvp)
		if err != nil {
			logger.Errorf("Failed to get oldest config block from orderer %s: %s", orderer.uri, err.Error())
			continue
		}
		return block, nil
	}

	return nil, errors.New("failed to get oldest config block from all orderers")
}

// getBlockByNum 获取指定区块号的区块
func getBlockByNum(cryptoC CryptoConfig, userC UserConfig, channelID string, num uint64, orderersC []OrdererConfig) (*common.Block, error) {
	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return nil, errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return nil, errors.WithMessage(err, "create Identity failed")
	}

	signedEnvp, err := createEnvpForBlockNum(*ide, crypto, channelID, num)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create envelope for getting newest block")
	}
	for _, ordererC := range orderersC {
		orderer, err := newOrderer(ordererC, crypto)
		if err != nil {
			logger.Errorf("create orderer %s failed, try next orderer", ordererC.Host)
			continue
		}
		// 在函数退出时关闭tcp连接
		defer orderer.conn.Close()

		block, err := orderer.deliver(signedEnvp)
		if err != nil {
			logger.Errorf("Failed to get oldest config block from orderer %s: %s", orderer.uri, err.Error())
			continue
		}
		return block, nil
	}

	return nil, errors.New("failed to get oldest config block from all orderers")
}

// GetLatestConfigBlock 获取最新的配置区块
func GetNewestConfigBlock(cryptoC CryptoConfig, userC UserConfig, channelID string, orderersC []OrdererConfig) (*common.Block, error) {
	newestBlock, err := GetNewestBlock(cryptoC, userC, channelID, orderersC)
	if err != nil {
		return nil, errors.Errorf("failed to get newest block for channel %s: %s", channelID, err.Error())
	}
	configIndex, err := protoutil.GetLastConfigIndexFromBlock(newestBlock)
	if err != nil {
		return nil, errors.Errorf("failed to get config index: %s", err.Error())
	}

	return getBlockByNum(cryptoC, userC, channelID, configIndex, orderersC)
}

// UpdateChannel 更新通道操作，将envp结构发送给orderer节点
// 创建通道其实是更新通道操作的一种特例
// 入参中orderer是一个数组，函数内部自动处理orderer连接或发送失败的情况
func UpdateChannel(cryptoC CryptoConfig, userC UserConfig, orderersC []OrdererConfig, channelId string, env []byte) error {
	logger.Debug("Enter update channel progress")
	defer logger.Debug("Exit update channel progress")

	envp := new(common.Envelope)
	if err := proto.Unmarshal(env, envp); err != nil {
		return errors.WithMessage(err, "unmarshal byte to envelope failed")
	}

	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return errors.WithMessage(err, "create Identity failed")
	}

	signedEnvp, err := buildAndSignChannelConfig(*ide, envp.GetPayload(), crypto, channelId)
	if err != nil {
		return errors.WithMessage(err, "create signed envelope failed")
	}

	for _, ordererC := range orderersC {
		orderer, err := newOrderer(ordererC, crypto)
		if err != nil {
			logger.Errorf("create orderer %s failed, try next orderer", ordererC.Host)
			continue
		}
		// 在函数退出时关闭tcp连接
		defer orderer.conn.Close()

		_, err = orderer.broadcast(signedEnvp)
		if err == nil {
			logger.Debug("update channel successful")
			return nil
		} else if err == ErrResponse {
			logger.Error("send update channel envelope to orderer successful, but handle it failed")
			return err
		}
	}

	logger.Error("Send envelope to all orderer failed")
	return errors.New("failed to send envelope to all orderers, update channel failed")
}

// JoinChannel peer加入某个通道
// 针对单个peer执行执行加入通道的操作
func JoinChannel(cryptoC CryptoConfig, userC UserConfig, peerC PeerConfig, blockBytes []byte) error {
	logger.Debug("Enter join channel process")
	defer logger.Debug("Exit join channel process")

	crypto, err := newECCryptSuiteFromConfig(cryptoC)
	if err != nil {
		return errors.WithMessage(err, "create CryptoSuite failed")
	}
	ide, err := newIdentityFromUserConfig(userC)
	if err != nil {
		return errors.WithMessage(err, "create Identity failed")
	}

	cc := ChainCode{Name: CSCC,
		Type:     ChaincodeSpec_GOLANG,
		Args:     []string{"JoinChain"},
		argBytes: blockBytes}

	_, signedProp, err := createSignedProposal(*ide, cc, crypto)
	if err != nil {
		return errors.New("failed to create signed proposal for joining channel")
	}

	peer, err := newPeer(peerC, crypto)
	if err != nil {
		return errors.WithMessage(err, "new peer failed")
	}
	defer peer.conn.Close()

	resp, err := peer.Endorse(signedProp, other)
	if err != nil {
		return errors.WithMessage(err, "join channel failed")
	}
	if resp.Response.Status != int32(common.Status_SUCCESS) {
		return errors.Errorf("join channel failed with status %d and message is %s", resp.Response.Status, resp.Response.Message)
	}

	return nil
}
