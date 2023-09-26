package gohfc

import (
	"github.com/hyperledger/fabric-protos-go/discovery"
	"github.com/pkg/errors"
)

type DiscoveryClientImpl struct {
	CryptoSuite
	*Identity
	Peers []*node
}

func (d *DiscoveryClientImpl) DiscoveryPeers(channel string) ([]ChannelPeer, error) {
	logger.Debug("enter discover channel peers progress")
	defer logger.Debug("exit discover channel peers progress")

	req := channelPeerRequest
	resp, err := d.getDiscoveryResponse(req, channel, nil, nil)
	if err != nil {
		logger.Errorf("discover failed: %v", err)
		return nil, err
	}

	cps, err := parseChannelPeers(resp.Results[0])
	if err != nil {
		return nil, errors.Errorf("parse response to channel peers failed: %s", err.Error())
	}

	return cps, nil
}

// DiscoveryChannelPeers 发现通道中所有的peer
// 返回结构体为gohfc定义的结构体，gohfc获取discovery服务返回结果后进行了解析构造
func (d *DiscoveryClientImpl) DiscoveryChannelPeers(channel string) ([]ChannelPeer, error) {
	logger.Debug("enter discover channel peers progress")
	defer logger.Debug("exit discover channel peers progress")

	req := channelPeerRequest
	resp, err := d.getDiscoveryResponse(req, channel, nil, nil)
	if err != nil {
		logger.Errorf("discover failed: %v", err)
		return nil, err
	}

	cps, err := parseChannelPeers(resp.Results[0])
	if err != nil {
		return nil, errors.Errorf("parse response to channel peers failed: %s", err.Error())
	}

	return cps, nil
}

// DiscoveryChannelPeers 发现本地peer
// 注意调用此函数，身份证书需为admin类型
func (d *DiscoveryClientImpl) DiscoveryLocalPeers() ([]LocalPeer, error) {
	logger.Debug("enter discover local peers progress")
	defer logger.Debug("exit discover local peers progress")

	req := localPeerRequest
	resp, err := d.getDiscoveryResponse(req, "", nil, nil)
	if err != nil {
		logger.Errorf("discover failed: %v", err)
		return nil, err
	}

	lps, err := parseLocalPeer(resp.Results[0])
	if err != nil {
		return nil, errors.Errorf("parse response to local peers failed: %s", err.Error())
	}

	return lps, nil
}

// DiscoveryChannelPeers 发现通道配置
// input: channel: 通道名字
// output: *discovery.ConfigResult: 得到通道的配置
func (d *DiscoveryClientImpl) DiscoveryChannelConfig(channel string) (*discovery.ConfigResult, error) {
	logger.Debug("enter discover channel config progress")
	defer logger.Debug("exit discover channel config progress")

	req := channelConfigRequest
	resp, err := d.getDiscoveryResponse(req, channel, nil, nil)
	if err != nil {
		logger.Errorf("discover failed: %v", err)
		return nil, err
	}

	cc, err := parseChannelConfig(resp.Results[0])
	if err != nil {
		return nil, errors.Errorf("parse response to channel config failed: %s", err.Error())
	}

	return cc, nil
}

// DiscoveryEndorsePolicy 查询多个chaincode和私有数据的collection的背书策略
// intput: channel: 通道名字
//         chaincodes 智能合约名字数组
//         collections: 私有数据collection集合(可以为空)，key为chaincode name，value为collection name,多个collection以","分割，例如map["mycc"]="collection1,collection2"
// output: []EndorsermentDescriptor: 得到的背书策略数组
func (d *DiscoveryClientImpl) DiscoveryEndorsePolicy(channel string, chaincodes []string, collections map[string]string) ([]EndorsermentDescriptor, error) {
	logger.Debug("enter discover endorse policy config progress")
	defer logger.Debug("exit discover endorse policy progress")

	req := endorsePolicyRequest
	resp, err := d.getDiscoveryResponse(req, channel, chaincodes, collections)
	if err != nil {
		logger.Errorf("discover failed: %v", err)
		return nil, err
	}

	ep, err := parseEndorsePolicy(resp.Results[0])
	if err != nil {
		return nil, errors.Errorf("parse response to endorse policy failed: %s", err.Error())
	}

	return ep, nil
}

func discover(req request, d *DiscoveryClientImpl, peer *node, channel string, chaincodes []string, collections map[string]string) (*discovery.Response, error) {
	r, err := req(channel, d.MspId, chaincodes, collections, d.Cert, peer.tlsCertHash)
	if err != nil {
		return nil, errors.Errorf("construct endorsement request failed: %s", err.Error())
	}

	sr, err := signDiscoveryRequest(r, d.CryptoSuite, d.PrivateKey)
	if err != nil {
		return nil, errors.Errorf("sign discovery request failed: %s", err.Error())
	}

	resp, err := sendDiscoverySignedRequest(sr, peer)
	if err != nil {
		return nil, errors.Errorf("send signed discovery request failed: %s", err.Error())
	}
	if len(resp.Results) != 1 {
		return nil, ErrResponse
	}

	return resp, nil
}

func (d *DiscoveryClientImpl) getDiscoveryResponse(req request, channel string, chaincodes []string, collections map[string]string) (resp *discovery.Response, err error) {
	for _, peer := range d.Peers {
		resp, err = discover(req, d, peer, channel, chaincodes, collections)
		if err == ErrResponse || err == nil {
			return
		}
		logger.Errorf("peer %s discover failed: %s", peer.uri, err.Error())
	}
	return
}

func (d *DiscoveryClientImpl) GetCrypto() (CryptoSuite, error) {
	if d.CryptoSuite == nil {
		return nil, errors.New("crypto suite is nil")
	}

	return d.CryptoSuite, nil
}
