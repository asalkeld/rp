// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/jim-minter/rp/pkg/env (interfaces: Interface)

// Package mock_env is a generated GoMock package.
package mock_env

import (
	context "context"
	net "net"
	http "net/http"
	reflect "reflect"

	autorest "github.com/Azure/go-autorest/autorest"
	gomock "github.com/golang/mock/gomock"

	dns "github.com/jim-minter/rp/pkg/env/shared/dns"
)

// MockInterface is a mock of Interface interface
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// Authenticated mocks base method
func (m *MockInterface) Authenticated(arg0 http.Handler) http.Handler {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authenticated", arg0)
	ret0, _ := ret[0].(http.Handler)
	return ret0
}

// Authenticated indicates an expected call of Authenticated
func (mr *MockInterfaceMockRecorder) Authenticated(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authenticated", reflect.TypeOf((*MockInterface)(nil).Authenticated), arg0)
}

// CosmosDB mocks base method
func (m *MockInterface) CosmosDB(arg0 context.Context) (string, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CosmosDB", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CosmosDB indicates an expected call of CosmosDB
func (mr *MockInterfaceMockRecorder) CosmosDB(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CosmosDB", reflect.TypeOf((*MockInterface)(nil).CosmosDB), arg0)
}

// DNS mocks base method
func (m *MockInterface) DNS() dns.Manager {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DNS")
	ret0, _ := ret[0].(dns.Manager)
	return ret0
}

// DNS indicates an expected call of DNS
func (mr *MockInterfaceMockRecorder) DNS() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DNS", reflect.TypeOf((*MockInterface)(nil).DNS))
}

// FPAuthorizer mocks base method
func (m *MockInterface) FPAuthorizer(arg0 context.Context, arg1 string) (autorest.Authorizer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FPAuthorizer", arg0, arg1)
	ret0, _ := ret[0].(autorest.Authorizer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FPAuthorizer indicates an expected call of FPAuthorizer
func (mr *MockInterfaceMockRecorder) FPAuthorizer(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FPAuthorizer", reflect.TypeOf((*MockInterface)(nil).FPAuthorizer), arg0, arg1)
}

// IsReady mocks base method
func (m *MockInterface) IsReady() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsReady")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsReady indicates an expected call of IsReady
func (mr *MockInterfaceMockRecorder) IsReady() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsReady", reflect.TypeOf((*MockInterface)(nil).IsReady))
}

// ListenTLS mocks base method
func (m *MockInterface) ListenTLS(arg0 context.Context) (net.Listener, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListenTLS", arg0)
	ret0, _ := ret[0].(net.Listener)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListenTLS indicates an expected call of ListenTLS
func (mr *MockInterfaceMockRecorder) ListenTLS(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListenTLS", reflect.TypeOf((*MockInterface)(nil).ListenTLS), arg0)
}

// Location mocks base method
func (m *MockInterface) Location() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Location")
	ret0, _ := ret[0].(string)
	return ret0
}

// Location indicates an expected call of Location
func (mr *MockInterfaceMockRecorder) Location() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Location", reflect.TypeOf((*MockInterface)(nil).Location))
}

// PullSecret mocks base method
func (m *MockInterface) PullSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PullSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// PullSecret indicates an expected call of PullSecret
func (mr *MockInterfaceMockRecorder) PullSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PullSecret", reflect.TypeOf((*MockInterface)(nil).PullSecret))
}

// ResourceGroup mocks base method
func (m *MockInterface) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup
func (mr *MockInterfaceMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockInterface)(nil).ResourceGroup))
}