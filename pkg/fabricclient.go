package gosdk

import (
	"math"
	"path/filepath"
	"strconv"

	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/pkg/errors"
)

type FabricClientImpl struct {
	client                *Client
	identity              map[string]Identity
	channelChaincodePeers map[string]map[string]*endorsePeers // 第一维为通道，第二维为智能合约，对应的peer信息
	channelPeers          map[string][]*Peer                  // 通道对应的peer信息
}

// 合约触发操作类函数

// getEndorsePeers 根据通道和合约名称获取对应peer
func (fc *FabricClientImpl) getEndorsePeers(cName, ccName string) (*endorsePeers, error) {
	ccPeers, ok := fc.channelChaincodePeers[cName]
	if !ok {
		return nil, errors.Errorf("channel %s is not configured", cName)
	}
	peers, ok := ccPeers[ccName]
	if !ok {
		return nil, errors.Errorf("chaincode %s is not configured", ccName)
	}

	return peers, nil
}

// getChannelPeers 根据通道名获取对应的Peer
func (fc *FabricClientImpl) getChannelPeers(cName string) ([]*Peer, error) {
	ps, ok := fc.channelPeers[cName]
	if !ok {
		return nil, errors.Errorf("channel %s is not configured", cName)
	}
	return ps, nil
}

// Init 执行chaincode实例化操作，在approve和commit时，采用参数InitReqired决定是否必要执行
func (fc *FabricClientImpl) Init(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error) {
	logger.Debug("enter fabric client init progress")
	defer logger.Debug("exit fabric client init progress")

	identity, peers, chaincode, err := fc.getBasicInfo(args, transientMap, cName, ccName, userName, true)
	if err != nil {
		return nil, errors.WithMessage(err, "get init basic info failed")
	}

	return fc.client.invoke(*identity, *chaincode, peers.invokePeers)
}

// Invoke 调用合约提供的写入账本方法
func (fc *FabricClientImpl) Invoke(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*InvokeResponse, error) {
	logger.Debug("enter fabric client invoke progress")
	defer logger.Debug("exit fabric client invoke progress")

	identity, peers, chaincode, err := fc.getBasicInfo(args, transientMap, cName, ccName, userName, false)
	if err != nil {
		return nil, errors.WithMessage(err, "get invoke basic info failed")
	}

	return fc.client.invoke(*identity, *chaincode, peers.invokePeers)
}

// Query 调用合约提供的查询方法
// 会对peer进行轮询查找，以增加高可用特性
func (fc *FabricClientImpl) Query(args []string, transientMap map[string][]byte, cName, ccName, userName string) (*peer.ProposalResponse, error) {
	logger.Debug("enter fabric client query progress")
	defer logger.Debug("exit fabric client query progress")

	identity, peers, chaincode, err := fc.getBasicInfo(args, transientMap, cName, ccName, userName, false)
	if err != nil {
		return nil, errors.WithMessage(err, "get query basic info failed")
	}

	_, sProposal, err := createSignedProposal(*identity, *chaincode, fc.client.Crypto)
	if err != nil {
		return nil, err
	}

	return querySinglePeer(sProposal, peers.queryPeers)
}

// event监听类函数

// ListenEventFullBlock 从指定的区块号对通道进行监听
// 返回channel，用于接受具体的区块信息，该区块信息不是原生的区块，被gosdk内部进行了整理
func (fc *FabricClientImpl) ListenEventFullBlock(channelName string, startNum uint64, fullBlockCh chan pBlock.Block) chan error {
	logger.Debug("enter fabric client listen full block progress")
	defer logger.Debug("exit fabric client listen full block progress")

	return fc.client.listenForFullBlock(fc.identity[defaultUser], startNum, channelName, fullBlockCh)
}

// ListenEventFilterBlock 从指定的区块号对通道进行监听
func (fc *FabricClientImpl) ListenEventFilterBlock(channelName string, startNum uint64, filterBlockCh chan FilteredBlockResponse) chan error {
	logger.Debug("enter fabric client listen filter block progress")
	defer logger.Debug("exit fabric client listen filter block progress")

	return fc.client.listenForFilteredBlock(fc.identity[defaultUser], startNum, channelName, filterBlockCh)
}

