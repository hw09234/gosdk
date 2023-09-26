/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	pBlock "github.com/hw09234/gosdk/pkg/parseBlock"
	"github.com/hw09234/gosdk/pkg/syncTxStatus"
	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-protos-go/peer/lifecycle"
	"github.com/hyperledger/fabric/common/policydsl"
	"github.com/pkg/errors"
	"math"
	"sync/atomic"
)

// FabricClient Hyperledger Fabric API
type Client struct {
	current   uint64
	Crypto    CryptoSuite
	Peers     map[string]*Peer   // 根据peer名称统计Peer信息
	orgPeers  map[string][]*Peer // 根据组织名称统计Peer信息
	Orderers  *OrderersInfo
	Listeners []*EventListener
}

// endorsePeers 将初始化的peer进行整理
// 主要分为两大类，一类用于执行写入类型的交易，通过解析背书策略获取
// 一类用于执行查询类型的交易，无需关注背书策略，只要用户配置了就纳入
type endorsePeers struct {
	invokePeers    map[string][]*Peer
	queryPeers     []*Peer
	allOrgs        map[string][]*Peer // 包含所有组织，用于commit向所有组织发送
	chanincodeType ChainCodeType
}

// getNextListener 获取下一个可用listener
func (c *Client) getNextListener() *EventListener {
	logger.Debug("enter get an available peer progress")
	defer logger.Debug("exit get an available peer progress")

	preCurrent := int(c.current)
	next := int(c.current)
	l := len(c.Listeners)
	// 如果当前游标对应节点可用，则直接返回
	if c.Listeners[next].peer.isAlive() {
		logger.Debugf("the current peer %s is available, no need to choose the next one", c.Listeners[next].peer.uri)
		return c.Listeners[next]
	}
	for {
		next = (next + 1) % l
		// 所有节点已被循环一遍，无可用节点，直接返回空
		if next == preCurrent {
			logger.Error("all peers are not alive, return nil")
			return nil
		}
		// 如果节点可用，则修改current游标，并返回对应的节点
		if c.Listeners[next].peer.isAlive() {
			logger.Debugf("currently using peer %s", c.Listeners[next].peer.uri)
			atomic.StoreUint64(&c.current, uint64(next))
			return c.Listeners[next]
		}
		logger.Warnf("%s is unavailable, choose next", c.Listeners[next].peer.uri)
	}
}

// CreateUpdateChannel 创建或更新通道
func (c *Client) createUpdateChannel(identity Identity, path, channelId string) error {
	envelope, err := decodeChannelFromFs(path)
	if err != nil {
		return errors.WithMessage(err, "decode channel file to envelope failed")
	}
	ou, err := buildAndSignChannelConfig(identity, envelope.GetPayload(), c.Crypto, channelId)
	if err != nil {
		return errors.WithMessage(err, "signed envelope's payload failed")
	}

	_, err = sendToOrderersBroadcast(c.Orderers, ou)
	if err != nil {
		return errors.WithMessage(err, "broadcast to orderer failed")
	}

	return nil
}

// joinChannel 指定peer加入通道
func (c *Client) joinChannel(identity Identity, channelId string, peerName string) (*PeerResponse, error) {
	p, ok := c.Peers[peerName]
	if !ok {
		return nil, errors.Errorf("peerName %s not exist", peerName)
	}

	block, err := getGenesisBlock(c.Orderers, identity, c.Crypto, channelId)
	if err != nil {
		logger.Errorf("get genesis block failed: %v", err)
		return nil, err
	}

	blockBytes, err := proto.Marshal(block)
	if err != nil {
		logger.Errorf("proto marshal failed: %v", err)
		return nil, err
	}

	chainCode := ChainCode{
		Name:     CSCC,
		Type:     ChaincodeSpec_GOLANG,
		Args:     []string{"JoinChain"},
		argBytes: blockBytes,
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, errors.WithMessage(err, "create signed proposal failed")
	}

	resp, err := p.Endorse(sProposal, other)
	if err != nil {
		return nil, err
	}
	return &PeerResponse{Response: resp, Name: p.name}, nil
}

