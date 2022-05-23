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
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/indykite/neo4j-graph-tool-core/planner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Scanner Errors", func() {
	It("Invalid folder", func() {
		_, err := planner.NewScanner("testdata/none")
		Ω(err).To(HaveOccurred())
	})
	It("Not a folder", func() {
		_, err := planner.NewScanner("../planner/scanner.go")
		Ω(err).To(HaveOccurred())
	})
})

var _ = Describe("Scanner", func() {

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

	It("Error case", func() {
		ver, _ := planner.ParseGraphModel("v0.1.0", "", "")
		_, err := v.Upgrade(ver, nil, 0, planner.Perf, nil)
		Ω(err).To(MatchError(MatchRegexp("out of range min")))

		nv, err := v.Upgrade(nil, ver.Model.Version, 0, planner.Perf, nil)
		Ω(nv).To(BeNil())
		Ω(err).To(Succeed())

		ver, _ = planner.ParseGraphModel("v2.0.0", "", "")
		_, err = v.Upgrade(ver, nil, 0, planner.Perf, nil)
		Ω(err).To(MatchError(MatchRegexp("invalid range low:2.0.0 > high:1.0.2")))

		_, err = v.Upgrade(nil, ver.Model.Version, 0, planner.Perf, nil)
		Ω(err).To(MatchError(MatchRegexp("out of range max")))

		ver, _ = planner.ParseGraphModel("1.0.2+4", "", "")
		_, err = v.Downgrade(ver.Model, nil, 0, nil)
		Ω(err).To(MatchError(MatchRegexp("out of range: can't downgrade ver 1.0.2 from 4 only from 3")))
	})

	It("Invalid Dir content ", func() {
		t, err := planner.NewScanner("testdata/case01")
		Ω(err).To(Succeed())
		v, err = t.ScanGraphModel()
		Ω(err).To(MatchError(MatchRegexp("missing down part of")))
		t, err = planner.NewScanner("testdata/case02")
		Ω(err).To(Succeed())
		v, err = t.ScanGraphModel()
		Ω(err).To(MatchError(MatchRegexp("does not match with the name")))
		t, err = planner.NewScanner("testdata/case03")
		Ω(err).To(Succeed())
		v, err = t.ScanGraphModel()
		Ω(err).To(MatchError(MatchRegexp("forbidden number '0'")))
	})

	DescribeTable("Upgrade",
		func(model, data, perf, target string, expected []string) {
			current, err := planner.ParseGraphModel(model, data, perf)
			Ω(err).To(Succeed())

			var tVer *semver.Version
			var tRev uint64
			if target != "" {
				var to *planner.GraphState
				to, err = planner.ParseGraphVersion(target)
				Ω(err).To(Succeed())
				tVer = to.Version
				tRev = to.Revision
			}
			var ops []string
			changed, err := v.Upgrade(current, tVer, tRev,
				planner.Perf,
				func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
					_, _ = fmt.Fprintf(GinkgoWriter, "ver:%s -> op: %s\n", ver, cf)
					ops = append(ops, cf.FilePath())
					return true, nil
				})
			Ω(err).To(Succeed())
			Ω(changed).To(Not(BeNil()))
			Ω(ops).To(Equal(expected))
		},
		Entry("Full", "", "", "", "", []string{
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
			"testdata/import/schema/v1.0.2/01_up_plan.cypher",
			"testdata/import/schema/v1.0.2/02_up_session.cypher",
			"testdata/import/schema/v1.0.2/03_up_test.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}),
		Entry("From v1.0.0-1", "1.0.0+01", "1.0.0+01", "", "", []string{
			"testdata/import/schema/v1.0.0/02_up_test_cmd.run",
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
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
		}),
		Entry("From v1.0.0-1 Data v1.0.1-1", "1.0.0+01", "1.0.1+01", "1.0.1+01", "1.0.2+02", []string{
			"testdata/import/schema/v1.0.0/02_up_test_cmd.run",
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
			"testdata/import/schema/v1.0.2/01_up_plan.cypher",
			"testdata/import/schema/v1.0.2/02_up_session.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}),

		Entry("From Data v1.0.0-1", "", "1.0.0+01", "", "", []string{
			"testdata/import/schema/v1.0.0/01_up_core.cypher",
			"testdata/import/schema/v1.0.0/02_up_test_cmd.run",
			"testdata/import/schema/v1.0.1/01_up_plan.cypher",
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
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
		}),

		Entry("To v1.0.1", "", "", "", "1.0.1", []string{
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
		}),

		Entry("From v1.0.1-1 to v1.0.2-2", "1.0.1+01", "1.0.1+01", "1.0.1+01", "1.0.2+02", []string{
			"testdata/import/schema/v1.0.1/02_up_contract.cypher",
			"testdata/import/data/v1.0.1/02_contracts.cypher",
			"testdata/import/data/v1.0.1/03_test_cmd.run",
			"testdata/import/perf/v1.0.1/02_contracts_2000.cypher",
			"testdata/import/schema/v1.0.2/01_up_plan.cypher",
			"testdata/import/schema/v1.0.2/02_up_session.cypher",
			"testdata/import/perf/v1.0.2/01_p100.cypher",
			"testdata/import/perf/v1.0.2/02_test_cmd.run",
		}),
	)

	DescribeTable("Downgrade",
		func(from, to string, expected []string, after string) {
			var high, low, expVer *planner.GraphState
			var lowVer *semver.Version
			var lowRev uint64
			var err error
			if from != "" {
				high, err = planner.ParseGraphVersion(from)
				Ω(err).To(Succeed())
			}
			if to != "" {
				low, err = planner.ParseGraphVersion(to)
				Ω(err).To(Succeed())
				lowVer = low.Version
				lowRev = low.Revision
				Ω(to).To(Equal(low.String()))
			}
			if after != "" {
				expVer, err = planner.ParseGraphVersion(after)
				Ω(err).To(Succeed())
			}

			var ops []string
			changed, err := v.Downgrade(high, lowVer, lowRev,
				func(ver *semver.Version, cf *planner.CypherFile) (bool, error) {
					_, _ = fmt.Fprintf(GinkgoWriter, "ver:%s -> op: %s\n", ver, cf)
					ops = append(ops, cf.FilePath())
					return true, nil
				})
			Ω(err).To(Succeed())
			if expVer != nil {
				Ω(changed.Compare(expVer)).To(Equal(0))
			} else {
				Ω(changed).To(Not(BeNil()))
			}
			Ω(ops).To(Equal(expected))
		},
		Entry("Full", "", "", []string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
			"testdata/import/schema/v1.0.2/02_down_session.cypher",
			"testdata/import/schema/v1.0.2/01_down_plan.cypher",
			"testdata/import/schema/v1.0.1/02_down_contract.cypher",
			"testdata/import/schema/v1.0.1/01_down_plan.cypher",
		}, "1.0.0+2"),

		Entry("Down from v1.0.1-1 to v1.0.0-02", "1.0.1+01", "1.0.0+02", []string{
			"testdata/import/schema/v1.0.1/01_down_plan.cypher",
		}, "1.0.0+2"),

		Entry("Down to v1.0.1-1", "", "1.0.1+01", []string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
			"testdata/import/schema/v1.0.2/02_down_session.cypher",
			"testdata/import/schema/v1.0.2/01_down_plan.cypher",
			"testdata/import/schema/v1.0.1/02_down_contract.cypher",
		}, "1.0.1+1"),

		Entry("Down from v1.0.2-2 to v1.0.1-1", "1.0.2+02", "1.0.1+01", []string{
			"testdata/import/schema/v1.0.2/02_down_session.cypher",
			"testdata/import/schema/v1.0.2/01_down_plan.cypher",
			"testdata/import/schema/v1.0.1/02_down_contract.cypher",
		}, "1.0.1+1"),

		Entry("Down from v1.0.2-3 to v1.0.2-2", "1.0.2+03", "1.0.2+02", []string{
			"testdata/import/schema/v1.0.2/03_down_test.cypher",
		}, "1.0.2+2"),
	)

	It("Create Upgrade plan", func() {
		buf := new(planner.ExecutionSteps)
		changed, err := v.Upgrade(nil, nil, 0, planner.Perf, planner.CreatePlan(buf, false))
		Ω(err).To(Succeed())
		Ω(changed).To(Not(BeNil()))
		plan := buf.String()
		Ω(plan).To(Equal(`// Importing model - ver:1.0.0 rev:1
:source testdata/import/schema/v1.0.0/01_up_core.cypher;
:param version => '1.0.0';
:param revision => 1;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for model - ver:1.0.0 rev:2
>>> graph-tool jkl --text "some with spaces" --address ***** --username ***** --password *****
:param version => '1.0.0';
:param revision => 2;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing data - ver:1.0.0 rev:1
:source testdata/import/data/v1.0.0/01_test.cypher;
:param version => '1.0.0';
:param revision => 1;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing model - ver:1.0.1 rev:1
:source testdata/import/schema/v1.0.1/01_up_plan.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing model - ver:1.0.1 rev:2
:source testdata/import/schema/v1.0.1/02_up_contract.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing data - ver:1.0.1 rev:1
:source testdata/import/data/v1.0.1/01_plans.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing data - ver:1.0.1 rev:2
:source testdata/import/data/v1.0.1/02_contracts.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for data - ver:1.0.1 rev:3
>>> graph-tool abc -n 456 --address ***** --username ***** --password *****
>>> graph-tool jkl --address ***** --username ***** --password *****
:param version => '1.0.1';
:param revision => 3;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing perf - ver:1.0.1 rev:1
:source testdata/import/perf/v1.0.1/01_plansx1000.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing perf - ver:1.0.1 rev:2
:source testdata/import/perf/v1.0.1/02_contracts_2000.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing model - ver:1.0.2 rev:1
:source testdata/import/schema/v1.0.2/01_up_plan.cypher;
:param version => '1.0.2';
:param revision => 1;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing model - ver:1.0.2 rev:2
:source testdata/import/schema/v1.0.2/02_up_session.cypher;
:param version => '1.0.2';
:param revision => 2;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing model - ver:1.0.2 rev:3
:source testdata/import/schema/v1.0.2/03_up_test.cypher;
:param version => '1.0.2';
:param revision => 3;
MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing perf - ver:1.0.2 rev:1
:source testdata/import/perf/v1.0.2/01_p100.cypher;
:param version => '1.0.2';
:param revision => 1;
MERGE (sm:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for perf - ver:1.0.2 rev:2
// Nothing to do in this file
:param version => '1.0.2';
:param revision => 2;
MERGE (sm:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

`))
	})
})
