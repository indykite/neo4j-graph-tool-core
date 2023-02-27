// Copyright (c) 2023 IndyKite
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/neo4j/neo4j-go-driver/v4/neo4j (interfaces: Driver,Transaction,Result,ResultSummary)

// package migrator_test is a generated GoMock package.
package migrator_test

import (
	url "net/url"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	neo4j "github.com/neo4j/neo4j-go-driver/v4/neo4j"
	db "github.com/neo4j/neo4j-go-driver/v4/neo4j/db"
)

// MockDriver is a mock of Driver interface.
type MockDriver struct {
	ctrl     *gomock.Controller
	recorder *MockDriverMockRecorder
}

// MockDriverMockRecorder is the mock recorder for MockDriver.
type MockDriverMockRecorder struct {
	mock *MockDriver
}

// NewMockDriver creates a new mock instance.
func NewMockDriver(ctrl *gomock.Controller) *MockDriver {
	mock := &MockDriver{ctrl: ctrl}
	mock.recorder = &MockDriverMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDriver) EXPECT() *MockDriverMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockDriver) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockDriverMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockDriver)(nil).Close))
}

// NewSession mocks base method.
func (m *MockDriver) NewSession(arg0 neo4j.SessionConfig) neo4j.Session {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewSession", arg0)
	ret0, _ := ret[0].(neo4j.Session)
	return ret0
}

// NewSession indicates an expected call of NewSession.
func (mr *MockDriverMockRecorder) NewSession(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewSession", reflect.TypeOf((*MockDriver)(nil).NewSession), arg0)
}

// Session mocks base method.
func (m *MockDriver) Session(arg0 neo4j.AccessMode, arg1 ...string) (neo4j.Session, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Session", varargs...)
	ret0, _ := ret[0].(neo4j.Session)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Session indicates an expected call of Session.
func (mr *MockDriverMockRecorder) Session(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Session", reflect.TypeOf((*MockDriver)(nil).Session), varargs...)
}

// Target mocks base method.
func (m *MockDriver) Target() url.URL {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Target")
	ret0, _ := ret[0].(url.URL)
	return ret0
}

// Target indicates an expected call of Target.
func (mr *MockDriverMockRecorder) Target() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Target", reflect.TypeOf((*MockDriver)(nil).Target))
}

// VerifyConnectivity mocks base method.
func (m *MockDriver) VerifyConnectivity() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "VerifyConnectivity")
	ret0, _ := ret[0].(error)
	return ret0
}

// VerifyConnectivity indicates an expected call of VerifyConnectivity.
func (mr *MockDriverMockRecorder) VerifyConnectivity() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "VerifyConnectivity", reflect.TypeOf((*MockDriver)(nil).VerifyConnectivity))
}

// MockTransaction is a mock of Transaction interface.
type MockTransaction struct {
	ctrl     *gomock.Controller
	recorder *MockTransactionMockRecorder
}

// MockTransactionMockRecorder is the mock recorder for MockTransaction.
type MockTransactionMockRecorder struct {
	mock *MockTransaction
}

// NewMockTransaction creates a new mock instance.
func NewMockTransaction(ctrl *gomock.Controller) *MockTransaction {
	mock := &MockTransaction{ctrl: ctrl}
	mock.recorder = &MockTransactionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTransaction) EXPECT() *MockTransactionMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockTransaction) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockTransactionMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockTransaction)(nil).Close))
}

// Commit mocks base method.
func (m *MockTransaction) Commit() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Commit")
	ret0, _ := ret[0].(error)
	return ret0
}

// Commit indicates an expected call of Commit.
func (mr *MockTransactionMockRecorder) Commit() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Commit", reflect.TypeOf((*MockTransaction)(nil).Commit))
}

// Rollback mocks base method.
func (m *MockTransaction) Rollback() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Rollback")
	ret0, _ := ret[0].(error)
	return ret0
}

// Rollback indicates an expected call of Rollback.
func (mr *MockTransactionMockRecorder) Rollback() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rollback", reflect.TypeOf((*MockTransaction)(nil).Rollback))
}