// installChainCode 安装chaincode
func (c *Client) installChainCode(identity Identity, req *InstallRequest, peerName string) (*PeerResponse, error) {
	var packageBytes []byte
	var err error

	p, ok := c.Peers[peerName]
	if !ok {
		return nil, errors.Errorf("peerName %s not exist", peerName)
	}

	switch req.ChainCodeType {
	case ChaincodeSpec_GOLANG:
		packageBytes, err = packGolangCC(req.Namespace, req.SrcPath, req.Libraries)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedChaincodeType
	}

	depSpec, err := proto.Marshal(&peer.ChaincodeDeploymentSpec{
		ChaincodeSpec: &peer.ChaincodeSpec{
			ChaincodeId: &peer.ChaincodeID{Name: req.ChainCodeName, Path: req.Namespace, Version: req.ChainCodeVersion},
			Type:        peer.ChaincodeSpec_Type(req.ChainCodeType),
		},
		CodePackage: packageBytes,
	})
	if err != nil {
		logger.Errorf("proto marshal chaincode deployment spec failed: %v", err)
		return nil, err
	}

	chaincode := ChainCode{
		Type:     req.ChainCodeType,
		Name:     LSCC,
		Args:     []string{"install"},
		argBytes: depSpec,
	}
	_, sProposal, err := createSignedProposal(identity, chaincode, c.Crypto)

	resp, err := p.Endorse(sProposal, other)
	if err != nil {
		return nil, err
	}

	if err = validateProposalResponse(resp); err != nil {
		return nil, errors.WithMessage(err, "failed to ProposalResponse failed")
	}

	return &PeerResponse{Response: resp, Name: p.name}, nil
}

// instantiateChainCode 实例化chaincode
func (c *Client) instantiateChainCode(identity Identity, req *ChainCode, peers []*Peer, operation, policy string) (*orderer.BroadcastResponse, error) {
	depSpec, err := chainCodeDeploymentSpec(*req)
	if err != nil {
		logger.Errorf("proto marshal chaincode deployment spec failed: %v", err)
		return nil, err
	}

	policyEnv := &common.SignaturePolicyEnvelope{}
	if policy == "" {
		logger.Debugf("the policy is empty, use %s signed", identity.MspId)
		policyEnv = policydsl.SignedByMspMember(identity.MspId)
	} else {
		policyEnv, err = policydsl.FromString(policy)
		if err != nil {
			return nil, errors.WithMessage(err, "create signaturePolicyEnv failed")
		}
	}
	marshPolicy, err := proto.Marshal(policyEnv)
	if err != nil {
		return nil, errors.WithMessage(err, "marshal signaturePolicyEnv failed")
	}

	args := [][]byte{
		[]byte(operation),
		[]byte(req.ChannelId),
		depSpec,
		marshPolicy,
		[]byte("escc"),
		[]byte("vscc"),
	}
	chaincode := ChainCode{
		ChannelId: req.ChannelId,
		Type:      req.Type,
		Name:      LSCC,
		rawArgs:   args,
	}

	propsal, sProposal, err := createSignedProposal(identity, chaincode, c.Crypto)
	if err != nil {
		logger.Errorf("create signed proposal failed: %v", err)
		return nil, err
	}

	// 实例化只需要发给一个peer
	mPeers := make(map[string][]*Peer, 1)
	mPeers["instantiate"] = peers
	resp := sendToPeers(mPeers, sProposal, instantiate)

	transaction, err := createTransaction(propsal.proposal, resp)
	if err != nil {
		return nil, err
	}

	signedTransaction, err := c.Crypto.Sign(transaction, identity.PrivateKey)
	if err != nil {
		return nil, err
	}

	envelope := &common.Envelope{Payload: transaction, Signature: signedTransaction}
	reply, err := sendToOrderersBroadcast(c.Orderers, envelope)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

// installChainCodeV2 fabricV2.2版本安装chaincode
func (c *Client) installChainCodeV2(identity Identity, pkg []byte, peerName string) (*lifecycle.InstallChaincodeResult, error) {
	peer, ok := c.Peers[peerName]
	if !ok {
		return nil, errors.Errorf("peerName %s not exist", peerName)
	}

	installChaincodeArgs := &lifecycle.InstallChaincodeArgs{
		ChaincodeInstallPackage: pkg,
	}
	installChaincodeArgsBytes, err := proto.Marshal(installChaincodeArgs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal InstallChaincodeArgs")
	}
	chainCode := ChainCode{
		Name:     lifecycleName,
		Type:     ChaincodeSpec_GOLANG,
		Args:     []string{"InstallChaincode"},
		argBytes: installChaincodeArgsBytes,
	}
	_, sProposl, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		logger.Errorf("create signed proposal failed: %v", err)
		return nil, err
	}

	resp, err := peer.Endorse(sProposl, other)
	if err != nil {
		logger.Errorf("%s endorse failed: %v", peerName, err)
		return nil, err
	}

	if err = validateProposalResponse(resp); err != nil {
		return nil, errors.WithMessage(err, "failed to ProposalResponse failed")
	}

	icr := &lifecycle.InstallChaincodeResult{}
	if err = proto.Unmarshal(resp.Response.Payload, icr); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}
	return icr, nil
}

