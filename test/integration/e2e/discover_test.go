package e2e

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	gosdk "github.com/hw09234/gosdk/pkg"
	"github.com/stretchr/testify/assert"
)

func newDiscoveryConfig(t *testing.T, version string) gosdk.DiscoveryConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "crypto-configV2"
	} else {
		cryptoPath = "crypto-config"
	}

	peerCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	identity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read identity failed")

	key, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read key failed")

	config := gosdk.DiscoveryConfig{
		CryptoConfig: gosdk.CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		PeerConfigs: []gosdk.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				DomainName:       "peer0.org1.example.com",
				TlsMutual:        false,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: peerCACert,
				},
			},
		},
		UserConfig: gosdk.UserConfig{
			MspID: "Org1MSP",
			Cert:  identity,
			Key:   key,
		},
	}

	return config
}

func newDiscoveryConfigGM(t *testing.T, version string) gosdk.DiscoveryConfig {
	var cryptoPath string

	if version == "v2" {
		cryptoPath = "gm-crypto-configV2"
	} else {
		cryptoPath = "gm-crypto-config"
	}

	peerCACert, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/tls/ca.crt", cryptoPath))
	assert.Nil(t, err, "read peer ca cert failed")

	identity, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/signcerts/Admin@org1.example.com-cert.pem", cryptoPath))
	assert.Nil(t, err, "read identity failed")

	key, err := ioutil.ReadFile(fmt.Sprintf("../../fixtures/%s/peerOrganizations/org1.example.com/users/Admin@org1.example.com/msp/keystore/priv_sk", cryptoPath))
	assert.Nil(t, err, "read key failed")

	config := gosdk.DiscoveryConfig{
		CryptoConfig: gmCryptoConfig,
		PeerConfigs: []gosdk.PeerConfig{
			{
				Name:             "peer0.org1.example.com",
				Host:             "localhost:7051",
				OrgName:          "org1",
				UseTLS:           true,
				Timeout:          3 * time.Second,
				KeepaliveTime:    10 * time.Second,
				KeepaliveTimeout: 3 * time.Second,
				DomainName:       "peer0.org1.example.com",
				TlsMutual:        false,
				TlsConfig: gosdk.TlsConfig{
					ServerCert: peerCACert,
				},
			},
		},
		UserConfig: gosdk.UserConfig{
			MspID: "Org1MSP",
			Cert:  identity,
			Key:   key,
		},
	}

	return config
}
