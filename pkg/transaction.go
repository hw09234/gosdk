/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
)

// TransactionId fabric交易ID
type TransactionId struct {
	Nonce         []byte
	TransactionId string
	Creator       []byte
}

// QueryResponse 查询返回
type QueryResponse struct {
	PeerName string
	Error    error
	Response *peer.ProposalResponse
}

// InvokeResponse invoke返回
type InvokeResponse struct {
	Status common.Status
	// TxID is transaction id. This id can be used to track transactions and their status
	TxID    string
	Payload []byte
}

// SyncInvokeResponse 同步交易返回
type SyncInvokeResponse struct {
	Status common.Status
	// TxID is transaction id. This id can be used to track transactions and their status
	TxID        string
	BlockNumber uint64
	TxStatus    peer.TxValidationCode
	Payload     []byte
}

type transactionProposal struct {
	proposal      []byte
	transactionId string
}

// marshalProtoIdentity 根据证书和MSPID序列化identity
func marshalProtoIdentity(identity Identity) ([]byte, error) {
	if len(identity.MspId) < 1 {
		logger.Errorf("MspId does not exited")
		return nil, ErrMspMissing
	}
	creator, err := proto.Marshal(&msp.SerializedIdentity{
		Mspid:   identity.MspId,
		IdBytes: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: identity.Certificate.Raw})})
	if err != nil {
		logger.Errorf("proto marshal serialized identity failed: %v", err)
		return nil, err
	}
	return creator, nil
}

// signatureHeader 根据creator和nonce创建新的签名头
func signatureHeader(creator []byte, tx *TransactionId) ([]byte, error) {
	sh := new(common.SignatureHeader)
	sh.Creator = creator
	sh.Nonce = tx.Nonce
	shBytes, err := proto.Marshal(sh)
	if err != nil {
		logger.Errorf("proto marshal signature header failed: %v", err)
		return nil, err
	}
	return shBytes, nil
}

// header 根据channel header 和signture header创建header
func header(signatureHeader, channelHeader []byte) *common.Header {
	header := new(common.Header)
	header.SignatureHeader = signatureHeader
	header.ChannelHeader = channelHeader
	return header
}

// channelHeader 创建新的channel header
func channelHeader(headerType common.HeaderType, tx *TransactionId, channelId string, epoch uint64, extension *peer.ChaincodeHeaderExtension) ([]byte, error) {
	logger.Debugf("using channel %s", channelId)

	ts, err := ptypes.TimestampProto(time.Now())
	if err != nil {
		logger.Errorf("get timestamp failed: %v", err)
		return nil, err
	}

	payloadChannelHeader := &common.ChannelHeader{
		Type:      int32(headerType),
		Version:   1,
		Timestamp: ts,
		ChannelId: channelId,
		Epoch:     epoch,
	}
	payloadChannelHeader.TxId = tx.TransactionId
	if extension != nil {
		serExt, err := proto.Marshal(extension)
		if err != nil {
			logger.Errorf("proto marshal channel header extension failed: %v", err)
			return nil, err
		}
		payloadChannelHeader.Extension = serExt
	}
	return proto.Marshal(payloadChannelHeader)
}

// payload 根据commonHeader和envelope data创建payload
func payload(header *common.Header, data []byte) ([]byte, error) {
	p := new(common.Payload)
	p.Header = header
	p.Data = data
	return proto.Marshal(p)
}

// newTransactionId 生成交易ID
func newTransactionId(creator []byte, crypto CryptoSuite) (*TransactionId, error) {
	nonce, err := generateRandomBytes(24)
	if err != nil {
		logger.Errorf("generate random bytes failed: %v", err)
		return nil, err
	}
	id := generateTxId(nonce, creator, crypto)
	return &TransactionId{Creator: creator, Nonce: nonce, TransactionId: id}, nil
}

// generateRandomBytes 创建随机的nonce
func generateRandomBytes(len int) ([]byte, error) {
	b := make([]byte, len)
	_, err := rand.Read(b)
	if err != nil {
		logger.Errorf("failed to get random number: %v", err)
		return nil, err
	}
	return b, nil
}

