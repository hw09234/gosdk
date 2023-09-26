package gosdk

import (
	"context"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/hyperledger/fabric-protos-go/gossip"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric/gossip/protoext"
	utils "github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// ChannelPeer 通道中的所有peer
type ChannelPeer struct {
	MSPID        string
	LedgerHeight uint64
	Endpoint     string
	Identity     string
	Chaincodes   []string
}

// LocalPeer 本地peer
type LocalPeer struct {
	MSPID    string
	Endpoint string
	Identity string
}

// endorser 背书节点信息
type endorser struct {
	MSPID        string
	LedgerHeight uint64
	Endpoint     string
	Identity     string
}

// EndorsermentDescriptor 背书策略
type EndorsermentDescriptor struct {
	Chaincode         string
	EndorsersByGroups map[string][]endorser
	Layouts           []*discovery.Layout
}

// chaincodesAndCollections chaincode以及私有数据collection
type chaincodesAndCollections struct {
	Chaincodes  *[]string
	Collections *map[string]string
}

// newDiscoveryClient 根据配置信息构建discovery client
func newDiscoveryClient(ctx context.Context, n *node) (discovery.DiscoveryClient, *grpc.ClientConn, error) {
	c, err := grpc.DialContext(ctx, n.uri, n.opts...)
	if err != nil {
		return nil, nil, errors.Errorf("connect host %s failed: %s", n.uri, err.Error())
	}

	return discovery.NewDiscoveryClient(c), c, nil
}

// existsInChaincodes 判断chaincode是否存在
// input: chaincodeName: 智能合约名字
// output: bool: chaincode存在为true，不存在为false
func (ec *chaincodesAndCollections) existsInChaincodes(chaincodeName string) bool {
	for _, cc := range *ec.Chaincodes {
		if chaincodeName == cc {
			return true
		}
	}
	return false
}

// parseInput 解析chaincodesAndCollections
// output: map[string][]string: map的key为智能合约名字，value为collection的数组集合
func (ec *chaincodesAndCollections) parseInput() (map[string][]string, error) {
	var emptyChaincodes []string
	var emptyCollections map[string]string

	if ec.Chaincodes == nil {
		ec.Chaincodes = &emptyChaincodes
	}
	if ec.Collections == nil {
		ec.Collections = &emptyCollections
	}

	res := make(map[string][]string)

	for _, cc := range *ec.Chaincodes {
		res[cc] = nil
	}

	for cc, collections := range *ec.Collections {
		if !ec.existsInChaincodes(cc) {
			return nil, errors.Errorf("a collection specified chaincode %s but it wasn't specified with a chaincode flag", cc)
		}
		res[cc] = strings.Split(collections, ",")
	}
	return res, nil
}

type request func(channel, mspID string, chaincodes []string, collections map[string]string, cert, tlsCertHash []byte) (*discovery.Request, error)

func channelPeerRequest(channel, mspID string, chaincodes []string, collections map[string]string, cert, tlsCertHash []byte) (*discovery.Request, error) {
	var req discovery.Request
	var q discovery.Query
	q.Channel = channel
	q.Query = &discovery.Query_PeerQuery{
		PeerQuery: &discovery.PeerMembershipQuery{
			Filter: &discovery.ChaincodeInterest{},
		},
	}
	req.Queries = append(req.Queries, &q)

	sId, err := serializeIdentity(cert, mspID)
	if err != nil {
		return nil, errors.Errorf("serialize identity failed: %s", err.Error())
	}

	req.Authentication = &discovery.AuthInfo{ClientIdentity: sId, ClientTlsCertHash: tlsCertHash}

	return &req, nil
}

func localPeerRequest(channel, mspID string, chaincodes []string, collections map[string]string, cert, tlsCertHash []byte) (*discovery.Request, error) {
	var req discovery.Request

	q := &discovery.Query_LocalPeers{
		LocalPeers: &discovery.LocalPeerQuery{},
	}
	req.Queries = append(req.Queries, &discovery.Query{
		Query: q,
	})

	sId, err := serializeIdentity(cert, mspID)
	if err != nil {
		return nil, errors.Errorf("serialize identity failed: %s", err.Error())
	}

	req.Authentication = &discovery.AuthInfo{ClientIdentity: sId, ClientTlsCertHash: tlsCertHash}

	return &req, nil
}

func channelConfigRequest(channel, mspID string, chaincodes []string, collections map[string]string, cert, tlsCertHash []byte) (*discovery.Request, error) {
	var req discovery.Request
	var q discovery.Query
	q.Channel = channel
	q.Query = &discovery.Query_ConfigQuery{
		ConfigQuery: &discovery.ConfigQuery{},
	}
	req.Queries = append(req.Queries, &q)

	sId, err := serializeIdentity(cert, mspID)
	if err != nil {
		return nil, errors.Errorf("serialize identity failed: %s", err.Error())
	}

	req.Authentication = &discovery.AuthInfo{ClientIdentity: sId, ClientTlsCertHash: tlsCertHash}

	return &req, nil
}

func endorsePolicyRequest(channel, mspID string, chaincodes []string, collections map[string]string, cert, tlsCertHash []byte) (*discovery.Request, error) {
	var req discovery.Request
	var q discovery.Query

	ccAndCol := &chaincodesAndCollections{
		Chaincodes:  &chaincodes,
		Collections: &collections,
	}
	cc2collections, err := ccAndCol.parseInput()
	if err != nil {
		return nil, errors.WithMessage(err, "parse chaincode and collection failed")
	}

	var ccCalls []*discovery.ChaincodeCall
	for _, cc := range *ccAndCol.Chaincodes {
		ccCalls = append(ccCalls, &discovery.ChaincodeCall{
			Name:            cc,
			CollectionNames: cc2collections[cc],
		})
	}

	var ccInterest []*discovery.ChaincodeInterest
	ccInterest = append(ccInterest, &discovery.ChaincodeInterest{Chaincodes: ccCalls})

	q.Channel = channel
	q.Query = &discovery.Query_CcQuery{
		CcQuery: &discovery.ChaincodeQuery{
			Interests: ccInterest,
		},
	}
	req.Queries = append(req.Queries, &q)

	sId, err := serializeIdentity(cert, mspID)
	if err != nil {
		return nil, errors.Errorf("serialize identity failed: %s", err.Error())
	}

	req.Authentication = &discovery.AuthInfo{ClientIdentity: sId, ClientTlsCertHash: tlsCertHash}

	return &req, nil
}

// rawPeerToChannelPeer 解析discovery发现的peer
// output: ChannelPeer: 解析后的peer信息
func toChannelPeer(mspID string, identity []byte, aliveMessage, stateInfoMessage *protoext.SignedGossipMessage) (ChannelPeer, error) {
	if aliveMessage == nil || stateInfoMessage == nil {
		return ChannelPeer{}, errors.New("aliveMessage or stateInfoMessage is nil")
	}
	var ccs []string
	var endpoint string

	if stateInfoMessage.GetStateInfo() == nil || stateInfoMessage.GetStateInfo().Properties == nil {
		return ChannelPeer{}, errors.New("get properties from stateInfoMessage is nil")
	}
	properties := stateInfoMessage.GetStateInfo().Properties
	ledgerHeight := properties.LedgerHeight
	for _, cc := range properties.Chaincodes {
		if cc == nil {
			continue
		}
		ccs = append(ccs, cc.Name)
	}

	if aliveMessage.GetAliveMsg() == nil || aliveMessage.GetAliveMsg().Membership == nil {
		return ChannelPeer{}, errors.New("memberShip from aliveMessage is nil")
	}
	endpoint = aliveMessage.GetAliveMsg().Membership.Endpoint

	sID := &msp.SerializedIdentity{}
	err := proto.Unmarshal(identity, sID)
	if err != nil {
		return ChannelPeer{}, errors.WithMessage(err, "unmarshal peer identity failed")
	}

	return ChannelPeer{
		MSPID:        mspID,
		Endpoint:     endpoint,
		LedgerHeight: ledgerHeight,
		Identity:     string(sID.IdBytes),
		Chaincodes:   ccs,
	}, nil
}

func toLocalPeer(mspID string, identity []byte, aliveMessage *protoext.SignedGossipMessage) (LocalPeer, error) {
	if aliveMessage == nil {
		return LocalPeer{}, errors.New("aliveMessage or stateInfoMessage is nil")
	}
	if aliveMessage.GetAliveMsg() == nil || aliveMessage.GetAliveMsg().Membership == nil {
		return LocalPeer{}, errors.New("memberShip from aliveMessage is nil")
	}
	endpoint := aliveMessage.GetAliveMsg().Membership.Endpoint

	sID := &msp.SerializedIdentity{}
	err := proto.Unmarshal(identity, sID)
	if err != nil {
		return LocalPeer{}, errors.WithMessage(err, "unmarshal peer identity failed")
	}

	return LocalPeer{
		MSPID:    mspID,
		Endpoint: endpoint,
		Identity: string(sID.IdBytes),
	}, nil
}

// parseEndorsementDescriptors 解析背书策略信息
// input: []*discovery.EndorsementDescriptor: discovery服务发现的背书策略，要进行解析
// output: []EndorsermentDescriptor: 解析后的背书策略
func parseEndorsementDescriptors(descriptors []*discovery.EndorsementDescriptor) ([]EndorsermentDescriptor, error) {
	var res []EndorsermentDescriptor

	for _, desc := range descriptors {
		endorsersByGroups := make(map[string][]endorser)

		for grp, endorsers := range desc.EndorsersByGroups {
			for _, p := range endorsers.Peers {
				e, err := endorserFromRaw(p)
				if err != nil {
					return nil, err
				}
				endorsersByGroups[grp] = append(endorsersByGroups[grp], e)
			}
		}

		res = append(res, EndorsermentDescriptor{
			Chaincode:         desc.Chaincode,
			Layouts:           desc.Layouts,
			EndorsersByGroups: endorsersByGroups,
		})
	}

	return res, nil
}

func endorserFromRaw(p *discovery.Peer) (endorser, error) {
	sId := &msp.SerializedIdentity{}

	err := proto.Unmarshal(p.Identity, sId)
	if err != nil {
		return endorser{}, errors.WithMessage(err, "unmarshal peer identity failed")
	}

	return endorser{
		MSPID:        sId.Mspid,
		Endpoint:     endpointFromEnvelope(p.MembershipInfo),
		LedgerHeight: ledgerHeightFromEnvelope(p.StateInfo),
		Identity:     string(sId.IdBytes),
	}, nil
}

func endpointFromEnvelope(env *gossip.Envelope) string {
	if env == nil {
		return ""
	}

	aliveMsg, _ := protoext.EnvelopeToGossipMessage(env)
	if aliveMsg == nil {
		return ""
	}

	if !protoext.IsAliveMsg(aliveMsg.GossipMessage) {
		return ""
	}

	if aliveMsg.GetAliveMsg().Membership == nil {
		return ""
	}

	return aliveMsg.GetAliveMsg().Membership.Endpoint
}

func ledgerHeightFromEnvelope(env *gossip.Envelope) uint64 {
	if env == nil {
		return 0
	}

	stateInfoMsg, _ := protoext.EnvelopeToGossipMessage(env)
	if stateInfoMsg == nil {
		return 0
	}

	if !protoext.IsStateInfoMsg(stateInfoMsg.GossipMessage) {
		return 0
	}

	if stateInfoMsg.GetStateInfo().Properties == nil {
		return 0
	}

	return stateInfoMsg.GetStateInfo().Properties.LedgerHeight
}

func serializeIdentity(identity []byte, mspID string) ([]byte, error) {
	sId := &msp.SerializedIdentity{
		Mspid:   mspID,
		IdBytes: identity,
	}
	return utils.MarshalOrPanic(sId), nil
}

func validateStateInfoMessage(message *protoext.SignedGossipMessage) error {
	si := message.GetStateInfo()
	if si == nil {
		return errors.New("message isn't a stateInfo message")
	}
	if si.Timestamp == nil {
		return errors.New("timestamp is nil")
	}
	if si.Properties == nil {
		return errors.New("properties is nil")
	}
	return nil
}

func validateAliveMessage(message *protoext.SignedGossipMessage) error {
	am := message.GetAliveMsg()
	if am == nil {
		return errors.New("message isn't an alive message")
	}
	m := am.Membership
	if m == nil {
		return errors.New("membership is empty")
	}
	if am.Timestamp == nil {
		return errors.New("timestamp is nil")
	}
	return nil
}

// parseChannelPeers 将查询结果转换为ChannelPeer结构
func parseChannelPeers(r *discovery.QueryResult) ([]ChannelPeer, error) {
	var cps []ChannelPeer

	if err := r.GetError(); err != nil {
		return nil, errors.Errorf("get error for discovery peer members: %s", err.Content)
	}
	membersRes := r.GetMembers()
	if membersRes == nil {
		return nil, errors.Errorf("expected QueryResult of either PeerMembershipResult or Error but got %v instead", r)
	}

	for org, peersOfCurrentOrg := range membersRes.PeersByOrg {
		for _, peer := range peersOfCurrentOrg.Peers {
			aliveMsg, err := protoext.EnvelopeToGossipMessage(peer.MembershipInfo)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshaling alive message")
			}
			var stateInfoMsg *protoext.SignedGossipMessage
			stateInfoMsg, err = protoext.EnvelopeToGossipMessage(peer.StateInfo)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshaling stateInfo message")
			}
			if err := validateStateInfoMessage(stateInfoMsg); err != nil {
				return nil, errors.Wrap(err, "failed validating stateInfo message")
			}
			if err := validateAliveMessage(aliveMsg); err != nil {
				return nil, errors.Wrap(err, "failed validating alive message")
			}

			cp, err := toChannelPeer(org, peer.Identity, aliveMsg, stateInfoMsg)
			if err != nil {
				return nil, errors.Wrap(err, "failed converting to ChannelPeer")
			}
			cps = append(cps, cp)
		}
	}

	return cps, nil
}

