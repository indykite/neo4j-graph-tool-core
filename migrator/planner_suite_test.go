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

package migrator_test

import (
	"context"
	"fmt"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/onsi/gomega/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlanner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Migration Suite")
}

type matcherWrapper struct {
	matcher types.GomegaMatcher
	// This is used to save variable between calls to Matches and String in case of error
	// to be able to print better messages on failure
	actual interface{}
}

func WrapMatcher(matcher types.GomegaMatcher) gomock.Matcher {
	return &matcherWrapper{matcher: matcher}
}

func (m *matcherWrapper) Matches(x interface{}) (ok bool) {
	m.actual = x
	var err error
	if ok, err = m.matcher.Match(x); err != nil {
		ok = false
	}
	return
}

func (m *matcherWrapper) String() string {
	return fmt.Sprintf("Wrapped Gomega fail message: %s", m.matcher.FailureMessage(m.actual))
}

type MockSession struct {
	neo4j.SessionWithContext
	tx neo4j.ManagedTransaction
}

func (*MockSession) LastBookmarks() neo4j.Bookmarks {
	return nil
}

func (ms *MockSession) BeginTransaction(
	_ context.Context,
	_ ...func(*neo4j.TransactionConfig),
) (neo4j.ExplicitTransaction, error) {
	panic("BeginTransaction is not supported")
}

func (ms *MockSession) ExecuteRead(
	ctx context.Context,
	work neo4j.ManagedTransactionWork,
	_ ...func(*neo4j.TransactionConfig),
) (interface{}, error) {
	return work(ms.tx)
}
func (ms *MockSession) ExecuteWrite(
	ctx context.Context,
	work neo4j.ManagedTransactionWork,
	_ ...func(*neo4j.TransactionConfig),
) (interface{}, error) {
	return work(ms.tx)
}

func (ms *MockSession) Run(
	_ context.Context,
	_ string,
	_ map[string]any,
	configurers ...func(*neo4j.TransactionConfig),
) (neo4j.ResultWithContext, error) {
	panic("Run is not supported")
}

func (ms *MockSession) Close(_ context.Context) error {
	return nil
}

var _ neo4j.SessionWithContext = &MockSession{}

// func Neo4jSession(tx test.ExplicitTransaction) test.SessionWithContext {

// 	return &MockSession{tx: tx}
// }