// generateTxId 进行sha256编码
func generateTxId(nonce, creator []byte, crypto CryptoSuite) string {
	return hex.EncodeToString(crypto.Hash(append(nonce, creator...)))
}

func chainCodeInvocationSpec(chainCode ChainCode) ([]byte, error) {
	invocation := &peer.ChaincodeInvocationSpec{
		ChaincodeSpec: &peer.ChaincodeSpec{
			Type:        peer.ChaincodeSpec_Type(chainCode.Type),
			ChaincodeId: &peer.ChaincodeID{Name: chainCode.Name},
			Input:       &peer.ChaincodeInput{Args: chainCode.toChainCodeArgs(), IsInit: chainCode.IsInit},
		},
	}
	invocationBytes, err := proto.Marshal(invocation)
	if err != nil {
		logger.Errorf("proto marshal chaincode invocation spec failed: %v", err)
		return nil, err
	}
	return invocationBytes, nil
}

func chainCodeDeploymentSpec(chainCode ChainCode) ([]byte, error) {
	depSpec, err := proto.Marshal(&peer.ChaincodeDeploymentSpec{
		ChaincodeSpec: &peer.ChaincodeSpec{
			ChaincodeId: &peer.ChaincodeID{Name: chainCode.Name, Version: chainCode.Version},
			Type:        peer.ChaincodeSpec_Type(chainCode.Type),
			Input:       &peer.ChaincodeInput{Args: chainCode.toChainCodeArgs()},
		},
	})
	if err != nil {
		logger.Errorf("proto marshal chaincode deployment spec failed: %v", err)
		return nil, err
	}

	return depSpec, nil
}

func proposal(header, payload []byte) ([]byte, error) {
	prop := new(peer.Proposal)
	prop.Header = header
	prop.Payload = payload

	propBytes, err := proto.Marshal(prop)
	if err != nil {
		logger.Errorf("proto marshal proposal failed: %v", err)
		return nil, err
	}
	return propBytes, nil
}

func signedProposal(prop []byte, identity Identity, crypt CryptoSuite) (*peer.SignedProposal, error) {
	sb, err := crypt.Sign(prop, identity.PrivateKey)
	if err != nil {
		logger.Errorf("get signature failed: %v", err)
		return nil, err
	}
	return &peer.SignedProposal{ProposalBytes: prop, Signature: sb}, nil
}

// sendToPeers 向peers发送proposal，并等待响应。返回结果顺序和发送顺序并无关系。
func sendToPeers(peers map[string][]*Peer, prop *peer.SignedProposal, endorsetype endorseType) []*PeerResponse {
	logger.Debug("enter multi groups' peers endorse progress")
	defer logger.Debug("exit multi groups' peers endorse progress")

	ch := make(chan *PeerResponse)
	defer close(ch)
	l := len(peers)
	logger.Debugf("try to get %d endorse responses form peer", l)
	resp := make([]*PeerResponse, 0, l)
	for _, eps := range peers {
		go sendToPeersSequentially(eps, prop, ch, endorsetype)
	}
	for i := 0; i < l; i++ {
		resp = append(resp, <-ch)
	}
	return resp
}

// sendToPeersSequentially 对peer数组顺序发送，只要有成功的即返回
func sendToPeersSequentially(peers []*Peer, prop *peer.SignedProposal, resp chan *PeerResponse, endorsetype endorseType) {
	logger.Debug("enter peers endorse progress")
	defer logger.Debug("exit peers endorse progress")

	if len(peers) == 0 {
		logger.Error("endorse peer is empty")
		resp <- &PeerResponse{Err: errors.New("endorse peer is empty")}
	}
	for _, p := range peers {
		logger.Debugf("peer %s try to endorse", p.uri)
		if !p.isAlive() {
			continue
		}
		res, err := p.Endorse(prop, endorsetype)
		if err == nil {
			resp <- &PeerResponse{Response: res, Name: p.name, Err: nil}
			logger.Debugf("peer %s complete endorse", p.uri)
			return
		}
		logger.Errorf("peer %s endorse failed: %s", p.uri, err.Error())
		if strings.Contains(err.Error(), "access denied") {
			continue
		}
		p.setAlive(false)
		go p.waitRecovery()
	}
	resp <- &PeerResponse{Err: errors.New("all peers endorse failed")}
}

