// Copyright (c) 2022 IndyKite
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

package planner_test

import (
	"github.com/indykite/neo4j-graph-tool-core/planner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecutionStep methods", func() {
	var ess planner.ExecutionSteps
	BeforeEach(func() {
		ess = planner.ExecutionSteps{}
		ess.AddCommand([]string{"my-super-command", "arg1", "arg2"})
		ess.AddCypher("test cypher\n")
		ess.AddCypher("another cypher\n", "more cypher\n", "even more cyphers\n")
	})

	It("First element is command", func() {
		Expect(ess[0].Command()).To(ConsistOf([]string{"my-super-command", "arg1", "arg2"}))
		Expect(ess[0].IsCypher()).To(BeFalse())
		Expect(ess[0].Cypher()).To(BeNil())
	})

	It("Second element is cypher", func() {
		Expect(ess[1].Command()).To(BeNil())
		Expect(ess[1].IsCypher()).To(BeTrue())
		Expect(ess[1].Cypher().String()).To(Equal("test cypher\n" +
			"another cypher\n" +
			"more cypher\n" +
			"even more cyphers\n"))
	})
})

var _ = Describe("ExecutionSteps", func() {
	It("AddCypher without parameters does nothing", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCypher()
		Expect(ess).To(HaveLen(0))
	})

	It("AddCommand with empty parameters does nothing", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCommand([]string{})
		Expect(ess).To(HaveLen(0))
	})

	It("AddCypher is always adding to previous buffer", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCypher("first cypher;", "second cypher;")
		ess.AddCypher("third cypher;", "last cypher;")

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal("first cypher;second cypher;third cypher;last cypher;"))
	})

	It("AddCypher creates new step when there is command between", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCypher("first cypher;", "second cypher;")
		ess.AddCommand([]string{"my-super-command"})
		ess.AddCypher("third cypher;", "last cypher;")

		Expect(ess).To(HaveLen(3))
		Expect(ess.String()).To(Equal(
			"first cypher;second cypher;" +
				">>> my-super-command --address ***** --username ***** --password *****\n" +
				"third cypher;last cypher;"))
	})

	It("Exit command is special when printing out", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCommand([]string{"exit", "useless-arguments"})

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal("// Nothing to do in this file\n"))
	})

	It("Printing out command with spaces works properly", func() {
		ess := planner.ExecutionSteps{}
		ess.AddCommand([]string{"my-cmd", "with spaces", "another text with spaces"})

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal(
			`>>> my-cmd "with spaces" "another text with spaces" ` +
				"--address ***** --username ***** --password *****\n",
		))
	})
})
