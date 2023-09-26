package e2e

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	gohfc "github.com/hw09234/gohfc/pkg"
	"github.com/stretchr/testify/assert"
)

func newOrg1ClientConfig(t *testing.T, version string) gohfc.ClientConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	peer01CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	peer02CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	ordererCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/users/Admin@example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read orderer ca cert failed")

	adminIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	orderers := make(map[string]gohfc.OrdererConfig)
	ordererConfig := gohfc.OrdererConfig{
		Name:             "orderer.example.com",
		Host:             "localhost:7050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer2Config := gohfc.OrdererConfig{
		Name:             "orderer2.example.com",
		Host:             "localhost:8050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer2.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer3Config := gohfc.OrdererConfig{
		Name:             "orderer3.example.com",
		Host:             "localhost:9050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer3.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderers["orderer0"] = ordererConfig
	orderers["orderer1"] = orderer2Config
	orderers["orderer2"] = orderer3Config

	peers := make(map[string]gohfc.PeerConfig)
	peerConfig01 := gohfc.PeerConfig{
		Name:               "peer0.org1.example.com",
		Host:               "localhost:7051",
		OrgName:            "org1",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org1.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer01CACert,
		},
	}
	peerConfig02 := gohfc.PeerConfig{
		Name:               "peer0.org2.example.com",
		Host:               "localhost:9051",
		OrgName:            "org2",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org2.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer02CACert,
		},
	}
	peers["peer01"] = peerConfig01
	peers["peer02"] = peerConfig02

	eventPeerConfigs := []gohfc.PeerConfig{
		{
			Name:               "peer0.org1.example.com",
			Host:               "localhost:7051",
			OrgName:            "org1",
			UseTLS:             true,
			Timeout:            10 * time.Second,
			InstantiateTimeout: 3 * time.Minute,
			KeepaliveTime:      60 * time.Second,
			KeepaliveTimeout:   30 * time.Second,
			DomainName:         "peer0.org1.example.com",
			TlsMutual:          false,
			TlsConfig: gohfc.TlsConfig{
				ServerCert: peer01CACert,
			},
		},
	}

	users := make(map[string]gohfc.UserConfig)
	adminConfig := gohfc.UserConfig{
		Cert:  adminIdentity,
		Key:   adminKey,
		MspID: "Org1MSP",
	}
	users["org1Admin"] = adminConfig
	userConfig := gohfc.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}
	users["org1User"] = userConfig

	config := gohfc.ClientConfig{
		Crypto: gohfc.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		Orderers:   orderers,
		Peers:      peers,
		EventPeers: eventPeerConfigs,
		Users:      users,
		Channels: []gohfc.ChannelChaincodeConfig{{
			ChannelId:        "mychannel",
			ChaincodeName:    "mycc",
			ChaincodeVersion: "1.0",
			ChaincodeType:    gohfc.ChaincodeSpec_GOLANG,
			ChaincodePolicy: gohfc.ChaincodePolicy{
				Orgs: []string{"org1", "org2"},
				Rule: "or",
			}},
		},
	}

	return config
}