// Run mocks base method.
func (m *MockTransaction) Run(arg0 string, arg1 map[string]interface{}) (neo4j.Result, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Run", arg0, arg1)
	ret0, _ := ret[0].(neo4j.Result)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Run indicates an expected call of Run.
func (mr *MockTransactionMockRecorder) Run(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Run", reflect.TypeOf((*MockTransaction)(nil).Run), arg0, arg1)
}

// MockResult is a mock of Result interface.
type MockResult struct {
	ctrl     *gomock.Controller
	recorder *MockResultMockRecorder
}

// MockResultMockRecorder is the mock recorder for MockResult.
type MockResultMockRecorder struct {
	mock *MockResult
}

// NewMockResult creates a new mock instance.
func NewMockResult(ctrl *gomock.Controller) *MockResult {
	mock := &MockResult{ctrl: ctrl}
	mock.recorder = &MockResultMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResult) EXPECT() *MockResultMockRecorder {
	return m.recorder
}

// Collect mocks base method.
func (m *MockResult) Collect() ([]*db.Record, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Collect")
	ret0, _ := ret[0].([]*db.Record)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Collect indicates an expected call of Collect.
func (mr *MockResultMockRecorder) Collect() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Collect", reflect.TypeOf((*MockResult)(nil).Collect))
}

// Consume mocks base method.
func (m *MockResult) Consume() (neo4j.ResultSummary, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Consume")
	ret0, _ := ret[0].(neo4j.ResultSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Consume indicates an expected call of Consume.
func (mr *MockResultMockRecorder) Consume() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Consume", reflect.TypeOf((*MockResult)(nil).Consume))
}

// Err mocks base method.
func (m *MockResult) Err() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Err")
	ret0, _ := ret[0].(error)
	return ret0
}

// Err indicates an expected call of Err.
func (mr *MockResultMockRecorder) Err() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Err", reflect.TypeOf((*MockResult)(nil).Err))
}

// Keys mocks base method.
func (m *MockResult) Keys() ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Keys")
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Keys indicates an expected call of Keys.
func (mr *MockResultMockRecorder) Keys() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Keys", reflect.TypeOf((*MockResult)(nil).Keys))
}

// Next mocks base method.
func (m *MockResult) Next() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Next indicates an expected call of Next.
func (mr *MockResultMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockResult)(nil).Next))
}

// NextRecord mocks base method.
func (m *MockResult) NextRecord(arg0 **db.Record) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NextRecord", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// NextRecord indicates an expected call of NextRecord.
func (mr *MockResultMockRecorder) NextRecord(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NextRecord", reflect.TypeOf((*MockResult)(nil).NextRecord), arg0)
}

// Record mocks base method.
func (m *MockResult) Record() *db.Record {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Record")
	ret0, _ := ret[0].(*db.Record)
	return ret0
}

// Record indicates an expected call of Record.
func (mr *MockResultMockRecorder) Record() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Record", reflect.TypeOf((*MockResult)(nil).Record))
}

// Single mocks base method.
func (m *MockResult) Single() (*db.Record, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Single")
	ret0, _ := ret[0].(*db.Record)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Single indicates an expected call of Single.
func (mr *MockResultMockRecorder) Single() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Single", reflect.TypeOf((*MockResult)(nil).Single))
}

// MockResultSummary is a mock of ResultSummary interface.
type MockResultSummary struct {
	ctrl     *gomock.Controller
	recorder *MockResultSummaryMockRecorder
}

// MockResultSummaryMockRecorder is the mock recorder for MockResultSummary.
type MockResultSummaryMockRecorder struct {
	mock *MockResultSummary
}

// NewMockResultSummary creates a new mock instance.
func NewMockResultSummary(ctrl *gomock.Controller) *MockResultSummary {
	mock := &MockResultSummary{ctrl: ctrl}
	mock.recorder = &MockResultSummaryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResultSummary) EXPECT() *MockResultSummaryMockRecorder {
	return m.recorder
}

// Counters mocks base method.
func (m *MockResultSummary) Counters() neo4j.Counters {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Counters")
	ret0, _ := ret[0].(neo4j.Counters)
	return ret0
}

