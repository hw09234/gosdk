package e2e

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	gosdk "github.com/hw09234/gosdk/pkg"
	"github.com/stretchr/testify/assert"
)

func newChaincodeClinetConfig(t *testing.T, version string) gosdk.ChaincodeConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	ordererTLSCert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read orderer tls cert failed")

	adminIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	tlscert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read tls cert failed")

	users := make(map[string]gosdk.UserConfig)
	adminConfig := gosdk.UserConfig{
		Cert:  adminIdentity,
		Key:   adminKey,
		MspID: "Org1MSP",
	}
	users["admin"] = adminConfig
	userConfig := gosdk.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}
	users["user"] = userConfig

	return gosdk.ChaincodeConfig{
		IsEnableSyncTx: true,
		CryptoConfig: gosdk.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		Users: users,
		PConfigs: []gosdk.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: tlscert,
				},
				DomainName: "peer0.org1.example.com",
				TlsMutual:  false,
			},
		},
		OConfigs: []gosdk.OrdererConfig{
			{
				Host:             "localhost:7050",
				DomainName:       "orderer.example.com",
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: ordererTLSCert,
				},
				UseTLS: true,
			},
		},
		Channels: []gosdk.ChannelChaincodeConfig{
			{
				ChannelId:     channelName,
				ChaincodeName: chaincodeName,
				ChaincodeType: gosdk.ChaincodeSpec_GOLANG,
				ChaincodePolicy: gosdk.ChaincodePolicy{
					Orgs: []string{"org1", "org2"},
					Rule: "or",
				},
			},
		},
	}
}

func newChaincodeClinetConfigGM(t *testing.T, version string) gosdk.ChaincodeConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	ordererTLSCert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read orderer tls cert failed")

	adminIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read admin identity failed")

	adminKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read admin key failed")

	userIdentity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read user identity failed")

	userKey, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/User1@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read user key failed")

	tlscert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/server.crt", cryptoPath))
	assert.Nil(t, err, "read tls cert failed")

	users := make(map[string]gosdk.UserConfig)
	adminConfig := gosdk.UserConfig{
		Cert:  adminIdentity,
		Key:   adminKey,
		MspID: "Org1MSP",
	}
	users["admin"] = adminConfig
	userConfig := gosdk.UserConfig{
		Cert:  userIdentity,
		Key:   userKey,
		MspID: "Org1MSP",
	}
	users["user"] = userConfig

	return gosdk.ChaincodeConfig{
		IsEnableSyncTx: true,
		CryptoConfig:   gmCryptoConfig,
		Users:          users,
		PConfigs: []gosdk.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: tlscert,
				},
				DomainName: "peer0.org1.example.com",
				TlsMutual:  false,
			},
		},
		OConfigs: []gosdk.OrdererConfig{
			{
				Host:             "localhost:7050",
				DomainName:       "orderer.example.com",
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: ordererTLSCert,
				},
				UseTLS: true,
			},
		},
		Channels: []gosdk.ChannelChaincodeConfig{
			{
				ChannelId:     channelName,
				ChaincodeName: chaincodeName,
				ChaincodeType: gosdk.ChaincodeSpec_GOLANG,
				ChaincodePolicy: gosdk.ChaincodePolicy{
					Orgs: []string{"org1", "org2"},
					Rule: "or",
				},
			},
		},
	}
}