// approveForMyOrg 组织审核chaincode
func (c *Client) approveForMyOrg(identity Identity, acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error) {
	policyBytes, err := createPolicyBytes(acReq.SignaturePolicy, acReq.ChannelConfigPolicy)
	if err != nil {
		return nil, errors.WithMessage(err, "create policy failed")
	}

	ccsrc := &lifecycle.ChaincodeSource{
		Type: &lifecycle.ChaincodeSource_LocalPackage{
			LocalPackage: &lifecycle.ChaincodeSource_Local{
				PackageId: acReq.PackageID,
			},
		},
	}

	args := &lifecycle.ApproveChaincodeDefinitionForMyOrgArgs{
		Name:                acReq.ChaincodeName,
		Version:             acReq.ChaincodeVserison,
		Sequence:            acReq.Sequence,
		ValidationParameter: policyBytes,
		InitRequired:        acReq.InitReqired,
		Source:              ccsrc,
	}

	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, err
	}

	chainCode := ChainCode{
		ChannelId: acReq.ChannelName,
		Name:      lifecycleName,
		Type:      ChaincodeSpec_GOLANG,
		Args:      []string{"ApproveChaincodeDefinitionForMyOrg"},
		argBytes:  argsBytes,
	}
	proposal, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)

	peers := c.orgPeers[acReq.OrgName]
	mPeers := make(map[string][]*Peer, 1)
	mPeers["approve"] = peers
	propResponses := sendToPeers(mPeers, sProposal, other)

	transaction, err := createTransaction(proposal.proposal, propResponses)
	if err != nil {
		return nil, err
	}

	signedTransaction, err := c.Crypto.Sign(transaction, identity.PrivateKey)
	if err != nil {
		return nil, err
	}

	envelope := &common.Envelope{Payload: transaction, Signature: signedTransaction}
	reply, err := sendToOrderersBroadcast(c.Orderers, envelope)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// lifecycleCommit 提交chaincode
