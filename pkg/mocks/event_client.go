// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	gosdk "github.com/hw09234/gosdk/pkg"
	mock "github.com/stretchr/testify/mock"

	parseBlock "github.com/hw09234/gosdk/pkg/parseBlock"
)

// EventClient is an autogenerated mock type for the EventClient type
type EventClient struct {
	mock.Mock
}

// CloseFilteredBlockListen provides a mock function with given fields:
func (_m *EventClient) CloseFilteredBlockListen() {
	_m.Called()
}

// CloseFullBlockListen provides a mock function with given fields:
func (_m *EventClient) CloseFullBlockListen() {
	_m.Called()
}

// Disconnect provides a mock function with given fields:
func (_m *EventClient) Disconnect() {
	_m.Called()
}

// GetCrypto provides a mock function with given fields:
func (_m *EventClient) GetCrypto() (gosdk.CryptoSuite, error) {
	ret := _m.Called()

	var r0 gosdk.CryptoSuite
	if rf, ok := ret.Get(0).(func() gosdk.CryptoSuite); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(gosdk.CryptoSuite)
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

// ListenEventFilterBlock provides a mock function with given fields: startNum, filterBlockCh
func (_m *EventClient) ListenEventFilterBlock(startNum uint64, filterBlockCh chan gosdk.FilteredBlockResponse) chan error {
	ret := _m.Called(startNum, filterBlockCh)

	var r0 chan error
	if rf, ok := ret.Get(0).(func(uint64, chan gosdk.FilteredBlockResponse) chan error); ok {
		r0 = rf(startNum, filterBlockCh)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan error)
		}
	}

	return r0
}

// ListenEventFullBlock provides a mock function with given fields: startNum, fullBlockCh
func (_m *EventClient) ListenEventFullBlock(startNum uint64, fullBlockCh chan parseBlock.Block) chan error {
	ret := _m.Called(startNum, fullBlockCh)

	var r0 chan error
	if rf, ok := ret.Get(0).(func(uint64, chan parseBlock.Block) chan error); ok {
		r0 = rf(startNum, fullBlockCh)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan error)
		}
	}

	return r0
}
