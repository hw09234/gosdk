// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	gohfc "github.com/hw09234/gohfc/pkg"
	common "github.com/hyperledger/fabric-protos-go/common"

	mock "github.com/stretchr/testify/mock"

	peer "github.com/hyperledger/fabric-protos-go/peer"
)

// LedgerClient is an autogenerated mock type for the LedgerClient type
type LedgerClient struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *LedgerClient) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetBlockByNumber provides a mock function with given fields: channelName, blockNum
func (_m *LedgerClient) GetBlockByNumber(channelName string, blockNum uint64) (*common.Block, error) {
	ret := _m.Called(channelName, blockNum)

	var r0 *common.Block
	if rf, ok := ret.Get(0).(func(string, uint64) *common.Block); ok {
		r0 = rf(channelName, blockNum)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*common.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, uint64) error); ok {
		r1 = rf(channelName, blockNum)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByTxID provides a mock function with given fields: channelName, txID
func (_m *LedgerClient) GetBlockByTxID(channelName string, txID string) (*common.Block, error) {
	ret := _m.Called(channelName, txID)

	var r0 *common.Block
	if rf, ok := ret.Get(0).(func(string, string) *common.Block); ok {
		r0 = rf(channelName, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*common.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(channelName, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeight provides a mock function with given fields: channelName
func (_m *LedgerClient) GetBlockHeight(channelName string) (uint64, error) {
	ret := _m.Called(channelName)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(string) uint64); ok {
		r0 = rf(channelName)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(channelName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCrypto provides a mock function with given fields:
func (_m *LedgerClient) GetCrypto() (gohfc.CryptoSuite, error) {
	ret := _m.Called()

	var r0 gohfc.CryptoSuite
	if rf, ok := ret.Get(0).(func() gohfc.CryptoSuite); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(gohfc.CryptoSuite)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNewestBlock provides a mock function with given fields: channelName
func (_m *LedgerClient) GetNewestBlock(channelName string) (*common.Block, error) {
	ret := _m.Called(channelName)

	var r0 *common.Block
	if rf, ok := ret.Get(0).(func(string) *common.Block); ok {
		r0 = rf(channelName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*common.Block)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(channelName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionByTxID provides a mock function with given fields: channelName, txID
func (_m *LedgerClient) GetTransactionByTxID(channelName string, txID string) (*peer.ProcessedTransaction, error) {
	ret := _m.Called(channelName, txID)

	var r0 *peer.ProcessedTransaction
	if rf, ok := ret.Get(0).(func(string, string) *peer.ProcessedTransaction); ok {
		r0 = rf(channelName, txID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*peer.ProcessedTransaction)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(channelName, txID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}