func createSignedProposal(identity Identity, cc ChainCode, crypto CryptoSuite) (*transactionProposal, *peer.SignedProposal, error) {
	proposal, err := createTransactionProposal(identity, cc, crypto)
	if err != nil {
		logger.Errorf("create transaction proposal failed: %v", err)
		return nil, nil, err
	}

	signedProposal, err := signedProposal(proposal.proposal, identity, crypto)
	if err != nil {
		logger.Errorf("get signed proposal failed: %v", err)
		return nil, nil, err
	}

	return proposal, signedProposal, nil
}

func createTransactionProposal(identity Identity, cc ChainCode, crypto CryptoSuite) (*transactionProposal, error) {
	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		logger.Errorf("get creator bytes failed: %v", err)
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		logger.Errorf("get transaction id failed: %v", err)
		return nil, err
	}
	logger.Debugf("get txid %s", txId.TransactionId)

	spec, err := chainCodeInvocationSpec(cc)
	if err != nil {
		logger.Errorf("get invocation spec bytes failed: %v", err)
		return nil, err
	}
	extension := &peer.ChaincodeHeaderExtension{ChaincodeId: &peer.ChaincodeID{Name: cc.Name}}
	channelHeader, err := channelHeader(common.HeaderType_ENDORSER_TRANSACTION, txId, cc.ChannelId, 0, extension)
	if err != nil {
		logger.Errorf("get channel header bytes failed: %v", err)
		return nil, err
	}
	signatureHeader, err := signatureHeader(creator, txId)
	if err != nil {
		logger.Errorf("get header signature bytes failed: %v", err)
		return nil, err
	}

	proposalPayload, err := proto.Marshal(&peer.ChaincodeProposalPayload{Input: spec, TransientMap: cc.TransientMap})
	if err != nil {
		return nil, err
	}

	header, err := proto.Marshal(header(signatureHeader, channelHeader))
	if err != nil {
		logger.Errorf("proto marshal header failed: %v", err)
		return nil, err
	}

	proposal, err := proposal(header, proposalPayload)
	if err != nil {
		logger.Errorf("get proposal bytes failed: %v", err)
		return nil, err
	}
	return &transactionProposal{proposal: proposal, transactionId: txId.TransactionId}, nil
}

func decodeChainCodeQueryResponse(data []byte) ([]*peer.ChaincodeInfo, error) {
	response := new(peer.ChaincodeQueryResponse)
	err := proto.Unmarshal(data, response)
	if err != nil {
		return nil, err
	}
	return response.GetChaincodes(), nil
}

func createTransaction(proposal []byte, endorsement []*PeerResponse) ([]byte, error) {
	var propResp *peer.ProposalResponse
	var pl []byte
	mEndorsements := make([]*peer.Endorsement, 0, len(endorsement))
	for _, e := range endorsement {
		if e.Err != nil {
			logger.Errorf("one of the endorser failed: %v", e.Err)
			return nil, e.Err
		} else if e.Response.Response.Status == 200 {
			propResp = e.Response
			mEndorsements = append(mEndorsements, e.Response.Endorsement)
			if pl == nil {
				pl = e.Response.Payload
			}
		} else {
			return nil, errors.WithMessage(ErrBadTransactionStatus, e.Response.Response.Message)
		}
		if bytes.Compare(pl, e.Response.Payload) != 0 {
			logger.Error(ErrEndorsementsDoNotMatch)
			return nil, ErrEndorsementsDoNotMatch
		}
	}

	// at least one is OK
	if len(mEndorsements) < 1 {
		logger.Error(ErrNoValidEndorsementFound)
		return nil, ErrNoValidEndorsementFound
	}

	originalProposal, err := getProposal(proposal)
	if err != nil {
		logger.Errorf("get proposal failed: %v", err)
		return nil, err
	}

	originalProposalHeader, err := getHeader(originalProposal.Header)
	if err != nil {
		logger.Errorf("get header failed: %v", err)
		return nil, err
	}

	originalProposalPayload, err := getChainCodeProposalPayload(originalProposal.Payload)
	if err != nil {
		logger.Errorf("get chaincode proposal payload failed: %v", err)
		return nil, err
	}

	// create actual invocation
	proposedPayload, err := proto.Marshal(&peer.ChaincodeProposalPayload{Input: originalProposalPayload.Input, TransientMap: nil})
	if err != nil {
		logger.Errorf("proto marshal chaincode proposal payload failed: %v", err)
		return nil, err
	}

	payload, err := proto.Marshal(&peer.ChaincodeActionPayload{
		Action: &peer.ChaincodeEndorsedAction{
			ProposalResponsePayload: propResp.Payload,
			Endorsements:            mEndorsements,
		},
		ChaincodeProposalPayload: proposedPayload,
	})
	if err != nil {
		logger.Errorf("proto marshal chaincode action payload failed: %v", err)
		return nil, err
	}

	sTransaction, err := proto.Marshal(&peer.Transaction{
		Actions: []*peer.TransactionAction{{Header: originalProposalHeader.SignatureHeader, Payload: payload}},
	})
	if err != nil {
		logger.Errorf("proto marshal transaction failed: %v", err)
		return nil, err
	}

	propBytes, err := proto.Marshal(&common.Payload{Header: originalProposalHeader, Data: sTransaction})
	if err != nil {
		logger.Errorf("proto marshal payload failed: %v", err)
		return nil, err
	}
	return propBytes, nil
}

