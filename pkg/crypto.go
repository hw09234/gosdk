/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"hash"
	"math/big"
	"net"
	"net/mail"
	"strings"

	"github.com/hw09234/gm-crypto/sm2"
	"github.com/hw09234/gm-crypto/sm3"
	"github.com/hw09234/gm-crypto/x509"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

//go:generate mockery --dir . --name CryptoSuite --case underscore  --output mocks/

// CryptSuite defines common interface for different crypto implementations.
// Currently Hyperledger Fabric supports only Elliptic curves.
type CryptoSuite interface {
	// GenerateKey returns PrivateKey.
	GenerateKey() (interface{}, error)
	// CreateCertificateRequest will create CSR request. It takes enrolmentId and Private key
	CreateCertificateRequest(enrollmentId string, key interface{}, hosts []string) ([]byte, error)
	// Sign signs message. It takes message to sign and Private key
	Sign(msg []byte, key interface{}) ([]byte, error)
	// Hash computes Hash value of provided data. Hash function will be different in different crypto implementations.
	Hash(data []byte) []byte
	GetFamily() string
}

var (
	// precomputed curves half order values for efficiency
	ecCurveHalfOrders = map[elliptic.Curve]*big.Int{
		elliptic.P224(): new(big.Int).Rsh(elliptic.P224().Params().N, 1),
		elliptic.P256(): new(big.Int).Rsh(elliptic.P256().Params().N, 1),
		elliptic.P384(): new(big.Int).Rsh(elliptic.P384().Params().N, 1),
		elliptic.P521(): new(big.Int).Rsh(elliptic.P521().Params().N, 1),
	}
)

type signature struct {
	R, S *big.Int
}

// ECCryptSuite implements Ecliptic curve crypto suite
type ECCryptSuite struct {
	curve        elliptic.Curve
	sigAlgorithm x509.SignatureAlgorithm
	key          *ecdsa.PrivateKey
	hashFunction func() hash.Hash
	family       string
}

func (c *ECCryptSuite) GenerateKey() (interface{}, error) {
	key, err := ecdsa.GenerateKey(c.curve, rand.Reader)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (c *ECCryptSuite) CreateCertificateRequest(enrollmentId string, key interface{}, hosts []string) ([]byte, error) {
	if enrollmentId == "" {
		return nil, ErrEnrollmentIdMissing
	}
	subj := pkix.Name{
		CommonName: enrollmentId,
	}
	rawSubj := subj.ToRDNSequence()

	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		return nil, err
	}

	ipAddr := make([]net.IP, 0)
	emailAddr := make([]string, 0)
	dnsAddr := make([]string, 0)

	for i := range hosts {
		if ip := net.ParseIP(hosts[i]); ip != nil {
			ipAddr = append(ipAddr, ip)
		} else if email, err := mail.ParseAddress(hosts[i]); err == nil && email != nil {
			emailAddr = append(emailAddr, email.Address)
		} else {
			dnsAddr = append(dnsAddr, hosts[i])
		}
	}

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: c.sigAlgorithm,
		IPAddresses:        ipAddr,
		EmailAddresses:     emailAddr,
		DNSNames:           dnsAddr,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, err
	}
	csr := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	return csr, nil
}

func (c *ECCryptSuite) Sign(msg []byte, k interface{}) ([]byte, error) {
	key, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidKeyType
	}
	var h []byte
	h = c.Hash(msg)
	R, S, err := ecdsa.Sign(rand.Reader, key, h)
	if err != nil {
		return nil, err
	}
	c.preventMalleability(key, S)
	sig, err := asn1.Marshal(signature{R, S})
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// For low-S
// ECDSA signature can be "exploited" using symmetry of S values.
// Fabric (by convention) accepts only signatures with lowS values
// If result of a signature is high-S value we have to subtract S from curve.N
// For more details https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki
func (c *ECCryptSuite) preventMalleability(k *ecdsa.PrivateKey, S *big.Int) {
	halfOrder := ecCurveHalfOrders[k.Curve]
	if S.Cmp(halfOrder) == 1 {
		S.Sub(k.Params().N, S)
	}
}

func (c *ECCryptSuite) Hash(data []byte) []byte {
	h := c.hashFunction()
	h.Write(data)
	return h.Sum(nil)
}

func (c *ECCryptSuite) GetFamily() string {
	return c.family
}

// GmCryptSuite implements Ecliptic curve sm crypto suite
type GmCryptSuite struct {
	curve        elliptic.Curve
	sigAlgorithm x509.SignatureAlgorithm
	key          *sm2.PrivateKey
	hashFunction func() hash.Hash
	family       string
}