// Counters indicates an expected call of Counters.
func (mr *MockResultSummaryMockRecorder) Counters() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Counters", reflect.TypeOf((*MockResultSummary)(nil).Counters))
}

// Database mocks base method.
func (m *MockResultSummary) Database() neo4j.DatabaseInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Database")
	ret0, _ := ret[0].(neo4j.DatabaseInfo)
	return ret0
}

// Database indicates an expected call of Database.
func (mr *MockResultSummaryMockRecorder) Database() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Database", reflect.TypeOf((*MockResultSummary)(nil).Database))
}

// Notifications mocks base method.
func (m *MockResultSummary) Notifications() []neo4j.Notification {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Notifications")
	ret0, _ := ret[0].([]neo4j.Notification)
	return ret0
}

// Notifications indicates an expected call of Notifications.
func (mr *MockResultSummaryMockRecorder) Notifications() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Notifications", reflect.TypeOf((*MockResultSummary)(nil).Notifications))
}

// Plan mocks base method.
func (m *MockResultSummary) Plan() neo4j.Plan {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Plan")
	ret0, _ := ret[0].(neo4j.Plan)
	return ret0
}

// Plan indicates an expected call of Plan.
func (mr *MockResultSummaryMockRecorder) Plan() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Plan", reflect.TypeOf((*MockResultSummary)(nil).Plan))
}

// Profile mocks base method.
func (m *MockResultSummary) Profile() neo4j.ProfiledPlan {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Profile")
	ret0, _ := ret[0].(neo4j.ProfiledPlan)
	return ret0
}

// Profile indicates an expected call of Profile.
func (mr *MockResultSummaryMockRecorder) Profile() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Profile", reflect.TypeOf((*MockResultSummary)(nil).Profile))
}

// Query mocks base method.
func (m *MockResultSummary) Query() neo4j.Query {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Query")
	ret0, _ := ret[0].(neo4j.Query)
	return ret0
}

// Query indicates an expected call of Query.
func (mr *MockResultSummaryMockRecorder) Query() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockResultSummary)(nil).Query))
}

// ResultAvailableAfter mocks base method.
func (m *MockResultSummary) ResultAvailableAfter() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResultAvailableAfter")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// ResultAvailableAfter indicates an expected call of ResultAvailableAfter.
func (mr *MockResultSummaryMockRecorder) ResultAvailableAfter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResultAvailableAfter", reflect.TypeOf((*MockResultSummary)(nil).ResultAvailableAfter))
}

// ResultConsumedAfter mocks base method.
func (m *MockResultSummary) ResultConsumedAfter() time.Duration {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResultConsumedAfter")
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// ResultConsumedAfter indicates an expected call of ResultConsumedAfter.
func (mr *MockResultSummaryMockRecorder) ResultConsumedAfter() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResultConsumedAfter", reflect.TypeOf((*MockResultSummary)(nil).ResultConsumedAfter))
}

// Server mocks base method.
func (m *MockResultSummary) Server() neo4j.ServerInfo {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Server")
	ret0, _ := ret[0].(neo4j.ServerInfo)
	return ret0
}

// Server indicates an expected call of Server.
func (mr *MockResultSummaryMockRecorder) Server() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Server", reflect.TypeOf((*MockResultSummary)(nil).Server))
}

// Statement mocks base method.
func (m *MockResultSummary) Statement() neo4j.Statement {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Statement")
	ret0, _ := ret[0].(neo4j.Statement)
	return ret0
}

// Statement indicates an expected call of Statement.
func (mr *MockResultSummaryMockRecorder) Statement() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Statement", reflect.TypeOf((*MockResultSummary)(nil).Statement))
}

// StatementType mocks base method.
func (m *MockResultSummary) StatementType() neo4j.StatementType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StatementType")
	ret0, _ := ret[0].(neo4j.StatementType)
	return ret0
}

// StatementType indicates an expected call of StatementType.
func (mr *MockResultSummaryMockRecorder) StatementType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StatementType", reflect.TypeOf((*MockResultSummary)(nil).StatementType))
}