func createEnvelope(proposal []byte, endorsement []*PeerResponse, identity Identity, crypto CryptoSuite) (*common.Envelope, error) {
	transaction, err := createTransaction(proposal, endorsement)
	if err != nil {
		return nil, err
	}
	signedTransaction, err := crypto.Sign(transaction, identity.PrivateKey)
	if err != nil {
		return nil, err
	}
	env := &common.Envelope{Payload: transaction, Signature: signedTransaction}

	return env, nil
}

func getProposal(data []byte) (*peer.Proposal, error) {
	prop := new(peer.Proposal)
	err := proto.Unmarshal(data, prop)
	if err != nil {
		logger.Errorf("unmarshal proposal failed: %v", err)
		return nil, err
	}
	return prop, nil
}

func getHeader(bytes []byte) (*common.Header, error) {
	h := &common.Header{}
	err := proto.Unmarshal(bytes, h)
	if err != nil {
		logger.Errorf("unmarshal header failed: %v", err)
		return nil, err
	}
	return h, err
}

func getChainCodeProposalPayload(bytes []byte) (*peer.ChaincodeProposalPayload, error) {
	cpp := &peer.ChaincodeProposalPayload{}
	err := proto.Unmarshal(bytes, cpp)
	if err != nil {
		logger.Errorf("unmarshal chaincode proposal payload failed: %v", err)
		return nil, err
	}
	return cpp, err
}

// parseHeight 从QueryResponse结构体中解析出区块高度
func parseHeight(resp *peer.ProposalResponse) (uint64, error) {
	if resp == nil || resp.Response == nil || resp.Response.Payload == nil {
		return 0, errors.New("payload is nil")
	}

	chainInfo := new(common.BlockchainInfo)
	if err := proto.Unmarshal(resp.Response.Payload, chainInfo); err != nil {
		return 0, errors.WithMessage(err, "unmarshal block chain info failed")
	}

	return chainInfo.Height, nil
}

// parseBlock 从QueryResponse结构体中解析出Block
func parseBlock(resp *peer.ProposalResponse) (*common.Block, error) {
	if resp.Response.Payload == nil {
		return nil, errors.New("payload is nil")
	}

	block := new(common.Block)
	if err := proto.Unmarshal(resp.Response.Payload, block); err != nil {
		return nil, errors.WithMessage(err, "unmarshal block failed")
	}

	return block, nil
}

// parseTX 从QueryResponse结构体中解析出交易结构体
func parseTX(resp *peer.ProposalResponse) (*peer.ProcessedTransaction, error) {
	if resp.Response.Payload == nil {
		return nil, errors.New("payload is nil")
	}

	transaction := new(peer.ProcessedTransaction)
	if err := proto.Unmarshal(resp.Response.Payload, transaction); err != nil {
		return nil, errors.WithMessage(err, "unmarshal transaction failed")
	}

	return transaction, nil
}
