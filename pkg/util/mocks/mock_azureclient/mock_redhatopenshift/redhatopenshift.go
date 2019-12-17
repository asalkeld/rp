// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jim-minter/rp/pkg/util/azureclient/redhatopenshift (interfaces: OpenShiftClustersClient)

// Package mock_redhatopenshift is a generated GoMock package.
package mock_redhatopenshift

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"

	redhatopenshift "github.com/jim-minter/rp/pkg/client/services/preview/redhatopenshift/mgmt/2019-12-31-preview/redhatopenshift"
)

// MockOpenShiftClustersClient is a mock of OpenShiftClustersClient interface
type MockOpenShiftClustersClient struct {
	ctrl     *gomock.Controller
	recorder *MockOpenShiftClustersClientMockRecorder
}

// MockOpenShiftClustersClientMockRecorder is the mock recorder for MockOpenShiftClustersClient
type MockOpenShiftClustersClientMockRecorder struct {
	mock *MockOpenShiftClustersClient
}

// NewMockOpenShiftClustersClient creates a new mock instance
func NewMockOpenShiftClustersClient(ctrl *gomock.Controller) *MockOpenShiftClustersClient {
	mock := &MockOpenShiftClustersClient{ctrl: ctrl}
	mock.recorder = &MockOpenShiftClustersClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockOpenShiftClustersClient) EXPECT() *MockOpenShiftClustersClientMockRecorder {
	return m.recorder
}

// CreateOrUpdateAndWait mocks base method
func (m *MockOpenShiftClustersClient) CreateOrUpdateAndWait(arg0 context.Context, arg1, arg2 string, arg3 redhatopenshift.OpenShiftCluster) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdateAndWait", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateOrUpdateAndWait indicates an expected call of CreateOrUpdateAndWait
func (mr *MockOpenShiftClustersClientMockRecorder) CreateOrUpdateAndWait(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdateAndWait", reflect.TypeOf((*MockOpenShiftClustersClient)(nil).CreateOrUpdateAndWait), arg0, arg1, arg2, arg3)
}

// DeleteAndWait mocks base method
func (m *MockOpenShiftClustersClient) DeleteAndWait(arg0 context.Context, arg1, arg2 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAndWait", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteAndWait indicates an expected call of DeleteAndWait
func (mr *MockOpenShiftClustersClientMockRecorder) DeleteAndWait(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAndWait", reflect.TypeOf((*MockOpenShiftClustersClient)(nil).DeleteAndWait), arg0, arg1, arg2)
}

// Get mocks base method
func (m *MockOpenShiftClustersClient) Get(arg0 context.Context, arg1, arg2 string) (redhatopenshift.OpenShiftCluster, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2)
	ret0, _ := ret[0].(redhatopenshift.OpenShiftCluster)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockOpenShiftClustersClientMockRecorder) Get(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockOpenShiftClustersClient)(nil).Get), arg0, arg1, arg2)
}

// GetCredentials mocks base method
func (m *MockOpenShiftClustersClient) GetCredentials(arg0 context.Context, arg1, arg2 string) (redhatopenshift.OpenShiftClusterCredentials, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCredentials", arg0, arg1, arg2)
	ret0, _ := ret[0].(redhatopenshift.OpenShiftClusterCredentials)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCredentials indicates an expected call of GetCredentials
func (mr *MockOpenShiftClustersClientMockRecorder) GetCredentials(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCredentials", reflect.TypeOf((*MockOpenShiftClustersClient)(nil).GetCredentials), arg0, arg1, arg2)
}