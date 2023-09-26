/*
Copyright: peerfintech. All Rights Reserved.
*/

package gosdk

import (
	"context"
	"google.golang.org/grpc/connectivity"
	"sync"
	"sync/atomic"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type PeersInfo struct {
	current uint64
	peers   []*Peer
}

// getNextPeer 获取下一个可用peer
func (pi *PeersInfo) getNextPeer() *Peer {
	logger.Debug("enter get an available peer progress")
	defer logger.Debug("exit get an available peer progress")

	preCurrent := int(pi.current)
	next := int(pi.current)
	l := len(pi.peers)
	// 如果当前游标对应节点可用，则直接返回
	if pi.peers[next].isAlive() {
		logger.Debugf("the current peer %s is available, no need to choose the next one", pi.peers[next].uri)
		return pi.peers[next]
	}
	for {
		next = (next + 1) % l
		// 所有节点已被循环一遍，无可用节点，直接返回空
		if next == preCurrent {
			logger.Error("all peers are not alive, return nil")
			return nil
		}
		// 如果节点可用，则修改current游标，并返回对应的节点
		if pi.peers[next].isAlive() {
			logger.Debugf("currently using peer %s", pi.peers[next].uri)
			atomic.StoreUint64(&pi.current, uint64(next))
			return pi.peers[next]
		}
		logger.Warnf("%s is unavailable, choose next", pi.peers[next].uri)
	}
}

// Peer expose API's to communicate with peer
type Peer struct {
	once    *sync.Once
	mux     sync.RWMutex
	conn    *grpc.ClientConn
	pClient peer.EndorserClient
	node
}

// setAlive 更改peer状态
func (p *Peer) setAlive(alive bool) {
	p.mux.Lock()
	p.alive = alive
	p.mux.Unlock()
}

// isAlive 判断peer是否可用
func (p *Peer) isAlive() (alive bool) {
	p.mux.RLock()
	alive = p.alive
	p.mux.RUnlock()
	return
}

// waitRecovery 监听连接状态变化，更改peer状态
func (p *Peer) waitRecovery() {
	if p.once != nil {
		p.once.Do(func() {
			for {
				p.mux.RLock()
				if p.conn == nil {
					p.mux.RUnlock()
					return
				}
				preState := p.conn.GetState()
				if p.conn.WaitForStateChange(context.Background(), preState) {
					if p.conn.GetState() == connectivity.Ready {
						p.mux.RUnlock()
						logger.Debugf("%s is available", p.uri)
						p.setAlive(true)
						p.once = new(sync.Once)
						return
					}
				}
				p.mux.RUnlock()
			}
		})
	}
}

// PeerResponse is response from peer transaction request
type PeerResponse struct {
	Response *peer.ProposalResponse
	Err      error
	Name     string
}

// newPeer 根据配置信息构建与peer的连接
func newPeer(nodeConfig nodeConfig, crypto CryptoSuite) (*Peer, error) {
	n, err := newNode(nodeConfig, crypto)
	if err != nil {
		return nil, errors.Errorf("new node for peer fialed: %s", err.Error())
	}

	ctxDial, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()
	c, err := grpc.DialContext(ctxDial, n.uri, n.opts...)
	if err != nil {
		return nil, errors.Errorf("connect host=%s failed, err:%s", n.uri, err.Error())
	}

	p := peer.NewEndorserClient(c)
	return &Peer{conn: c, pClient: p, node: *n, once: new(sync.Once)}, nil
}

// Endorse sends single transaction to single peer.
func (p *Peer) Endorse(prop *peer.SignedProposal, endorsetype endorseType) (*peer.ProposalResponse, error) {
	logger.Debug("enter peer endorse progress")
	defer logger.Debug("exit peer endorse progress")

	var ctx context.Context
	var cancel context.CancelFunc

	if endorsetype == instantiate {
		logger.Debug("instantiate chaincode")
		ctx, cancel = context.WithTimeout(context.Background(), p.instantiateTimeout)
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), p.timeout)
	}
	defer cancel()

	proposalResp, err := p.pClient.ProcessProposal(ctx, prop)
	if err != nil {
		logger.Errorf("process proposal failed: %v", err)
		return nil, err
	}
	return proposalResp, nil
}