// Admin操作类函数

// CreateUpdateChannel 创建或更新通道
// 通过配置路径读取通道配置文件，签名发送给orderer节点，在操作时使用默认的admin身份
func (fc *FabricClientImpl) CreateUpdateChannel(channelId, path string) error {
	logger.Debug("enter fabric client create or update channel progress")
	defer logger.Debug("exit fabric client create or update channel progress")

	i, ok := fc.identity[adminUser]
	if !ok {
		return errors.New("admin user is not configured")
	}
	if err := fc.client.createUpdateChannel(i, path, channelId); err != nil {
		return errors.WithMessage(err, "failed to create or update channel")
	}

	return nil
}

// JoinChannel 将某个节点加入某个通道
func (fc *FabricClientImpl) JoinChannel(channelId, peerName string) (*peer.ProposalResponse, error) {
	logger.Debug("enter fabric client join channel progress")
	defer logger.Debug("exit fabric client join channel progress")

	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}
	res, err := fc.client.joinChannel(i, channelId, peerName)
	if err != nil {
		return nil, errors.WithMessagef(err, "peer %s join channel %s failed", peerName, channelId)
	}
	if res == nil {
		return nil, errors.Errorf("peer %s does not return response when join channel %s", peerName, channelId)
	}
	if res.Err != nil {
		return nil, errors.WithMessagef(err, "peer %s join channel %s failed", peerName, channelId)
	}

	return res.Response, nil
}

// UpdateAnchorPeer 更新锚节点
// 通过配置路径读取通道配置文件，签名发送给orderer节点，在操作时使用默认的admin身份
func (fc *FabricClientImpl) UpdateAnchorPeer(channelId, path string) error {
	logger.Debug("enter fabric client update anchor peer progress")
	defer logger.Debug("exit fabric client update anchor peer progress")

	i, ok := fc.identity[adminUser]
	if !ok {
		return errors.New("admin user is not configured")
	}

	if err := fc.client.createUpdateChannel(i, path, channelId); err != nil {
		return errors.WithMessagef(err, "update anchor peer for channel %s", channelId)
	}

	return nil
}

// InstallChainCode 对指定peer安装智能合约
func (fc *FabricClientImpl) InstallChainCode(peerName string, req *InstallRequest) (*peer.ProposalResponse, error) {
	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	res, err := fc.client.installChainCode(i, req, peerName)
	if err != nil {
		return nil, errors.WithMessagef(err, "install cc to peer %s failed", peerName)
	}
	if res == nil {
		return nil, errors.Errorf("peer %s does not return response when install cc", peerName)
	}

	return res.Response, nil
}

// InstantiateChainCode 实例化合约
// 实例化信息只需要发送给一个peer即可，默认取第一个peer
func (fc *FabricClientImpl) InstantiateChainCode(policy string, req *ChainCode) (*orderer.BroadcastResponse, error) {
	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}
	peers, err := fc.getChannelPeers(req.ChannelId)
	if err != nil {
		return nil, err
	}
	res, err := fc.client.instantiateChainCode(i, req, peers, deploy, policy)
	if err != nil {
		return nil, errors.WithMessage(err, "instantiate cc failed")
	}

	return res, nil
}

// InstallChainCodeV2 2.2版本fabric对指定peer安装智能合约
func (fc *FabricClientImpl) InstallChainCodeV2(chaincodePkg []byte, peerName string) (*lifecycle.InstallChaincodeResult, error) {
	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	return fc.client.installChainCodeV2(i, chaincodePkg, peerName)
}

// QueryInstalled 查询peer上已安装的chiancode
func (fc *FabricClientImpl) QueryInstalledV2(peerName string) ([]*lifecycle.QueryInstalledChaincodesResult_InstalledChaincode, error) {
	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	peers := make([]*Peer, 0)
	peer, ok := fc.client.Peers[peerName]
	if !ok {
		return nil, errors.Errorf("peer %s is not exists", peerName)
	}
	peers = append(peers, peer)

	args := &lifecycle.QueryInstalledChaincodesArgs{}
	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal args")
	}

	cc := ChainCode{
		Name: lifecycleName,
		Args: []string{"QueryInstalledChaincodes", string(argsBytes)},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}

	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessage(err, "query chaincodes installed failed")
	}

	qicr := &lifecycle.QueryInstalledChaincodesResult{}
	err = proto.Unmarshal(res.Response.Payload, qicr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}

	return qicr.InstalledChaincodes, nil
}