func (c *Client) lifecycleCommit(identity Identity, peers *endorsePeers, acReq ApproveCommitRequest) (*orderer.BroadcastResponse, error) {
	policyBytes, err := createPolicyBytes(acReq.SignaturePolicy, acReq.ChannelConfigPolicy)
	if err != nil {
		return nil, errors.WithMessage(err, "create policy failed")
	}
	args := &lifecycle.CommitChaincodeDefinitionArgs{
		Name:                acReq.ChaincodeName,
		Version:             acReq.ChaincodeVserison,
		Sequence:            acReq.Sequence,
		ValidationParameter: policyBytes,
		InitRequired:        acReq.InitReqired,
	}
	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, err
	}

	chainCode := ChainCode{
		ChannelId: acReq.ChannelName,
		Name:      lifecycleName,
		Type:      ChaincodeSpec_GOLANG,
		Args:      []string{"CommitChaincodeDefinition"},
		argBytes:  argsBytes,
	}
	proposal, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)

	propResponses := sendToPeers(peers.allOrgs, sProposal, instantiate)
	transaction, err := createTransaction(proposal.proposal, propResponses)
	if err != nil {
		return nil, err
	}

	signedTransaction, err := c.Crypto.Sign(transaction, identity.PrivateKey)
	if err != nil {
		return nil, err
	}

	envelope := &common.Envelope{Payload: transaction, Signature: signedTransaction}
	reply, err := sendToOrderersBroadcast(c.Orderers, envelope)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// queryCommitted 查询已经commit了的chaincode信息
func (c *Client) queryCommitted(input CommittedQueryInput, peers []*Peer, identity Identity) (*lifecycle.QueryChaincodeDefinitionResult, *lifecycle.QueryChaincodeDefinitionsResult, error) {
	var function string
	var args proto.Message

	if input.ChaincodeName != "" {
		function = "QueryChaincodeDefinition"
		args = &lifecycle.QueryChaincodeDefinitionArgs{
			Name: input.ChaincodeName,
		}
	} else {
		function = "QueryChaincodeDefinitions"
		args = &lifecycle.QueryChaincodeDefinitionsArgs{}
	}

	argsBytes, err := proto.Marshal(args)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to marshal args")
	}

	chainCode := ChainCode{
		ChannelId: input.ChannelID,
		Name:      lifecycleName,
		Type:      ChaincodeSpec_GOLANG,
		Args:      []string{function, string(argsBytes)},
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)

	proposalResponse, err := querySinglePeer(sProposal, peers)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to endorse proposal")
	}
	if err = validateProposalResponse(proposalResponse); err != nil {
		return nil, nil, errors.WithMessage(err, "failed to ProposalResponse failed")
	}

	if input.ChaincodeName != "" {
		result := &lifecycle.QueryChaincodeDefinitionResult{}
		err := proto.Unmarshal(proposalResponse.Response.Payload, result)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
		}
		return result, nil, nil
	}
	result := &lifecycle.QueryChaincodeDefinitionsResult{}
	err = proto.Unmarshal(proposalResponse.Response.Payload, result)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to unmarshal proposal response's response payload")
	}

	return nil, result, nil
}

// QueryInstalledChainCodes 查询已经安装了的chaincode
func (c *Client) QueryInstalledChainCodes(identity Identity, peers []*Peer) ([]*ChainCodesResponse, error) {
	if len(identity.MspId) == 0 {
		return nil, ErrMspMissing
	}
	chainCode := ChainCode{
		Name: LSCC,
		Type: ChaincodeSpec_GOLANG,
		Args: []string{"getinstalledchaincodes"},
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}

	// 将需要查询的peer构造为map结构，加快查询效率
	mPeers := make(map[string][]*Peer, len(peers))
	for _, p := range peers {
		mPeers[p.name] = []*Peer{p}
	}

	r := sendToPeers(mPeers, sProposal, other)

	response := make([]*ChainCodesResponse, len(r))
	for idx, p := range r {
		ic := ChainCodesResponse{PeerName: p.Name, Error: p.Err}
		if p.Err != nil {
			ic.Error = p.Err
		} else {
			dec, err := decodeChainCodeQueryResponse(p.Response.Response.GetPayload())
			if err != nil {
				ic.Error = err
			}
			ic.ChainCodes = dec
		}
		response[idx] = &ic
	}
	return response, nil
}