// parseLocalPeer 将查询结果转换为LocalPeer结构
func parseLocalPeer(r *discovery.QueryResult) ([]LocalPeer, error) {
	var lps []LocalPeer

	if err := r.GetError(); err != nil {
		return nil, errors.Errorf("get error for discovery peer members: %s", err.Content)
	}
	membersRes := r.GetMembers()
	if membersRes == nil {
		return nil, errors.Errorf("expected QueryResult of either PeerMembershipResult or Error but got %v instead", r)
	}

	for org, peersOfCurrentOrg := range membersRes.PeersByOrg {
		for _, peer := range peersOfCurrentOrg.Peers {
			aliveMsg, err := protoext.EnvelopeToGossipMessage(peer.MembershipInfo)
			if err != nil {
				return nil, errors.Wrap(err, "failed unmarshaling alive message")
			}
			if err := validateAliveMessage(aliveMsg); err != nil {
				return nil, errors.Wrap(err, "failed validating alive message")
			}

			lp, err := toLocalPeer(org, peer.Identity, aliveMsg)
			if err != nil {
				return nil, errors.Wrap(err, "failed converting to LocalPeer")
			}
			lps = append(lps, lp)
		}
	}

	return lps, nil
}

func parseChannelConfig(r *discovery.QueryResult) (*discovery.ConfigResult, error) {
	if err := r.GetError(); err != nil {
		return nil, errors.Errorf("get error for discovery channel config: %s", err.Content)
	}
	configRes := r.GetConfigResult()
	if configRes == nil {
		return nil, errors.Errorf("expected QueryResult of either ConfigResult or Error but got %v instead", r)
	}

	return configRes, nil
}