func newOrg2ClientConfig(t *testing.T, version string) gohfc.ClientConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	peer01CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	peer02CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	ordererCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/users/Admin@example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read orderer ca cert failed")

	adminIdentity2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp/signcerts/Admin@org2.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/User1@org2.example.com/msp/signcerts/User1@org2.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/User1@org2.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	orderers := make(map[string]gohfc.OrdererConfig)
	ordererConfig := gohfc.OrdererConfig{
		Name:             "orderer.example.com",
		Host:             "localhost:7050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer2Config := gohfc.OrdererConfig{
		Name:             "orderer2.example.com",
		Host:             "localhost:8050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer2.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer3Config := gohfc.OrdererConfig{
		Name:             "orderer3.example.com",
		Host:             "localhost:9050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer3.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderers["orderer0"] = ordererConfig
	orderers["orderer1"] = orderer2Config
	orderers["orderer2"] = orderer3Config

	peers := make(map[string]gohfc.PeerConfig)
	peerConfig01 := gohfc.PeerConfig{
		Name:               "peer0.org1.example.com",
		Host:               "localhost:7051",
		OrgName:            "org1",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org1.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer01CACert,
		},
	}
	peerConfig02 := gohfc.PeerConfig{
		Name:               "peer0.org2.example.com",
		Host:               "localhost:9051",
		OrgName:            "org2",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org2.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer02CACert,
		},
	}
	peers["peer01"] = peerConfig01
	peers["peer02"] = peerConfig02

	eventPeerConfigs := []gohfc.PeerConfig{
		{
			Name:               "peer0.org1.example.com",
			Host:               "localhost:7051",
			OrgName:            "org1",
			UseTLS:             true,
			Timeout:            10 * time.Second,
			InstantiateTimeout: 3 * time.Minute,
			KeepaliveTime:      60 * time.Second,
			KeepaliveTimeout:   30 * time.Second,
			DomainName:         "peer0.org1.example.com",
			TlsMutual:          false,
			TlsConfig: gohfc.TlsConfig{
				ServerCert: peer01CACert,
			},
		},
	}

	users := make(map[string]gohfc.UserConfig)
	adminConfig2 := gohfc.UserConfig{
		Cert:  adminIdentity2,
		Key:   adminKey2,
		MspID: "Org2MSP",
	}
	users["org2Admin"] = adminConfig2
	userConfig2 := gohfc.UserConfig{
		Cert:  userIdentity2,
		Key:   userKey2,
		MspID: "Org2MSP",
	}
	users["org2User"] = userConfig2

	config := gohfc.ClientConfig{
		Crypto: gohfc.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		Orderers:   orderers,
		Peers:      peers,
		EventPeers: eventPeerConfigs,
		Users:      users,
		Channels: []gohfc.ChannelChaincodeConfig{{
			ChannelId:        "mychannel",
			ChaincodeName:    "mycc",
			ChaincodeVersion: "1.0",
			ChaincodeType:    gohfc.ChaincodeSpec_GOLANG,
			ChaincodePolicy: gohfc.ChaincodePolicy{
				Orgs: []string{"org1", "org2"},
				Rule: "or",
			}},
		},
	}

	return config
}

func newUser(t *testing.T, version string) gohfc.UserConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	userConfig := gohfc.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}

	return userConfig
}

func newUserGM(t *testing.T, version string) gohfc.UserConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	userConfig := gohfc.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}

	return userConfig
}

func newOrg1ClientConfigGM(t *testing.T, version string) gohfc.ClientConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	peer01CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	peer02CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	ordererCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/users/Admin@example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read orderer ca cert failed")

	adminIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	orderers := make(map[string]gohfc.OrdererConfig)
	ordererConfig := gohfc.OrdererConfig{
		Name:             "orderer.example.com",
		Host:             "localhost:7050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}

	orderer2Config := gohfc.OrdererConfig{
		Name:             "orderer2.example.com",
		Host:             "localhost:8050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer2.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer3Config := gohfc.OrdererConfig{
		Name:             "orderer3.example.com",
		Host:             "localhost:9050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer3.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}

	orderers["orderer0"] = ordererConfig
	orderers["orderer1"] = orderer2Config
	orderers["orderer2"] = orderer3Config

	peers := make(map[string]gohfc.PeerConfig)
	peerConfig := gohfc.PeerConfig{
		Name:               "peer0.org1.example.com",
		Host:               "localhost:7051",
		OrgName:            "org1",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org1.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer01CACert,
		},
	}
	peerConfig2 := gohfc.PeerConfig{
		Name:               "peer0.org2.example.com",
		Host:               "localhost:9051",
		OrgName:            "org2",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org2.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer02CACert,
		},
	}
	peers["peer01"] = peerConfig
	peers["peer02"] = peerConfig2

	eventPeerConfig := []gohfc.PeerConfig{
		{
			Name:               "peer0.org1.example.com",
			Host:               "localhost:7051",
			OrgName:            "org1",
			UseTLS:             true,
			Timeout:            10 * time.Second,
			InstantiateTimeout: 3 * time.Minute,
			KeepaliveTime:      60 * time.Second,
			KeepaliveTimeout:   30 * time.Second,
			DomainName:         "peer0.org1.example.com",
			TlsMutual:          false,
			TlsConfig: gohfc.TlsConfig{
				ServerCert: peer01CACert,
			},
		},
	}

	users := make(map[string]gohfc.UserConfig)
	adminConfig := gohfc.UserConfig{
		Cert:  adminIdentity,
		Key:   adminKey,
		MspID: "Org1MSP",
	}
	users["org1Admin"] = adminConfig
	userConfig := gohfc.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}
	users["org1User"] = userConfig

	config := gohfc.ClientConfig{
		Crypto: gohfc.CryptoConfig{
			Family:    "gm",
			Algorithm: "P256SM2",
			Hash:      "SM3",
		},
		Orderers:   orderers,
		Peers:      peers,
		EventPeers: eventPeerConfig,
		Users:      users,
		Channels: []gohfc.ChannelChaincodeConfig{{
			ChannelId:        "mychannel",
			ChaincodeName:    "mycc",
			ChaincodeVersion: "1.0",
			ChaincodeType:    gohfc.ChaincodeSpec_GOLANG,
			ChaincodePolicy: gohfc.ChaincodePolicy{
				Orgs: []string{"org1", "org2"},
				Rule: "or",
			}},
		},
	}

	return config
}