// QueryInstantiatedChainCodes 查询已经实例化的chaincode
func (c *Client) QueryInstantiatedChainCodes(identity Identity, channelId string, peers []*Peer) ([]*ChainCodesResponse, error) {
	chainCode := ChainCode{
		ChannelId: channelId,
		Name:      LSCC,
		Type:      ChaincodeSpec_GOLANG,
		Args:      []string{"getchaincodes"},
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}

	// 将需要查询的peer构造为map结构，加快查询效率
	mPeers := make(map[string][]*Peer, len(peers))
	for _, p := range peers {
		mPeers[p.name] = []*Peer{p}
	}
	r := sendToPeers(mPeers, sProposal, other)
	response := make([]*ChainCodesResponse, len(r))
	for idx, p := range r {
		ic := ChainCodesResponse{PeerName: p.Name, Error: p.Err}
		if p.Err != nil {
			ic.Error = p.Err
		} else {
			dec, err := decodeChainCodeQueryResponse(p.Response.Response.GetPayload())
			if err != nil {
				ic.Error = err
			}
			ic.ChainCodes = dec
		}
		response[idx] = &ic
	}
	return response, nil
}

// QueryChannels 查询peer加入的通道
func (c *Client) QueryChannels(identity Identity, peers []*Peer) ([]*QueryChannelsResponse, error) {
	chainCode := ChainCode{
		Name: CSCC,
		Type: ChaincodeSpec_GOLANG,
		Args: []string{"GetChannels"},
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}

	// 将需要查询的peer构造为map结构，加快查询效率
	mPeers := make(map[string][]*Peer, len(peers))
	for _, p := range peers {
		mPeers[p.name] = []*Peer{p}
	}

	r := sendToPeers(mPeers, sProposal, other)
	response := make([]*QueryChannelsResponse, 0, len(r))
	for _, pr := range r {
		peerResponse := QueryChannelsResponse{PeerName: pr.Name}
		if pr.Err != nil {
			peerResponse.Error = err
		} else {
			channels := new(peer.ChannelQueryResponse)
			if err := proto.Unmarshal(pr.Response.Response.Payload, channels); err != nil {
				peerResponse.Error = err

			} else {
				peerResponse.Channels = make([]string, 0, len(channels.Channels))
				for _, ci := range channels.Channels {
					peerResponse.Channels = append(peerResponse.Channels, ci.ChannelId)
				}
			}
		}
		response = append(response, &peerResponse)
	}
	return response, nil
}

// QueryChannelInfo 查询peer加入的通道信息
func (c *Client) QueryChannelInfo(identity Identity, channelId string, peers []*Peer) ([]*QueryChannelInfoResponse, error) {
	chainCode := ChainCode{
		ChannelId: channelId,
		Name:      QSCC,
		Type:      ChaincodeSpec_GOLANG,
		Args:      []string{"GetChainInfo", channelId},
	}
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}

	// 将需要查询的peer构造为map结构，加快查询效率
	mPeers := make(map[string][]*Peer, len(peers))
	for _, p := range peers {
		mPeers[p.name] = []*Peer{p}
	}
	r := sendToPeers(mPeers, sProposal, other)
	response := make([]*QueryChannelInfoResponse, 0, len(r))
	for _, pr := range r {
		peerResponse := QueryChannelInfoResponse{PeerName: pr.Name}
		if pr.Err != nil {
			peerResponse.Error = err
		} else {
			bci := new(common.BlockchainInfo)
			if err := proto.Unmarshal(pr.Response.Response.Payload, bci); err != nil {
				peerResponse.Error = err

			} else {
				peerResponse.Info = bci
			}
		}
		response = append(response, &peerResponse)
	}
	return response, nil

}

// query execute chainCode to one or many peers and return there responses without sending
// them to orderer for transaction - ReadOnly operation.
// Because is expected all peers to be in same state this function allows very easy horizontal scaling by
// distributing query operations between peers.
func (c *Client) query(identity Identity, chainCode ChainCode, peers []*Peer) (*peer.ProposalResponse, error) {
	logger.Debugf("enter query function")
	_, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}

	ch := make(chan *PeerResponse)
	defer close(ch)

	go sendToPeersSequentially(peers, sProposal, ch, other)
	resp := <-ch
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Response, nil
}