// GetInstalledPackage 从peer获得已安装的chaincode包
func (fc *FabricClientImpl) GetInstalledPackageV2(packageID, outputDirectory, peerName string) error {
	if packageID == "" || outputDirectory == "" {
		return errors.New("packageID and outputDirectory cannot be empty")
	}

	i, ok := fc.identity[adminUser]
	if !ok {
		return errors.New("admin user is not configured")
	}

	peers := make([]*Peer, 0)
	peer, ok := fc.client.Peers[peerName]
	if !ok {
		return errors.Errorf("peer %s is not exists", peerName)
	}
	peers = append(peers, peer)

	args := &lifecycle.GetInstalledChaincodePackageArgs{
		PackageId: packageID,
	}
	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return errors.Wrap(err, "failed to marshal args")
	}

	cc := ChainCode{
		Name: lifecycleName,
		Args: []string{"GetInstalledChaincodePackage", string(argsBytes)},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return err
	}

	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return errors.WithMessage(err, "query chaincodes installed failed")
	}

	return writePackage(res, outputDirectory, packageID)
}

// ApproveForMyOrg 组织对chaincode进行审批
func (fc *FabricClientImpl) ApproveForMyOrg(acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error) {
	if err := acReq.validate(); err != nil {
		return nil, err
	}

	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	return fc.client.approveForMyOrg(i, acReq)
}

// QueryApproved 查询组织已经通过approved的chaincode，当sequence不指定时查询最新的sequence
func (fc *FabricClientImpl) QueryApproved(channelName, chaincodeName, orgName string, sequence int64) (*ApproveChaincode, error) {
	if channelName == "" || chaincodeName == "" {
		return nil, errors.New("channelName and chaincodeName cannot be empty")
	}

	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	peers, ok := fc.client.orgPeers[orgName]
	if !ok {
		return nil, errors.Errorf("org %s is not exists", orgName)
	}

	args := &lifecycle.QueryApprovedChaincodeDefinitionArgs{
		Name:     chaincodeName,
		Sequence: sequence,
	}
	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal args")
	}

	cc := ChainCode{
		Name:      lifecycleName,
		ChannelId: channelName,
		Args:      []string{"QueryApprovedChaincodeDefinition", string(argsBytes)},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}

	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessage(err, "query chaincodes installed failed")
	}

	return parseApprovedChaincode(res)
}

// LifecycleCommit 在超过一半组织进行approve后进行commit
func (fc *FabricClientImpl) LifecycleCommit(acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error) {
	if err := acReq.validate(); err != nil {
		return nil, err
	}

	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	peers, err := fc.getEndorsePeers(acReq.ChannelName, acReq.ChaincodeName)
	if err != nil {
		return nil, errors.Errorf("get endorse peers failed: %s", err.Error())
	}

	return fc.client.lifecycleCommit(i, peers, acReq)
}

// CheckCommitreadiness 检查chaincode是否可以commit
func (fc *FabricClientImpl) CheckCommitreadiness(request CheckCommitreadinessRequest) (map[string]bool, error) {
	if err := request.validate(); err != nil {
		return nil, err
	}

	i, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	peers, err := fc.getChannelPeers(request.ChannelName)
	if err != nil {
		return nil, errors.Errorf("get peers in %s failed", request.ChannelName)
	}
	policyBytes, err := createPolicyBytes(request.SignaturePolicy, "")
	if err != nil {
		return nil, errors.WithMessage(err, "create policy failed")
	}
	args := &lifecycle.CheckCommitReadinessArgs{
		Name:                request.ChaincodeName,
		Version:             request.ChaincodeVserison,
		Sequence:            request.Sequence,
		ValidationParameter: policyBytes,
		InitRequired:        request.InitRequired,
	}
	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal args")
	}

	cc := ChainCode{
		Name:      lifecycleName,
		ChannelId: request.ChannelName,
		Args:      []string{"CheckCommitReadiness", string(argsBytes)},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}

	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessage(err, "query chaincodes installed failed")
	}

	result := &lifecycle.CheckCommitReadinessResult{}
	err = proto.Unmarshal(res.Response.Payload, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}

	return result.Approvals, nil
}