func parseEndorsePolicy(r *discovery.QueryResult) ([]EndorsermentDescriptor, error) {
	if err := r.GetError(); err != nil {
		return nil, errors.Errorf("get error for discovery endorse policy: %s", err.Content)
	}
	ccRes := r.GetCcQueryRes()
	if ccRes == nil {
		return nil, errors.Errorf("expected QueryResult of either ConfigResult or Error but got %v instead", r)
	}

	endorser, err := parseEndorsementDescriptors(ccRes.Content)
	if err != nil {
		return nil, errors.WithMessage(err, "parse endorser failed")
	}

	return endorser, nil
}

// signDiscoveryRequest 对discovery请求进行签名，组装为SignedRequest结构
func signDiscoveryRequest(r *discovery.Request, crypto CryptoSuite, key interface{}) (*discovery.SignedRequest, error) {
	if r == nil {
		return nil, errors.New("request is nil")
	}

	payload, err := proto.Marshal(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling Request to bytes")
	}
	sig, err := crypto.Sign(payload, key)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to sign payload")
	}

	return &discovery.SignedRequest{
		Payload:   payload,
		Signature: sig,
	}, nil
}

// sendDiscoverySignedRequest 连接peer并发送签名的discovery请求
// 在退出函数时会关闭与peer建立的连接
func sendDiscoverySignedRequest(sr *discovery.SignedRequest, p *node) (*discovery.Response, error) {
	client, cc, err := newDiscoveryClient(context.Background(), p)
	if err != nil {
		return nil, errors.Errorf("connect to peer %s failed: %s", p.uri, err.Error())
	}
	defer cc.Close()
	resp, err := client.Discover(context.Background(), sr)
	if err != nil {
		return nil, errors.Errorf("discover peer failed: %s", err.Error())
	}

	return resp, nil
}