// querySinglePeer 对配置的多个peer进行冗余查询，只要有一个节点返回查询结构则返回
func querySinglePeer(proposal *peer.SignedProposal, peers []*Peer) (*peer.ProposalResponse, error) {
	logger.Debug("enter querySinglePeer function")
	defer logger.Debug("exit querySinglePeer function")

	ch := make(chan *PeerResponse)
	defer close(ch)

	go sendToPeersSequentially(peers, proposal, ch, other)
	resp := <-ch
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Response, nil
}

func getPeerQueryRes(identity Identity, chainCode ChainCode, crypto CryptoSuite, peers []*Peer) ([]*QueryResponse, error) {
	_, sProposal, err := createSignedProposal(identity, chainCode, crypto)
	if err != nil {
		logger.Errorf("create signed proposal failed: %v", err)
		return nil, err
	}

	// 将需要查询的peer构造为map结构，加快查询效率
	mPeers := make(map[string][]*Peer, len(peers))
	for _, p := range peers {
		mPeers[p.name] = []*Peer{p}
	}
	r := sendToPeers(mPeers, sProposal, other)
	response := make([]*QueryResponse, len(r))
	for idx, p := range r {
		ic := QueryResponse{PeerName: p.Name, Error: p.Err, Response: p.Response}
		response[idx] = &ic
	}
	return response, nil
}

// invoke execute chainCode for ledger update. Peers that simulate the chainCode must be enough to satisfy the policy.
// When Invoke returns with success this is not granite that ledger was update. Invoke will return `transactionId`.
// This ID will be transactionId in events.
// It is responsibility of SDK user to build logic that handle successful and failed commits.
// If chaincode call `shim.Error` or simulation fails for other reasons this is considered as simulation failure.
// In such case Invoke will return the error and transaction will NOT be send to orderer. This transaction will NOT be
// committed to blockchain.
func (c *Client) invoke(identity Identity, chainCode ChainCode, peers map[string][]*Peer) (*InvokeResponse, error) {
	proposal, sProposal, err := createSignedProposal(identity, chainCode, c.Crypto)
	if err != nil {
		return nil, err
	}
	propResponses := sendToPeers(peers, sProposal, other)
	transaction, err := createTransaction(proposal.proposal, propResponses)
	if err != nil {
		return nil, err
	}
	signedTransaction, err := c.Crypto.Sign(transaction, identity.PrivateKey)
	if err != nil {
		return nil, err
	}

	env := &common.Envelope{Payload: transaction, Signature: signedTransaction}
	reply, err := sendToOrderersBroadcast(c.Orderers, env)
	if err != nil {
		return nil, err
	}
	return &InvokeResponse{Status: reply.Status, TxID: proposal.transactionId, Payload: propResponses[0].Response.Response.Payload}, nil
}

// listenForFullBlock 监听区块
func (c *Client) listenForFullBlock(identity Identity, startNum uint64, channelId string, response chan<- pBlock.Block) chan error {
	var (
		err      error
		listener *EventListener
		blockNum uint64
		current  = c.current
		errRes   = make(chan error, 1)
	)

	if channelId == "" {
		errRes <- errors.New("the listened channel is nil")
		return errRes
	}

	listener = c.Listeners[current]
	go func() {
		defer func() {
			close(errRes)
		}()

		for {
			if listener == nil {
				errRes <- errors.New("all listeners are unavailable")
				return
			}

			if err = listener.newClient(EventTypeFullBlock); err != nil {
				logger.Warningf("create full block client failed: %v", err)
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = c.getNextListener()
				continue
			}

			if blockNum != 0 {
				startNum = blockNum + 1
			}

			if startNum == math.MaxUint64 {
				err = listener.SeekNewest(EventTypeFullBlock, channelId, identity, listener.peer, c.Crypto)
			} else {
				err = listener.SeekRange(startNum, math.MaxUint64, EventTypeFullBlock, channelId, identity, listener.peer, c.Crypto)
			}

			if err != nil {
				logger.Warningf("seek block number failed: %s", err.Error())
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = c.getNextListener()
				continue
			}

			blockNum, err = listener.listenEventFullBlock(response, c.Crypto)
			if err == ErrCloseEvent {
				// 用户主动退出event
				return
			}

			go listener.peer.waitRecovery()
			listener.peer.setAlive(false)
			listener = c.getNextListener()
		}
	}()

	return errRes
}