// PackageChaincode 将chaincode打包成tar.gz
func (fc *FabricClientImpl) PackageChaincode(info ChaincodeInfo) error {
	if err := info.validate(); err != nil {
		return err
	}

	pkgTarGzBytes, err := getTarGzBytes(info)
	if err != nil {
		return err
	}

	dir, name := filepath.Split(info.OutputFile)
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}
	err = writeFile(dir, name, pkgTarGzBytes)
	if err != nil {
		err = errors.Wrapf(err, "error writing chaincode package to %s", info.OutputFile)
		return err
	}

	return nil
}

// QueryCommitted 查询已经提交的chaincode
func (fc *FabricClientImpl) QueryCommitted(input CommittedQueryInput) (*lifecycle.QueryChaincodeDefinitionResult, *lifecycle.QueryChaincodeDefinitionsResult, error) {
	identity, ok := fc.identity[adminUser]
	if !ok {
		return nil, nil, errors.New("admin user is not configured")
	}

	if input.ChannelID == "" {
		return nil, nil, errors.New("channel name must be specified")
	}
	peers, err := fc.getChannelPeers(input.ChannelID)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "get channel peers failed")
	}
	return fc.client.queryCommitted(input, peers, identity)
}

// 账本查询类函数

func (fc *FabricClientImpl) GetBlockHeight(channelName string) (uint64, error) {
	i, ok := fc.identity[defaultUser]
	if !ok {
		return 0, errors.New("user is not configured")
	}

	if channelName == "" {
		return 0, errors.New("channel name is nil")
	}
	peers, err := fc.getChannelPeers(channelName)
	if err != nil {
		return 0, err
	}

	cc := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      []string{"GetChainInfo", channelName},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return 0, err
	}
	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return 0, errors.WithMessagef(err, "failed get block height in channel %s", channelName)
	}

	h, err := parseHeight(res)
	if err != nil {
		return 0, errors.WithMessage(err, "parse response from peer %s to height failed")
	}
	return h, nil
}

func (fc *FabricClientImpl) GetBlockByNumber(channelName string, blockNum uint64) (*common.Block, error) {
	i, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	if channelName == "" {
		return nil, errors.New("channel name is nil")
	}
	peers, err := fc.getChannelPeers(channelName)
	if err != nil {
		return nil, err
	}

	strBlockNum := strconv.FormatUint(blockNum, 10)
	cc := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      []string{"GetBlockByNumber", channelName, strBlockNum},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}
	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get block %d in channel %s", blockNum, channelName)
	}

	b, err := parseBlock(res)
	if err != nil {
		return nil, errors.WithMessage(err, "parse response to block failed")
	}
	return b, nil
}

func (fc *FabricClientImpl) GetBlockByTxID(channelName string, txID string) (*common.Block, error) {
	i, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	if txID == "" || channelName == "" {
		return nil, errors.New("channel name or txID is empty")
	}
	peers, err := fc.getChannelPeers(channelName)
	if err != nil {
		return nil, err
	}

	cc := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      []string{"GetBlockByTxID", channelName, txID},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}
	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get block by txID %s in channel %s", txID, channelName)
	}

	b, err := parseBlock(res)
	if err != nil {
		return nil, errors.WithMessage(err, "parse response to block failed")
	}
	return b, nil
}

func (fc *FabricClientImpl) GetTransactionByTxID(channelName string, txID string) (*peer.ProcessedTransaction, error) {
	i, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	if txID == "" || channelName == "" {
		return nil, errors.New("channel name or txid is empty")
	}
	peers, err := fc.getChannelPeers(channelName)
	if err != nil {
		return nil, err
	}

	cc := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      []string{"GetTransactionByID", channelName, txID},
	}
	// 只对第一个节点进行查询
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}
	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get tx %s in channel %s", txID, channelName)
	}

	t, err := parseTX(res)
	if err != nil {
		return nil, errors.WithMessage(err, "parse response to tx failed")
	}
	return t, nil
}

