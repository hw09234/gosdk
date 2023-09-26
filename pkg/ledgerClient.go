/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"math"
	"strconv"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
)

type LedgerClientImpl struct {
	crypto    CryptoSuite
	peersInfo *PeersInfo
	identity  *Identity
}

// queryByQscc 根据系统智能合约Qscc查询信息
func (lc *LedgerClientImpl) queryByQscc(args []string, channelName string) ([]*QueryResponse, error) {
	var peer *Peer
	var err error
	var qr []*QueryResponse
	var current = lc.peersInfo.current

	if channelName == "" {
		return nil, errors.New("channel name is nil")
	}

	chaincode := ChainCode{
		ChannelId: channelName,
		Type:      ChaincodeSpec_GOLANG,
		Name:      QSCC,
		Args:      args,
	}

	peer = lc.peersInfo.peers[current]
	for {
		logger.Debugf("peer %s try to query", peer.uri)
		qr, err = getPeerQueryRes(*lc.identity, chaincode, lc.crypto, []*Peer{peer})
		// 发送给peer处理成功，直接返回
		// 或peer成功接收，但内部处理失败，直接返回
		if err == nil || qr[0].Error == nil {
			return qr, nil
		}
		logger.Errorf("peer %s query failed: %s", peer.uri, err.Error())

		// 发送给peer失败，选取下一个peer继续发送
		peer.setAlive(false)
		go peer.waitRecovery()
		if peer = lc.peersInfo.getNextPeer(); peer == nil {
			return nil, errors.New("all peers are unavailable to deliver")
		}
	}
}

// GetBlockHeight: 查询区块高度
func (lc *LedgerClientImpl) GetBlockHeight(channelName string) (uint64, error) {
	if channelName == "" {
		return 0, errors.New("channel name is nil")
	}

	args := []string{"GetChainInfo", channelName}
	res, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return 0, errors.WithMessagef(err, "failed get block height in channel %s", channelName)
	}
	if len(res) == 0 {
		return 0, errors.Errorf("does not get response for block height in channel %s", channelName)
	}

	h, err := parseHeight(res[0].Response)
	if err != nil {
		return 0, errors.WithMessage(err, "parse response to height failed")
	}
	return h, nil
}

// GetBlockByNumber 根据区块编号查询区块
func (lc *LedgerClientImpl) GetBlockByNumber(channelName string, blockNum uint64) (*common.Block, error) {
	if channelName == "" {
		return nil, errors.New("channel name is nil")
	}

	strBlockNum := strconv.FormatUint(blockNum, 10)
	args := []string{"GetBlockByNumber", channelName, strBlockNum}

	res, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get block %d in channel %s", blockNum, channelName)
	}
	if len(res) == 0 {
		return nil, errors.Errorf("does not get response for block %d in channel %s", blockNum, channelName)
	}

	b, err := parseBlock(res[0].Response)
	if err != nil {
		return nil, errors.WithMessagef(err, "parse response to block failed")
	}
	return b, nil
}

// GetBlockByTxID 根据transaction ID查询区块
// 当有相同txID的交易时，只会取已被验证通过的交易，所以只会有一笔交易
func (lc *LedgerClientImpl) GetBlockByTxID(channelName string, txID string) (*common.Block, error) {
	if txID == "" || channelName == "" {
		return nil, errors.New("channel name or txid is empty")
	}

	args := []string{"GetBlockByTxID", channelName, txID}

	res, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get block by txID %s in channel %s", txID, channelName)
	}
	if len(res) == 0 {
		return nil, errors.Errorf("does not get response for block by txID %s in channel %s", txID, channelName)
	}

	b, err := parseBlock(res[0].Response)
	if err != nil {
		return nil, errors.WithMessagef(err, "parse response to block failed")
	}
	return b, nil
}

// GetTransactionByTxID 根据交易ID查询交易
func (lc *LedgerClientImpl) GetTransactionByTxID(channelName string, txID string) (*peer.ProcessedTransaction, error) {
	if txID == "" || channelName == "" {
		return nil, errors.New("channel name or txid is empty")
	}

	args := []string{"GetTransactionByID", channelName, txID}

	res, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get tx %s in channel %s", txID, channelName)
	}
	if len(res) == 0 {
		return nil, errors.Errorf("does not get response for tx %s in channel %s", txID, channelName)
	}

	t, err := parseTX(res[0].Response)
	if err != nil {
		return nil, errors.WithMessagef(err, "parse response to tx failed")
	}
	return t, nil
}

// GetLastBlock 获取最新的一个区块
func (lc *LedgerClientImpl) GetNewestBlock(channelName string) (*common.Block, error) {
	if channelName == "" {
		return nil, errors.New("channel name is nil")
	}

	// 当区块号为math.MaxUint64，获取最新区块
	strBlockNum := strconv.FormatUint(math.MaxUint64, 10)
	args := []string{"GetBlockByNumber", channelName, strBlockNum}

	res, err := lc.queryByQscc(args, channelName)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed get last block in channel %s", channelName)
	}
	if len(res) == 0 {
		return nil, errors.Errorf("does not get response for last block in channel %s", channelName)
	}

	b, err := parseBlock(res[0].Response)
	if err != nil {
		return nil, errors.WithMessagef(err, "parse response to block failed")
	}
	return b, nil
}

func (lc *LedgerClientImpl) Close() error {
	for _, peer := range lc.peersInfo.peers {
		if peer.conn == nil {
			continue
		}
		if err := peer.conn.Close(); err != nil {
			logger.Errorf("close connection to peer %s failed: %s", peer.uri, err.Error())
			continue
		}
		peer.conn = nil
	}
	return nil
}

func (lc *LedgerClientImpl) GetCrypto() (CryptoSuite, error) {
	if lc.crypto == nil {
		return nil, errors.New("crypto suite is nil")
	}

	return lc.crypto, nil
}
