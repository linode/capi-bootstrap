// Code generated by MockGen. DO NOT EDIT.
// Source: providers/backend/types.go
//
// Generated by this command:
//
//	mockgen-v0.4.0 -destination=providers/backend/mock/mock_types.go -source=providers/backend/types.go
//

// Package mock_backend is a generated GoMock package.
package mock_backend

import (
	yaml "capi-bootstrap/yaml"
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

// MockProvider is a mock of Provider interface.
type MockProvider struct {
	ctrl     *gomock.Controller
	recorder *MockProviderMockRecorder
}

// MockProviderMockRecorder is the mock recorder for MockProvider.
type MockProviderMockRecorder struct {
	mock *MockProvider
}

// NewMockProvider creates a new mock instance.
func NewMockProvider(ctrl *gomock.Controller) *MockProvider {
	mock := &MockProvider{ctrl: ctrl}
	mock.recorder = &MockProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProvider) EXPECT() *MockProviderMockRecorder {
	return m.recorder
}

// Delete mocks base method.
func (m *MockProvider) Delete(ctx context.Context, clusterName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, clusterName)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockProviderMockRecorder) Delete(ctx, clusterName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockProvider)(nil).Delete), ctx, clusterName)
}

// ListClusters mocks base method.
func (m *MockProvider) ListClusters(arg0 context.Context) (map[string]*v1.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListClusters", arg0)
	ret0, _ := ret[0].(map[string]*v1.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListClusters indicates an expected call of ListClusters.
func (mr *MockProviderMockRecorder) ListClusters(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListClusters", reflect.TypeOf((*MockProvider)(nil).ListClusters), arg0)
}

// PreCmd mocks base method.
func (m *MockProvider) PreCmd(ctx context.Context, clusterName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PreCmd", ctx, clusterName)
	ret0, _ := ret[0].(error)
	return ret0
}

// PreCmd indicates an expected call of PreCmd.
func (mr *MockProviderMockRecorder) PreCmd(ctx, clusterName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PreCmd", reflect.TypeOf((*MockProvider)(nil).PreCmd), ctx, clusterName)
}

// Read mocks base method.
func (m *MockProvider) Read(ctx context.Context, clusterName string) (*v1.Config, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Read", ctx, clusterName)
	ret0, _ := ret[0].(*v1.Config)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Read indicates an expected call of Read.
func (mr *MockProviderMockRecorder) Read(ctx, clusterName any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Read", reflect.TypeOf((*MockProvider)(nil).Read), ctx, clusterName)
}

// WriteConfig mocks base method.
func (m *MockProvider) WriteConfig(ctx context.Context, clusterName string, config *v1.Config) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteConfig", ctx, clusterName, config)
	ret0, _ := ret[0].(error)
	return ret0
}

// WriteConfig indicates an expected call of WriteConfig.
func (mr *MockProviderMockRecorder) WriteConfig(ctx, clusterName, config any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteConfig", reflect.TypeOf((*MockProvider)(nil).WriteConfig), ctx, clusterName, config)
}

// WriteFiles mocks base method.
func (m *MockProvider) WriteFiles(ctx context.Context, clusterName string, cloudInitFile *yaml.Config) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WriteFiles", ctx, clusterName, cloudInitFile)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WriteFiles indicates an expected call of WriteFiles.
func (mr *MockProviderMockRecorder) WriteFiles(ctx, clusterName, cloudInitFile any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WriteFiles", reflect.TypeOf((*MockProvider)(nil).WriteFiles), ctx, clusterName, cloudInitFile)
}