func (fc *FabricClientImpl) GetNewestBlock(channelName string) (*common.Block, error) {
	i, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	if channelName == "" {
		return nil, errors.New("channel name is nil")
	}
	peers, err := fc.getChannelPeers(channelName)
	if err != nil {
		return nil, err
	}

	// 当区块号为math.MaxUint64，默认获取最新区块
	strBlockNum := strconv.FormatUint(math.MaxUint64, 10)
	cc := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      []string{"GetBlockByNumber", channelName, strBlockNum},
	}
	_, sProposal, err := createSignedProposal(i, cc, fc.client.Crypto)
	if err != nil {
		return nil, err
	}
	res, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get last block in channel %s", channelName)
	}

	b, err := parseBlock(res)
	if err != nil {
		return nil, errors.WithMessage(err, "parse response to block failed")
	}
	return b, nil
}

// 服务发现类函数

func (fc *FabricClientImpl) DiscoveryChannelPeers(channel string) ([]ChannelPeer, error) {
	ide, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	for _, p := range fc.client.Peers {
		r, err := channelPeerRequest(channel, ide.MspId, nil, nil, ide.Cert, p.tlsCertHash)
		if err != nil {
			logger.Errorf("construct discovery peer in channel %s for peer %s failed: %s", channel, p.uri, err.Error())
			continue
		}

		sr, err := signDiscoveryRequest(r, fc.client.Crypto, ide.PrivateKey)
		if err != nil {
			logger.Errorf("sign discovery request for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		resp, err := sendDiscoverySignedRequest(sr, &p.node)
		if err != nil {
			logger.Errorf("send discovery request to peer %s failed: %s", p.uri, err.Error())
			continue
		}
		if len(resp.Results) != 1 {
			logger.Errorf("get %d results from peer %s, which is expected to be 1", len(resp.Results), p.uri)
			continue
		}
		cps, err := parseChannelPeers(resp.Results[0])
		if err != nil {
			logger.Errorf("parse channel peers from peer %s failed: %s", p.uri, err.Error())
			continue
		}

		return cps, nil
	}

	return nil, errors.New("discovery channel peers to all peers failed")
}

func (fc *FabricClientImpl) DiscoveryLocalPeers() ([]LocalPeer, error) {
	ide, ok := fc.identity[adminUser]
	if !ok {
		return nil, errors.New("admin user is not configured")
	}

	for _, p := range fc.client.Peers {
		r, err := localPeerRequest("", ide.MspId, nil, nil, ide.Cert, p.tlsCertHash)
		if err != nil {
			logger.Errorf("construct discovery local peer for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		sr, err := signDiscoveryRequest(r, fc.client.Crypto, ide.PrivateKey)
		if err != nil {
			logger.Errorf("sign discovery request for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		resp, err := sendDiscoverySignedRequest(sr, &p.node)
		if err != nil {
			logger.Errorf("send discovery request to peer %s failed: %s", p.uri, err.Error())
			continue
		}
		if len(resp.Results) != 1 {
			logger.Errorf("get %d results from peer %s, which is expected to be 1", len(resp.Results), p.uri)
			continue
		}
		lps, err := parseLocalPeer(resp.Results[0])
		if err != nil {
			logger.Errorf("parse local peer from peer %s failed: %s", p.uri, err.Error())
			continue
		}

		return lps, nil
	}

	return nil, errors.New("discovery local peers to all peers failed")
}

func (fc *FabricClientImpl) DiscoveryChannelConfig(channel string) (*discovery.ConfigResult, error) {
	ide, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	for _, p := range fc.client.Peers {
		r, err := channelConfigRequest(channel, ide.MspId, nil, nil, ide.Cert, p.tlsCertHash)
		if err != nil {
			logger.Errorf("construct discovery channel config for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		sr, err := signDiscoveryRequest(r, fc.client.Crypto, ide.PrivateKey)
		if err != nil {
			logger.Errorf("sign discovery request for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		resp, err := sendDiscoverySignedRequest(sr, &p.node)
		if err != nil {
			logger.Errorf("send discovery request to peer %s failed: %s", p.uri, err.Error())
			continue
		}
		if len(resp.Results) != 1 {
			logger.Errorf("get %d results from peer %s, which is expected to be 1", len(resp.Results), p.uri)
			continue
		}
		cc, err := parseChannelConfig(resp.Results[0])
		if err != nil {
			logger.Errorf("parse channel config from peer %s failed: %s", p.uri, err.Error())
			continue
		}

		return cc, nil
	}

	return nil, errors.Errorf("discovery channel %s config to all peers failed", channel)
}

func (fc *FabricClientImpl) DiscoveryEndorsePolicy(channel string, chaincodes []string, collections map[string]string) ([]EndorsermentDescriptor, error) {
	ide, ok := fc.identity[defaultUser]
	if !ok {
		return nil, errors.New("user is not configured")
	}

	for _, p := range fc.client.Peers {
		r, err := endorsePolicyRequest(channel, ide.MspId, chaincodes, collections, ide.Cert, p.tlsCertHash)
		if err != nil {
			logger.Errorf("construct discovery endorse policy for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		sr, err := signDiscoveryRequest(r, fc.client.Crypto, ide.PrivateKey)
		if err != nil {
			logger.Errorf("sign discovery request for peer %s failed: %s", p.uri, err.Error())
			continue
		}

		resp, err := sendDiscoverySignedRequest(sr, &p.node)
		if err != nil {
			logger.Errorf("send discovery request to peer %s failed: %s", p.uri, err.Error())
			continue
		}
		if len(resp.Results) != 1 {
			logger.Errorf("get %d results from peer %s, which is expected to be 1", len(resp.Results), p.uri)
			continue
		}
		ep, err := parseEndorsePolicy(resp.Results[0])
		if err != nil {
			logger.Errorf("parse endorse policy from peer %s failed: %s", p.uri, err.Error())
			continue
		}

		return ep, nil
	}

	return nil, errors.New("discovery endorse policy to all peers failed")
}

func (fc *FabricClientImpl) AddUser(userName string, userConfig UserConfig) error {
	user, err := newIdentityFromUserConfig(userConfig)
	if err != nil {
		return errors.WithMessagef(err, "create user %s failed", userName)
	}
	fc.identity[userName] = *user
	return nil
}

func (fc *FabricClientImpl) GetCrypto() (CryptoSuite, error) {
	if fc.client.Crypto == nil {
		return nil, errors.New("crypto suite is nil")
	}

	return fc.client.Crypto, nil
}

// getBasicInfo 获取交易基本信息：identity，peer，chaincode信息
func (fc *FabricClientImpl) getBasicInfo(args []string, transientMap map[string][]byte, cName, ccName, userName string, isInit bool) (*Identity, *endorsePeers, *ChainCode, error) {
	if userName == "" {
		userName = defaultUser
	}
	identity, ok := fc.identity[userName]
	if !ok {
		return nil, nil, nil, errors.Errorf("user %s is not configured", userName)
	}

	peers, err := fc.getEndorsePeers(cName, ccName)
	if err != nil {
		return nil, nil, nil, errors.WithMessage(err, "get endorse peers failed")
	}
	chaincode, err := getChainCodeObj(args, transientMap, cName, ccName, peers.chanincodeType, isInit)
	if err != nil {
		return nil, nil, nil, errors.WithMessage(err, "get chaincode object failed")
	}

	return &identity, peers, chaincode, nil
}

func parseApprovedChaincode(proposalResponse *peer.ProposalResponse) (*ApproveChaincode, error) {
	var packageID string

	result := &lifecycle.QueryApprovedChaincodeDefinitionResult{}
	err := proto.Unmarshal(proposalResponse.Response.Payload, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}

	if result.Source != nil {
		switch source := result.Source.Type.(type) {
		case *lifecycle.ChaincodeSource_LocalPackage:
			packageID = source.LocalPackage.PackageId
		case *lifecycle.ChaincodeSource_Unavailable_:
		}
	}

	return &ApproveChaincode{Sequence: result.Sequence,
		Version:           result.Version,
		InitRequired:      result.InitRequired,
		PackageID:         packageID,
		EndorsementPlugin: result.EndorsementPlugin,
		ValidationPlugin:  result.ValidationPlugin}, nil
}