func (c *GmCryptSuite) GenerateKey() (interface{}, error) {
	key, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (c *GmCryptSuite) CreateCertificateRequest(enrollmentId string, key interface{}, hosts []string) ([]byte, error) {
	if enrollmentId == "" {
		return nil, ErrEnrollmentIdMissing
	}
	subj := pkix.Name{
		CommonName: enrollmentId,
	}
	rawSubj := subj.ToRDNSequence()

	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		return nil, err
	}

	ipAddr := make([]net.IP, 0)
	emailAddr := make([]string, 0)
	dnsAddr := make([]string, 0)

	for i := range hosts {
		if ip := net.ParseIP(hosts[i]); ip != nil {
			ipAddr = append(ipAddr, ip)
		} else if email, err := mail.ParseAddress(hosts[i]); err == nil && email != nil {
			emailAddr = append(emailAddr, email.Address)
		} else {
			dnsAddr = append(dnsAddr, hosts[i])
		}
	}

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: c.sigAlgorithm,
		IPAddresses:        ipAddr,
		EmailAddresses:     emailAddr,
		DNSNames:           dnsAddr,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, err
	}
	csr := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	return csr, nil
}

func (c *GmCryptSuite) Sign(msg []byte, k interface{}) ([]byte, error) {
	key, ok := k.(*sm2.PrivateKey)
	if !ok {
		return nil, ErrInvalidKeyType
	}
	h := c.Hash(msg)
	R, S, err := sm2.Sign(rand.Reader, key, h)
	if err != nil {
		return nil, err
	}
	sig, err := asn1.Marshal(signature{R, S})
	if err != nil {
		return nil, err
	}
	return sig, nil
}

func (c *GmCryptSuite) Hash(data []byte) []byte {
	h := c.hashFunction()
	h.Write(data)
	return h.Sum(nil)
}

func (c *GmCryptSuite) GetFamily() string {
	return c.family
}

// newECCryptSuite creates new Elliptic curve crypto suite from config
func newECCryptSuiteFromConfig(config CryptoConfig) (CryptoSuite, error) {
	if strings.ToUpper(config.Family) != "ECDSA" &&
		strings.ToUpper(config.Family) != "GM" {
		return nil, ErrInvalidAlgorithmFamily
	}
	var hashFunc func() hash.Hash
	switch config.Hash {
	case "SHA2-256":
		hashFunc = sha256.New
	case "SHA2-384":
		hashFunc = sha512.New384
	case "SHA3-256":
		hashFunc = sha3.New256
	case "SHA3-384":
		hashFunc = sha3.New384
	case "SM3":
		hashFunc = sm3.New
	default:
		return nil, ErrInvalidHash
	}

	switch config.Algorithm {
	case "P256-SHA256":
		return &ECCryptSuite{curve: elliptic.P256(), sigAlgorithm: x509.ECDSAWithSHA256, family: config.Family, hashFunction: hashFunc}, nil
	case "P384-SHA384":
		return &ECCryptSuite{curve: elliptic.P384(), sigAlgorithm: x509.ECDSAWithSHA384, family: config.Family, hashFunction: hashFunc}, nil
	case "P521-SHA512":
		return &ECCryptSuite{curve: elliptic.P521(), sigAlgorithm: x509.ECDSAWithSHA512, family: config.Family, hashFunction: hashFunc}, nil
	case "P256SM2":
		return &GmCryptSuite{curve: sm2.P256(), sigAlgorithm: x509.SM2WithSM3, family: config.Family, hashFunction: hashFunc}, nil
	default:
		return nil, ErrInvalidAlgorithm
	}
}

//newIdentityFromUserConfig 从结构体初始化Identity
func newIdentityFromUserConfig(conf UserConfig) (*Identity, error) {
	cpb, _ := pem.Decode(conf.Cert)
	kpb, _ := pem.Decode(conf.Key)
	crt, err := x509.ParseCertificate(cpb.Bytes)
	if err != nil {
		return nil, errors.WithMessage(err, "parse Certificate failed")
	}
	key, err := x509.ParsePKCS8PrivateKey(kpb.Bytes)
	if err != nil {
		return nil, errors.WithMessage(err, "parse private key failed")
	}
	return &Identity{Certificate: crt, PrivateKey: key, MspId: conf.MspID, Cert: conf.Cert, Key: conf.Key}, nil
}

var defaultCryptoConfig = CryptoConfig{
	Family:    "ecdsa",
	Algorithm: "P256-SHA256",
	Hash:      "SHA2-256",
}

var defaultCryptSuite = &ECCryptSuite{
	curve:        elliptic.P256(),
	sigAlgorithm: x509.ECDSAWithSHA256,
	hashFunction: sha256.New,
	family:       "ecdsa",
}
