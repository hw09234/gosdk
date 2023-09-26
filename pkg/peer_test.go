/*
Copyright: peerfintech. All Rights Reserved.
*/

package gohfc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hw09234/gohfc/pkg/testpb"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

var (
	listener        net.Listener
	server          *grpc.Server
	fakeEchoService *EchoServiceServer
)

type EchoServiceServer struct {
	EchoStub       func(context.Context, *testpb.Message) (*testpb.Message, error)
	EchoStreamStub func(testpb.EchoService_EchoStreamServer) error
}

func (fake *EchoServiceServer) Echo(arg1 context.Context, arg2 *testpb.Message) (*testpb.Message, error) {
	return nil, nil
}
func (fake *EchoServiceServer) EchoStream(arg1 testpb.EchoService_EchoStreamServer) error {
	return nil
}

//构建一个GRPC server
func fakeGRPCServer() {
	var err error

	fakeEchoService = &EchoServiceServer{}
	fakeEchoService.EchoStub = func(ctx context.Context, msg *testpb.Message) (*testpb.Message, error) {
		msg.Sequence++
		return msg, nil
	}
	fakeEchoService.EchoStreamStub = func(stream testpb.EchoService_EchoStreamServer) error {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		msg.Sequence++
		return stream.Send(msg)
	}

	listener, err = net.Listen("tcp", ":9999")
	if err != nil {
		panic(err)
	}
	server = grpc.NewServer()
	testpb.RegisterEchoServiceServer(server, fakeEchoService)

	server.Serve(listener)
}

//测试构建peer
func TestNewPeerFromConfig(t *testing.T) {
	type args struct {
		cliConfig   ChannelChaincodeConfig
		conf        PeerConfig
		cryptoSuite CryptoSuite
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test withoutTLS",
			args: args{
				conf: PeerConfig{Host: ":9999", UseTLS: false, Timeout: 3 * time.Second, KeepaliveTime: 10 * time.Second, KeepaliveTimeout: 3 * time.Second},
			},
			want: ":9999",
		},
		//{
		//	name: "test with TLS",
		//	args : args{
		//		conf: PeerConfig{
		//			Host:       ":9999",
		//			UseTLS:     true,
		//			TlsPath:    "/home/vagrant/one_pc_test/deploy/e2ecli/crypto-config/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt",
		//		},
		//	},
		//	want: ":9999",
		//},
	}
	go fakeGRPCServer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPeer(tt.args.conf, tt.args.cryptoSuite)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPeerFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got.uri)
		})
	}
}

func TestEndorse(t *testing.T) {
	var wantedResp = &peer.ProposalResponse{Version: 9527}
	var prop = &peer.SignedProposal{}
	p := &Peer{
		node:    node{name: "org1peer0", orgName: "org1", uri: "org1.peer0.example.com"},
		pClient: getMockEndorserClient(wantedResp, nil),
	}
	resp, err := p.Endorse(prop, other)
	assert.Nil(t, err)
	assert.Equal(t, wantedResp, resp)
}

func TestEndorseFailures(t *testing.T) {
	var prop = &peer.SignedProposal{}
	var wantedErr = errors.New("endorse failure")
	p := &Peer{
		node:    node{name: "org1peer0", orgName: "org1", uri: "org1.peer0.example.com"},
		pClient: getMockEndorserClient(nil, wantedErr),
	}
	resp, err := p.Endorse(prop, other)
	assert.NotNil(t, err)
	assert.Equal(t, wantedErr, err)
	assert.Nil(t, resp)
}
