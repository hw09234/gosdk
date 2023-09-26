package gohfc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func initClient() *Client {
	org1Peer0 := &Peer{node: node{name: "org1peer0", orgName: "org1"}}
	org1Peer1 := &Peer{node: node{name: "org1peer1", orgName: "org1"}}
	org2Peer0 := &Peer{node: node{name: "org2peer0", orgName: "org2"}}
	org2Peer1 := &Peer{node: node{name: "org2peer1", orgName: "org2"}}
	org2Peer2 := &Peer{node: node{name: "org2peer2", orgName: "org2"}}
	org3Peer0 := &Peer{node: node{name: "org3peer0", orgName: "org3"}}
	client := &Client{
		Peers: map[string]*Peer{
			"org1peer0": org1Peer0,
			"org1peer1": org1Peer1,
			"org2peer0": org2Peer0,
			"org2peer1": org2Peer1,
			"org2peer2": org2Peer2,
			"org3Peer0": org3Peer0,
		},
		orgPeers: map[string][]*Peer{
			"org1": {org1Peer0, org1Peer1},
			"org2": {org2Peer0, org2Peer1, org2Peer2},
			"org3": {org3Peer0},
		},
	}

	return client
}

func TestParsePolicyFailures(t *testing.T) {
	client := initClient()
	cPeers, ccPeers, err := parsePolicy(nil, client.orgPeers, client.Peers)
	assert.Nil(t, cPeers)
	assert.Nil(t, ccPeers)
	assert.NotNil(t, err)
	assert.EqualError(t, err, "no cc is configured")
}

func TestParsePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		testName        string
		config          []ChannelChaincodeConfig
		expectedCpeers  map[string]int
		expectedCCpeers map[string]map[string]struct {
			queryPeersNum  int
			invokePeersNum map[string]int
		}
	}{
		{
			testName: "one channel and one chaincode for or policy",
			config: []ChannelChaincodeConfig{
				{ChannelId: "channel1", ChaincodeName: "chaincode1", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "or"}},
			},
			expectedCpeers: map[string]int{"channel1": 6},
			expectedCCpeers: map[string]map[string]struct {
				queryPeersNum  int
				invokePeersNum map[string]int
			}{"channel1": {"chaincode1": {queryPeersNum: 6, invokePeersNum: map[string]int{"or": 5}}}},
		},
		{
			testName: "one channel and one chaincode for and policy",
			config: []ChannelChaincodeConfig{
				{ChannelId: "channel1", ChaincodeName: "chaincode1", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "and"}},
			},
			expectedCpeers: map[string]int{"channel1": 6},
			expectedCCpeers: map[string]map[string]struct {
				queryPeersNum  int
				invokePeersNum map[string]int
			}{"channel1": {"chaincode1": {queryPeersNum: 6, invokePeersNum: map[string]int{"org1": 2, "org2": 3}}}},
		},
		{
			testName: "one channel and two chaincodes for two policies",
			config: []ChannelChaincodeConfig{
				{ChannelId: "channel1", ChaincodeName: "chaincode1", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "or"}},
				{ChannelId: "channel1", ChaincodeName: "chaincode2", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "and"}},
			},
			expectedCpeers: map[string]int{"channel1": 6},
			expectedCCpeers: map[string]map[string]struct {
				queryPeersNum  int
				invokePeersNum map[string]int
			}{"channel1": {"chaincode1": {queryPeersNum: 6, invokePeersNum: map[string]int{"or": 5}},
				"chaincode2": {queryPeersNum: 6, invokePeersNum: map[string]int{"org1": 2, "org2": 3}}}},
		},
		{
			testName: "two channels and two chaincodes for two policies",
			config: []ChannelChaincodeConfig{
				{ChannelId: "channel1", ChaincodeName: "chaincode1", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "or"}},
				{ChannelId: "channel1", ChaincodeName: "chaincode2", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2"}, Rule: "and"}},
				{ChannelId: "channel2", ChaincodeName: "chaincode3", ChaincodeType: ChaincodeSpec_GOLANG, ChaincodePolicy: ChaincodePolicy{Orgs: []string{"org1", "org2", "org3"}, Rule: "and"}},
			},
			expectedCpeers: map[string]int{"channel1": 6, "channel2": 6},
			expectedCCpeers: map[string]map[string]struct {
				queryPeersNum  int
				invokePeersNum map[string]int
			}{"channel1": {"chaincode1": {queryPeersNum: 6, invokePeersNum: map[string]int{"or": 5}},
				"chaincode2": {queryPeersNum: 6, invokePeersNum: map[string]int{"org1": 2, "org2": 3}}},
				"channel2": {"chaincode3": {queryPeersNum: 6, invokePeersNum: map[string]int{"org1": 2, "org2": 3, "org3": 1}}}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()

			client := initClient()
			cPeers, ccPeers, err := parsePolicy(tt.config, client.orgPeers, client.Peers)
			assert.Nil(t, err)
			assert.Equal(t, len(tt.expectedCpeers), len(cPeers))
			for k, v := range tt.expectedCpeers {
				assert.Equal(t, v, len(cPeers[k]))
			}
			assert.Equal(t, len(tt.expectedCCpeers), len(ccPeers))
			for k, v := range tt.expectedCCpeers {

				assert.NotNil(t, ccPeers[k])
				for k1, v1 := range v {
					assert.Equal(t, v1.queryPeersNum, len(ccPeers[k][k1].queryPeers))
					assert.Equal(t, len(v1.invokePeersNum), len(ccPeers[k][k1].invokePeers))
					for k2, v2 := range v1.invokePeersNum {
						assert.Equal(t, v2, len(ccPeers[k][k1].invokePeers[k2]))
					}
				}
			}
		})
	}
}
