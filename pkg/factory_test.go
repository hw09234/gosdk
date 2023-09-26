/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	// Arbitrary valid pem encoded x509 certificate from crypto/x509 tests.
	// The contents of the certifcate don't matter, we just need a valid certificate
	// to pass marshaling/unmarshalling.
	dummyCert = `-----BEGIN CERTIFICATE-----
MIIDATCCAemgAwIBAgIRAKQkkrFx1T/dgB/Go/xBM5swDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xNjA4MTcyMDM2MDdaFw0xNzA4MTcyMDM2
MDdaMBIxEDAOBgNVBAoTB0FjbWUgQ28wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQDAoJtjG7M6InsWwIo+l3qq9u+g2rKFXNu9/mZ24XQ8XhV6PUR+5HQ4
jUFWC58ExYhottqK5zQtKGkw5NuhjowFUgWB/VlNGAUBHtJcWR/062wYrHBYRxJH
qVXOpYKbIWwFKoXu3hcpg/CkdOlDWGKoZKBCwQwUBhWE7MDhpVdQ+ZljUJWL+FlK
yQK5iRsJd5TGJ6VUzLzdT4fmN2DzeK6GLeyMpVpU3sWV90JJbxWQ4YrzkKzYhMmB
EcpXTG2wm+ujiHU/k2p8zlf8Sm7VBM/scmnMFt0ynNXop4FWvJzEm1G0xD2t+e2I
5Utr04dOZPCgkm++QJgYhtZvgW7ZZiGTAgMBAAGjUjBQMA4GA1UdDwEB/wQEAwIF
oDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMBsGA1UdEQQUMBKC
EHRlc3QuZXhhbXBsZS5jb20wDQYJKoZIhvcNAQELBQADggEBADpqKQxrthH5InC7
X96UP0OJCu/lLEMkrjoEWYIQaFl7uLPxKH5AmQPH4lYwF7u7gksR7owVG9QU9fs6
1fK7II9CVgCd/4tZ0zm98FmU4D0lHGtPARrrzoZaqVZcAvRnFTlPX5pFkPhVjjai
/mkxX9LpD8oK1445DFHxK5UjLMmPIIWd8EOi+v5a+hgGwnJpoW7hntSl8kHMtTmy
fnnktsblSUV4lRCit0ymC7Ojhe+gzCCwkgs5kDzVVag+tnl/0e2DloIjASwOhpbH
KVcg7fBd484ht/sS+l0dsB4KDOSpd8JzVDMF8OZqlaydizoJO0yWr9GbCN1+OKq5
EhLrEqU=
-----END CERTIFICATE-----
`

	// Arbitrary valid pem encoded ec private key.
	// The contents of the private key don't matter, we just need a valid
	// EC private key to pass marshaling/unmarshalling.
	dummyPrivateKey = `-----BEGIN EC PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgDZUgDvKixfLi8cK8
/TFLY97TDmQV3J2ygPpvuI8jSdihRANCAARRN3xgbPIR83dr27UuDaf2OJezpEJx
UC3v06+FD8MUNcRAboqt4akehaNNSh7MMZI+HdnsM4RXN2y8NePUQsPL
-----END EC PRIVATE KEY-----
`
)

func TestNewDiscovery(t *testing.T) {
	config := DiscoveryConfig{
		CryptoConfig: CryptoConfig{
			Family:    "ecdsa",
			Algorithm: "P256-SHA256",
			Hash:      "SHA2-256",
		},
		PeerConfigs: []PeerConfig{
			{
				Name:       "peer0.org1.example.com",
				Host:       "peer0.org1.example.com:7051",
				OrgName:    "org1",
				UseTLS:     true,
				DomainName: "peer0.org1.example.com",
				TlsMutual:  false,
				TlsConfig: TlsConfig{
					ServerCert: []byte(dummyCert),
				},
			},
		},
		UserConfig: UserConfig{
			MspID: "Org1MSP",
			Cert:  []byte(dummyCert),
			Key:   []byte(dummyPrivateKey),
		},
	}

	_, err := NewDiscoveryClient(config)
	assert.Nil(t, err)
	//assert.Equal(t, &DiscoveryClientImpl{config: config}, sdk.(*DiscoveryClientImpl))
}