// listenForFilteredBlock 监听区块，此区块已被过滤
func (c *Client) listenForFilteredBlock(identity Identity, startNum uint64, channelId string, response chan<- FilteredBlockResponse) chan error {
	var (
		err      error
		listener *EventListener
		blockNum uint64
		current  = c.current
		errRes   = make(chan error, 1)
	)

	if channelId == "" {
		errRes <- errors.New("the listened channel is nil")
		return errRes
	}

	listener = c.Listeners[current]
	go func() {
		defer func() {
			close(errRes)
		}()

		for {
			if listener == nil {
				errRes <- errors.New("all listeners are unavailable")
				return
			}

			if err = listener.newClient(EventTypeFiltered); err != nil {
				logger.Warningf("create filter block client failed: %v", err)
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = c.getNextListener()
				continue
			}

			if blockNum != 0 {
				startNum = blockNum + 1
			}

			if startNum == math.MaxUint64 {
				err = listener.SeekNewest(EventTypeFiltered, channelId, identity, listener.peer, c.Crypto)
			} else {
				err = listener.SeekRange(uint64(startNum), math.MaxUint64, EventTypeFiltered, channelId, identity, listener.peer, c.Crypto)
			}
			if err != nil {
				logger.Warningf("seek block number failed: %s", err.Error())
				go listener.peer.waitRecovery()
				listener.peer.setAlive(false)
				listener = c.getNextListener()
				continue
			}

			blockNum, err = listener.listenEventFilterBlock(response)
			if err == ErrCloseEvent {
				// 用户主动退出event
				return
			}

			go listener.peer.waitRecovery()
			listener.peer.setAlive(false)
			listener = c.getNextListener()
		}
	}()

	return errRes
}

