/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"

	"github.com/hw09234/gm-crypto/sm2"
	"github.com/hw09234/gm-crypto/x509"
	"github.com/pkg/errors"
)

// Identity is participant public and private key
type Identity struct {
	Certificate *x509.Certificate
	PrivateKey  interface{}
	MspId       string
	Cert        []byte
	Key         []byte
}

// EnrollmentId get enrollment id from certificate
func (i *Identity) EnrollmentId() string {
	return i.Certificate.Subject.CommonName
}

// EnrollmentId get enrollment id from certificate
func (i *Identity) ToPem() ([]byte, []byte, error) {
	switch i.PrivateKey.(type) {
	case *ecdsa.PrivateKey, *sm2.PrivateKey:
		b, err := x509.MarshalECPrivateKey(i.PrivateKey)
		if err != nil {
			return nil, nil, err
		}
		privateKey := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
		cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: i.Certificate.Raw})
		return cert, privateKey, nil

	default:
		return nil, nil, ErrInvalidKeyType
	}
}

// MarshalIdentity marshal identity to string
func MarshalIdentity(i *Identity) (string, error) {
	var pk, cert string
	switch i.PrivateKey.(type) {
	case *ecdsa.PrivateKey, *sm2.PrivateKey:
		b, err := x509.MarshalECPrivateKey(i.PrivateKey)
		if err != nil {
			return "", err
		}
		block := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
		pk = base64.RawStdEncoding.EncodeToString(block)

	default:
		return "", ErrInvalidKeyType
	}

	cert = base64.RawStdEncoding.EncodeToString(i.Certificate.Raw)
	str, err := json.Marshal(map[string]string{"cert": cert, "pk": pk, "mspid": i.MspId})
	if err != nil {
		return "", err
	}

	return string(str), nil
}

// UnmarshalIdentity unmarshal identity from string
func UnmarshalIdentity(data string) (*Identity, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, err
	}
	// check do we have all keys
	if _, ok := raw["cert"]; !ok || len(raw["cert"]) < 1 {
		return nil, ErrInvalidDataForParcelIdentity
	}
	if _, ok := raw["pk"]; !ok || len(raw["pk"]) < 1 {
		return nil, ErrInvalidDataForParcelIdentity
	}

	certRaw, err := base64.RawStdEncoding.DecodeString(raw["cert"])
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certRaw)
	if err != nil {
		return nil, err
	}

	keyRaw, err := base64.RawStdEncoding.DecodeString(raw["pk"])
	if err != nil {
		return nil, err
	}
	keyPem, _ := pem.Decode(keyRaw)
	if keyPem == nil {
		return nil, ErrInvalidDataForParcelIdentity
	}
	var pk interface{}
	switch keyPem.Type {
	case "EC PRIVATE KEY":
		pk, err = x509.ParseECPrivateKey(keyPem.Bytes)
		if err != nil {
			return nil, ErrInvalidDataForParcelIdentity
		}
	default:
		return nil, ErrInvalidDataForParcelIdentity
	}

	identity := &Identity{Certificate: cert, PrivateKey: pk, MspId: raw["mspid"]}
	return identity, nil

}

// newUsers 根据用户配置生成用户identity
// 选取第一个用户为默认用户，在不指定用户时选择此用户
// 选取一个admin用户为默认admin用户，在不选取admin用户时选择此用户
func newUsers(config map[string]UserConfig, identity map[string]Identity) error {
	for key, userConfig := range config {
		user, err := newIdentityFromUserConfig(userConfig)
		if err != nil {
			return errors.WithMessagef(err, "create user %s failed", key)
		}

		if _, ok := identity[defaultUser]; !ok {
			identity[defaultUser] = *user
		}

		if _, ok := identity[adminUser]; !ok && user.Certificate.Subject.OrganizationalUnit[0] == "admin" {
			identity[adminUser] = *user
		}

		identity[key] = *user
	}
	return nil
}
