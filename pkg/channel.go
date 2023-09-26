/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"io/ioutil"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/pkg/errors"
)

// QueryChannelsResponse peer加入的通道
type QueryChannelsResponse struct {
	PeerName string
	Error    error
	Channels []string
}

// QueryChannelInfoResponse peer加入的通道的信息
type QueryChannelInfoResponse struct {
	PeerName string
	Error    error
	Info     *common.BlockchainInfo
}

// decodeChannelFromFs 读取channel.tx文件
func decodeChannelFromFs(path string) (*common.Envelope, error) {
	channel, err := ioutil.ReadFile(path)
	if err != nil {
		logger.Errorf("read %s failed: %v", path, err)
		return nil, err
	}
	envelope := new(common.Envelope)
	if err := proto.Unmarshal(channel, envelope); err != nil {
		logger.Errorf("proto unmarshal failed: %v", err)
		return nil, err
	}
	return envelope, nil
}

// buildAndSignChannelConfig 获取通道配置并生成envelope
func buildAndSignChannelConfig(identity Identity, configPayload []byte, crypto CryptoSuite, channelId string) (*common.Envelope, error) {
	logger.Debug("enter create singned envelope progress")
	defer logger.Debug("exit create singned envelope progress")

	pl := &common.Payload{}
	if err := proto.Unmarshal(configPayload, pl); err != nil {
		return nil, errors.Errorf("envelope does not carry a valid payload: %v", err)
	}

	configUpdateEnvelope := &common.ConfigUpdateEnvelope{}
	err := proto.Unmarshal(pl.GetData(), configUpdateEnvelope)
	if err != nil {
		logger.Errorf("unmarshal config envelope failed: %v", err)
		return nil, err
	}
	creator, err := marshalProtoIdentity(identity)
	if err != nil {
		logger.Errorf("marshal identity failed: %v", err)
		return nil, err
	}
	txId, err := newTransactionId(creator, crypto)
	if err != nil {
		logger.Errorf("generate transaction id failed: %v", err)
		return nil, err
	}

	sigHeaderBytes, err := signatureHeader(creator, txId)
	if err != nil {
		logger.Errorf("get header signature bytes failed: %v", err)
		return nil, err
	}

	sig, err := crypto.Sign(append(sigHeaderBytes, configUpdateEnvelope.GetConfigUpdate()...), identity.PrivateKey)
	if err != nil {
		logger.Errorf("get signature failed: %v", err)
		return nil, err
	}

	configSignature := new(common.ConfigSignature)
	configSignature.SignatureHeader = sigHeaderBytes
	configSignature.Signature = sig
	configUpdateEnvelope.Signatures = append(configUpdateEnvelope.GetSignatures(), configSignature)

	channelHeaderBytes, err := channelHeader(common.HeaderType_CONFIG_UPDATE, txId, channelId, 0, nil)
	header := header(sigHeaderBytes, channelHeaderBytes)

	envelopeBytes, err := proto.Marshal(configUpdateEnvelope)
	if err != nil {
		logger.Errorf("proto marshal config envelope failed: %v", err)
		return nil, err
	}
	commonPayload, err := payload(header, envelopeBytes)
	if err != nil {
		logger.Errorf("proto marshal header failed: %v", err)
		return nil, err
	}
	signedCommonPayload, err := crypto.Sign(commonPayload, identity.PrivateKey)
	if err != nil {
		logger.Errorf("get signature failed: %v", err)
		return nil, err
	}
	return &common.Envelope{Payload: commonPayload, Signature: signedCommonPayload}, nil
}