// newFabricClientFromConfig 根据配置生成fabric client
func newFabricClientFromConfig(config ClientConfig) (*Client, error) {
	var crypto CryptoSuite
	var err error

	crypto, err = newECCryptSuiteFromConfig(config.Crypto)
	if err != nil {
		return nil, err
	}

	peers := make(map[string]*Peer)
	orgPeers := make(map[string][]*Peer)
	for name, p := range config.Peers {
		newPeer, err := newPeer(p, crypto)
		if err != nil {
			return nil, errors.Errorf("new peer failed: %s", err.Error())
		}
		peers[name] = newPeer
		orgPeers[p.OrgName] = append(orgPeers[p.OrgName], newPeer)
	}

	listeners := make([]*EventListener, 0)
	for _, peerConfig := range config.EventPeers {
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

	orderers := make([]*Orderer, 0)
	for _, o := range config.Orderers {
		newOrderer, err := newOrderer(o, crypto)
		if err != nil {
			return nil, errors.Errorf("new orderer failed: %s", err.Error())
		}
		orderers = append(orderers, newOrderer)
	}
	ord := &OrderersInfo{orderers: orderers}
	client := Client{Peers: peers, orgPeers: orgPeers, Listeners: listeners, Orderers: ord, Crypto: crypto}
	return &client, nil
}

// parsePolicy 解析背书策略
// input： orgPeers以组织名称为key，以组织区分所有peer
// 		   peers以peer名称为key，包含所有peer且不进行区分
func parsePolicy(ccs []ChannelChaincodeConfig, orgPeers map[string][]*Peer, peers map[string]*Peer) (map[string][]*Peer, map[string]map[string]*endorsePeers, error) {
	l := len(ccs)
	if l == 0 {
		return nil, nil, errors.New("no cc is configured")
	}
	cCcPeers := make(map[string]map[string]*endorsePeers)
	channelPeers := make(map[string][]*Peer)
	// 对每个channel-chaincode的背书策略进行解析
	for _, cc := range ccs {
		if cc.ChaincodeType == ChaincodeSpec_UNDEFINED {
			return nil, nil, errors.Errorf("chaincode %s does not define chaincode type", cc.ChaincodeName)
		}
		var eps endorsePeers
		if cc.ChaincodePolicy.Rule == "and" {
			andPeers := make(map[string][]*Peer, len(cc.ChaincodePolicy.Orgs))
			for org, ps := range orgPeers {
				if containsStr(cc.ChaincodePolicy.Orgs, org) {
					andPeers[org] = ps
				}
			}
			eps.invokePeers = andPeers
		} else {
			orPeers := make([]*Peer, 0)
			for org, ps := range orgPeers {
				if containsStr(cc.ChaincodePolicy.Orgs, org) {
					orPeers = append(orPeers, ps...)
				}
			}
			eps.invokePeers = map[string][]*Peer{"or": orPeers}
		}

		for _, p := range peers {
			eps.queryPeers = append(eps.queryPeers, p)
		}
		eps.chanincodeType = cc.ChaincodeType

		andPeers := make(map[string][]*Peer, len(cc.ChaincodePolicy.Orgs))
		for org, ps := range orgPeers {
			if containsStr(cc.ChaincodePolicy.Orgs, org) {
				andPeers[org] = ps
			}
		}
		eps.allOrgs = andPeers

		if ccPeers, ok := cCcPeers[cc.ChannelId]; ok {
			ccPeers[cc.ChaincodeName] = &eps
		} else {
			ccPeers := make(map[string]*endorsePeers)
			ccPeers[cc.ChaincodeName] = &eps
			cCcPeers[cc.ChannelId] = ccPeers
		}
	}

	for channelID, ccPeers := range cCcPeers {
		// 针对单一通道，利用map结构查找到所有名称的peer，然后再转为数组结构
		mPeers := make(map[string]*Peer)
		for _, peers := range ccPeers {
			for _, p := range peers.queryPeers {
				if _, ok := mPeers[p.name]; !ok {
					mPeers[p.name] = p
				}
			}
		}
		aPeers := make([]*Peer, 0)
		for _, v := range mPeers {
			aPeers = append(aPeers, v)
		}
		channelPeers[channelID] = aPeers
	}

	return channelPeers, cCcPeers, nil
}

// 构建同步交易监听句柄
func createSyncTxEventHandle(channelPeerList map[string][]*Peer, config ChaincodeConfig, handler syncTxStatus.SyncTxHandler) error {
	for chanName, peerList := range channelPeerList {
		var chanPeerConfigList []PeerConfig
		var userConfig UserConfig
		for _, ucfg := range config.Users {
			// 取第一个为默认用户
			userConfig = ucfg
			break
		}
		// 找出对应channelName的peer配置列表
		for _, peerNode := range peerList {
			for _, pcfg := range config.PConfigs {
				if pcfg.OrgName == peerNode.orgName && pcfg.Name == peerNode.name {
					chanPeerConfigList = append(chanPeerConfigList, pcfg)
				}
			}
		}
		curEventCfg := EventConfig{
			CryptoConfig:  config.CryptoConfig,
			PeerConfigs:   chanPeerConfigList,
			UserConfig:    userConfig,
			CName:         chanName,
			RetryTimeout:  config.RetryTimeout,
			RetryInterval: config.RetryInterval,
			Peers:         peerList,
		}
		ec, err := NewEventClient(curEventCfg)
		if err != nil {
			return err
		}
		filterBlockChan := make(chan FilteredBlockResponse)
		errChan := ec.ListenEventFilterBlock(math.MaxUint64, filterBlockChan)
		go func() {
			for {
				select {
				case curErr, ok := <-errChan:
					if !ok {
						logger.Warningf("listen event func exit")
						return
					}
					logger.Errorf("createSyncTxEventHandle failed %s", curErr)
				case filterBlock := <-filterBlockChan:
					for _, tx := range filterBlock.Transactions {
						handler.PublishTxStatus(filterBlock.BlockNumber, tx.Id, tx.Status)
					}
				}
			}
		}()
	}
	return nil
}