func newOrg2ClientConfigGM(t *testing.T, version string) gohfc.ClientConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	peer01CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	peer02CACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	ordererCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/users/Admin@example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read orderer ca cert failed")

	adminIdentity2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp/signcerts/Admin@org2.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/Admin@org2.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/User1@org2.example.com/msp/signcerts/User1@org2.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey2, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org2.example.com/users/User1@org2.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	orderers := make(map[string]gohfc.OrdererConfig)
	ordererConfig := gohfc.OrdererConfig{
		Name:             "orderer.example.com",
		Host:             "localhost:7050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}

	orderer2Config := gohfc.OrdererConfig{
		Name:             "orderer2.example.com",
		Host:             "localhost:8050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer2.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}
	orderer3Config := gohfc.OrdererConfig{
		Name:             "orderer3.example.com",
		Host:             "localhost:9050",
		OrgName:          "",
		UseTLS:           true,
		Timeout:          10 * time.Second,
		KeepaliveTime:    60 * time.Second,
		KeepaliveTimeout: 30 * time.Second,
		DomainName:       "orderer3.example.com",
		TlsMutual:        false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: ordererCACert,
		},
	}

	orderers["orderer0"] = ordererConfig
	orderers["orderer1"] = orderer2Config
	orderers["orderer2"] = orderer3Config

	peers := make(map[string]gohfc.PeerConfig)
	peerConfig := gohfc.PeerConfig{
		Name:               "peer0.org1.example.com",
		Host:               "localhost:7051",
		OrgName:            "org1",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org1.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer01CACert,
		},
	}
	peerConfig2 := gohfc.PeerConfig{
		Name:               "peer0.org2.example.com",
		Host:               "localhost:9051",
		OrgName:            "org2",
		UseTLS:             true,
		Timeout:            3 * time.Minute,
		InstantiateTimeout: 3 * time.Minute,
		KeepaliveTime:      60 * time.Second,
		KeepaliveTimeout:   30 * time.Second,
		DomainName:         "peer0.org2.example.com",
		TlsMutual:          false,
		TlsConfig: gohfc.TlsConfig{
			ServerCert: peer02CACert,
		},
	}
	peers["peer01"] = peerConfig
	peers["peer02"] = peerConfig2

	eventPeerConfig := []gohfc.PeerConfig{
		{
			Name:               "peer0.org1.example.com",
			Host:               "localhost:7051",
			OrgName:            "org1",
			UseTLS:             true,
			Timeout:            10 * time.Second,
			InstantiateTimeout: 3 * time.Minute,
			KeepaliveTime:      60 * time.Second,
			KeepaliveTimeout:   30 * time.Second,
			DomainName:         "peer0.org1.example.com",
			TlsMutual:          false,
			TlsConfig: gohfc.TlsConfig{
				ServerCert: peer01CACert,
			},
		},
	}

	users := make(map[string]gohfc.UserConfig)
	adminConfig2 := gohfc.UserConfig{
		Cert:  adminIdentity2,
		Key:   adminKey2,
		MspID: "Org2MSP",
	}
	users["org2Admin"] = adminConfig2
	userConfig2 := gohfc.UserConfig{
		Cert:  userIdentity2,
		Key:   userKey2,
		MspID: "Org2MSP",
	}
	users["org2User"] = userConfig2

	config := gohfc.ClientConfig{
		Crypto: gohfc.CryptoConfig{
			Family:    "gm",
			Algorithm: "P256SM2",
			Hash:      "SM3",
		},
		Orderers:   orderers,
		Peers:      peers,
		EventPeers: eventPeerConfig,
		Users:      users,
		Channels: []gohfc.ChannelChaincodeConfig{{
			ChannelId:        "mychannel",
			ChaincodeName:    "mycc",
			ChaincodeVersion: "1.0",
			ChaincodeType:    gohfc.ChaincodeSpec_GOLANG,
			ChaincodePolicy: gohfc.ChaincodePolicy{
				Orgs: []string{"org1", "org2"},
				Rule: "or",
			}},
		},
	}

	return config
}
