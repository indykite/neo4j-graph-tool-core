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
	"github.com/Masterminds/semver/v3"

	"github.com/indykite/neo4j-graph-tool-core/planner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	v100, _ = semver.NewVersion("v1.0.0")
	v101, _ = semver.NewVersion("v1.0.1")
	v102, _ = semver.NewVersion("v1.0.2")
	v103, _ = semver.NewVersion("v1.0.3")
)

var _ = Describe("Plan", func() {
	var v planner.GraphVersions

	BeforeEach(func() {
		t, err := planner.NewScanner("testdata/import")
		Ω(err).To(Succeed())
		v, err = t.ScanGraphModel()
		Ω(err).To(Succeed())
		v, err = t.ScanData(v)
		Ω(err).To(Succeed())
		v, err = t.ScanPerfData(v)
		Ω(err).To(Succeed())
		Ω(v).To(HaveLen(3))
	})

	It("No Change", func() {
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v101, Revision: 2},
				Data:  &planner.GraphState{Version: v101, Revision: 3},
				Perf:  &planner.GraphState{Version: v101, Revision: 2},
			},
			&planner.GraphState{Version: v101},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(BeNil())
	})

	It("Up one", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v102, Revision: 2},
				Data:  &planner.GraphState{Version: v101, Revision: 2},
				Perf:  &planner.GraphState{Version: v101, Revision: 1},
			},
			&planner.GraphState{Version: v102},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v102}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.2/03_up_test.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}))
	})

	It("Down one", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v102, Revision: 3},
			},
			&planner.GraphState{Version: v102, Revision: 2},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v102, Revision: 2}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
		}))
	})

	It("Down one version", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v102, Revision: 3},
			},
			&planner.GraphState{Version: v101, Revision: 2},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v101, Revision: 2}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
			"testdata/import/schema/v1.0.2/02_down_session.cypher",
			"testdata/import/schema/v1.0.2/01_down_plan.cypher",
		}))

		ops = ops[:0]
		after, err = v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v102, Revision: 3},
			},
			&planner.GraphState{Version: v101},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v101, Revision: 2}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
			"testdata/import/schema/v1.0.2/02_down_session.cypher",
			"testdata/import/schema/v1.0.2/01_down_plan.cypher",
		}))
	})

	It("Up one from version", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v101, Revision: 2},
				Data:  &planner.GraphState{Version: v101, Revision: 3},
				Perf:  &planner.GraphState{Version: v101, Revision: 1},
			},
			nil,
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v102}))
		Ω(ops).To(Equal([]string{
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
			"testdata/import/schema/v1.0.2/01_up_plan.cypher",
			"testdata/import/schema/v1.0.2/02_up_session.cypher",
			"testdata/import/schema/v1.0.2/03_up_test.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}))
	})

	It("Up one to version", func() {
		var ops []string
		after, err := v.Plan(
			nil,
			&planner.GraphState{Version: v101, Revision: 1},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v101}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.0/01_up_core.cypher",
			"testdata/import/schema/v1.0.0/02_up_test_cmd.run",
			"testdata/import/data/v1.0.0/01_test.cypher",
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/data/v1.0.1/01_plans.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/01_plansx1000.cypher",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
		}))

		ops = ops[:0]
		after, err = v.Plan(
			nil,
			&planner.GraphState{Version: v101},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v101}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.0/01_up_core.cypher",
			"testdata/import/schema/v1.0.0/02_up_test_cmd.run",
			"testdata/import/data/v1.0.0/01_test.cypher",
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
			"testdata/import/data/v1.0.1/01_plans.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/01_plansx1000.cypher",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
		}))
	})

	It("Out of supported", func() {
		_, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v103, Revision: 2},
				Data:  &planner.GraphState{Version: v103, Revision: 2},
			},
			&planner.GraphState{Version: v103},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				return true, nil
			})
		Ω(err).To(HaveOccurred())
	})

	It("Up with Data", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v101, Revision: 2},
				Data:  &planner.GraphState{Version: v100, Revision: 1},
				Perf:  &planner.GraphState{Version: v101, Revision: 0},
			},
			&planner.GraphState{Version: v102},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v102}))
		Ω(ops).To(Equal([]string{
			"testdata/import/data/v1.0.1/01_plans.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/01_plansx1000.cypher",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
			"testdata/import/schema/v1.0.2/01_up_plan.cypher",
			"testdata/import/schema/v1.0.2/02_up_session.cypher",
			"testdata/import/schema/v1.0.2/03_up_test.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}))
	})

	It("Up with Unknown", func() {
		var ops []string
		after, err := v.Plan(
			&planner.GraphModel{
				Model: &planner.GraphState{Version: v101, Revision: 0},
			}, // &planner.GraphState{Version: v100, Revision: 1},
			&planner.GraphState{Version: v101},
			planner.Perf,
			func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
				ops = append(ops, cf.FilePath())
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphState{Version: v101}))
		Ω(ops).To(Equal([]string{
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
			"testdata/import/data/v1.0.1/01_plans.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/01_plansx1000.cypher",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
		}))
	})
})
