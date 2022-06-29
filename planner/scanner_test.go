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

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/planner"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type builderOperation struct {
	filename     string
	fileVersion  string
	writeVersion string
}

func newBuilderOp(filename, fileVersion, writeVersion string) builderOperation {
	return builderOperation{
		filename:     filename,
		fileVersion:  fileVersion,
		writeVersion: writeVersion,
	}
}

var _ = Describe("Scanner Errors", func() {
	var p *planner.Planner
	BeforeEach(func() {
		c, err := config.New()
		Expect(err).To(Succeed())

		p, err = planner.NewPlanner(c)
		Expect(err).To(Succeed())
	})

	It("Invalid folder", func() {
		_, err := p.NewScanner("testdata/none")
		Expect(err).To(MatchError(ContainSubstring("directory not exists: 'testdata/none'")))
	})

	It("Not a folder", func() {
		_, err := p.NewScanner("scanner_test.go")
		Expect(err).To(MatchError(ContainSubstring("scanner must point to a directory 'scanner_test.go'")))
	})

	It("Invalid Dir content", func() {
		t, err := p.NewScanner("testdata/case01")
		Expect(err).To(Succeed())
		_, err = t.ScanFolders()
		Expect(err).To(MatchError(
			"inconsistent state: missing down part of 'testdata/case01/schema/v1.0.1/02_up_contract.cypher'",
		))

		t, err = p.NewScanner("testdata/case02")
		Expect(err).To(Succeed())
		_, err = t.ScanFolders()
		Expect(err).To(MatchError("file 'testdata/case02/schema/v1.0.1/-1_up_plan.cypher' has invalid name"))

		t, err = p.NewScanner("testdata/case03")
		Expect(err).To(Succeed())
		_, err = t.ScanFolders()
		Expect(err).To(MatchError("forbidden number '0' at file 'testdata/case03/schema/v1.0.1/00_up_plan.cypher'"))
	})
})

var _ = Describe("Scanner", func() {
	var (
		vf planner.VersionFolders
		p  *planner.Planner
	)

	BeforeEach(func() {
		c := &config.Config{Planner: &config.Planner{
			BaseFolder:   "import",
			SchemaFolder: &config.SchemaFolder{FolderName: "schema", MigrationType: config.DefaultSchemaMigrationType},
			Folders: map[string]*config.FolderDetail{
				"data": {MigrationType: config.DefaultFolderMigrationType, NodeLabels: []string{"DataVersion"}},
				"perf": {MigrationType: config.DefaultFolderMigrationType},
			},
			Batches: map[string]*config.BatchDetail{
				"seed":      {Folders: []string{"data"}},
				"perf-seed": {Folders: []string{"data", "perf"}},
			},
		}}
		err := c.Normalize()
		Expect(err).To(Succeed())

		p, err = planner.NewPlanner(c)
		Expect(err).To(Succeed())

		s, err := p.NewScanner("testdata/import")
		Expect(err).To(Succeed())

		vf, err = s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(vf).To(HaveLen(3))
	})

	It("Error case", func() {
		dbSchemaVersion, err := planner.ParseGraphVersion("v0.1.0")
		Expect(err).To(Succeed())
		dbModel := planner.DatabaseModel{"schema": dbSchemaVersion}

		_, err = p.Upgrade(vf, dbModel, nil, "not-checked", nil)
		Expect(err).To(MatchError("out of range min:&1.0.0 low:0.1.0"))

		nv, err := p.Upgrade(vf, nil, dbSchemaVersion, "perf-seed", nil)
		Expect(nv).To(BeNil())
		Expect(err).To(Succeed())

		dbSchemaVersion, err = planner.ParseGraphVersion("v2.0.0")
		Expect(err).To(Succeed())
		dbModel = planner.DatabaseModel{"schema": dbSchemaVersion}

		_, err = p.Upgrade(vf, dbModel, nil, "perf", nil)
		Expect(err).To(MatchError(MatchRegexp("invalid range low:2.0.0 > high:1.0.2")))

		_, err = p.Upgrade(vf, nil, dbSchemaVersion, "not-checked", nil)
		Expect(err).To(MatchError("out of range max:1.0.2 high:2.0.0"))

		dbSchemaVersion, err = planner.ParseGraphVersion("v1.0.2+4")
		Expect(err).To(Succeed())
		_, err = p.Downgrade(vf, dbSchemaVersion, nil, 0, nil)
		Expect(err).To(MatchError(MatchRegexp("out of range: can't downgrade ver 1.0.2 from 4 only from 3")))
	})

	DescribeTable("Upgrade",
		func(model, data, perf, target string, expected []builderOperation) {
			var err error
			dbModel := planner.DatabaseModel{}

			if model != "" {
				dbModel["schema"], err = planner.ParseGraphVersion(model)
				Expect(err).To(Succeed())
			}
			if data != "" {
				dbModel["data"], err = planner.ParseGraphVersion(data)
				Expect(err).To(Succeed())
			}
			if perf != "" {
				dbModel["perf"], err = planner.ParseGraphVersion(perf)
				Expect(err).To(Succeed())
			}

			var to *planner.GraphVersion
			if target != "" {
				to, err = planner.ParseGraphVersion(target)
				Expect(err).To(Succeed())
			}

			var ops []builderOperation

			changed, err := p.Upgrade(vf, dbModel, to, "perf-seed",
				func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
					_, _ = fmt.Fprintf(GinkgoWriter,
						"'%s' (%s) -> folder '%s' file: '%s'\n", fileVer, writeVer, folder, cf.FilePath())
					ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
					return true, nil
				})
			Expect(err).To(Succeed())
			Expect(changed).To(Not(BeNil()))
			Expect(ops).To(Equal(expected))
		},
		Entry("Full", "", "", "", "", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/01_up_core.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/data/v1.0.0/01_test.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/03_up_test.cypher", "1.0.2+03", "1.0.2+03"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}),
		Entry("From v1.0.0-1", "1.0.0+01", "1.0.0+01", "", "", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/03_up_test.cypher", "1.0.2+03", "1.0.2+03"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}),
		Entry("From v1.0.0-1 Data v1.0.1-1", "1.0.0+01", "1.0.1+01", "1.0.1+01", "1.0.2+02", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}),

		Entry("From Data v1.0.0-1", "", "1.0.0+01", "", "", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/01_up_core.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/03_up_test.cypher", "1.0.2+03", "1.0.2+03"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}),

		Entry("To v1.0.1", "", "", "", "1.0.1", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/01_up_core.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/data/v1.0.0/01_test.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
		}),

		Entry("From v1.0.1-1 to v1.0.2-2", "1.0.1+01", "1.0.1+01", "1.0.1+01", "1.0.2+02", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}),
	)

	DescribeTable("Downgrade",
		func(from, to string, expected []builderOperation, after string) {
			var high, low, expVer *planner.GraphVersion
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

			var ops []builderOperation
			changed, err := p.Downgrade(vf, high, lowVer, lowRev,
				func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
					_, _ = fmt.Fprintf(GinkgoWriter,
						"'%s' (%s) -> folder '%s' file: '%s'\n", fileVer, writeVer, folder, cf.FilePath())
					ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
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
		Entry("Full", "", "", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_down_session.cypher", "1.0.2+02", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_down_plan.cypher", "1.0.2+01", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_down_contract.run", "1.0.1+02", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_down_plan.cypher", "1.0.1+01", "1.0.0+02"),
		}, "1.0.0+2"),

		Entry("Down from v1.0.1-1 to v1.0.0-02", "1.0.1+01", "1.0.0+02", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.1/01_down_plan.cypher", "1.0.1+01", "1.0.0+02"),
		}, "1.0.0+2"),

		Entry("Down to v1.0.1-1", "", "1.0.1+01", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_down_session.cypher", "1.0.2+02", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_down_plan.cypher", "1.0.2+01", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_down_contract.run", "1.0.1+02", "1.0.1+01"),
		}, "1.0.1+1"),

		Entry("Down from v1.0.2-2 to v1.0.1-1", "1.0.2+02", "1.0.1+01", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/02_down_session.cypher", "1.0.2+02", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_down_plan.cypher", "1.0.2+01", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_down_contract.run", "1.0.1+02", "1.0.1+01"),
		}, "1.0.1+1"),

		Entry("Down from v1.0.2-3 to v1.0.2-2", "1.0.2+03", "1.0.2+02", []builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
		}, "1.0.2+2"),
	)

	It("Create Upgrade plan", func() {
		buf := new(planner.ExecutionSteps)
		changed, err := p.Upgrade(vf, nil, nil, "perf-seed", p.CreateBuilder(buf, false))
		Ω(err).To(Succeed())
		Ω(changed).To(Not(BeNil()))
		plan := buf.String()
		Ω(plan).To(Equal(`// Importing folder schema - ver:1.0.0+01
:source testdata/import/schema/v1.0.0/01_up_core.cypher;
:param version => '1.0.0';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for folder schema - ver:1.0.0+02
>>> graph-tool jkl --text "some with spaces" --address ***** --username ***** --password *****
:param version => '1.0.0';
:param revision => 2;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder data - ver:1.0.0+01
:source testdata/import/data/v1.0.0/01_test.cypher;
:param version => '1.0.0';
:param revision => 1;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder schema - ver:1.0.1+01
:source testdata/import/schema/v1.0.1/01_up_plan.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder schema - ver:1.0.1+02
:source testdata/import/schema/v1.0.1/02_up_contract.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder data - ver:1.0.1+01
:source testdata/import/data/v1.0.1/01_plans.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder data - ver:1.0.1+02
:source testdata/import/data/v1.0.1/02_contracts.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for folder data - ver:1.0.1+03
>>> graph-tool abc -n 456 --address ***** --username ***** --password *****
>>> graph-tool jkl --address ***** --username ***** --password *****
:param version => '1.0.1';
:param revision => 3;
MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder perf - ver:1.0.1+01
:source testdata/import/perf/v1.0.1/01_plansx1000.cypher;
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:GraphToolMigration:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder perf - ver:1.0.1+02
:source testdata/import/perf/v1.0.1/02_contracts_2000.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:GraphToolMigration:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder schema - ver:1.0.2+01
:source testdata/import/schema/v1.0.2/01_up_plan.cypher;
:param version => '1.0.2';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder schema - ver:1.0.2+02
:source testdata/import/schema/v1.0.2/02_up_session.cypher;
:param version => '1.0.2';
:param revision => 2;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder schema - ver:1.0.2+03
:source testdata/import/schema/v1.0.2/03_up_test.cypher;
:param version => '1.0.2';
:param revision => 3;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Importing folder perf - ver:1.0.2+01
:source testdata/import/perf/v1.0.2/01_p100.cypher;
:param version => '1.0.2';
:param revision => 1;
MERGE (sm:GraphToolMigration:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for folder perf - ver:1.0.2+02
// Nothing to do in this file
:param version => '1.0.2';
:param revision => 2;
MERGE (sm:GraphToolMigration:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

`))
	})

	It("Create Downgrade plan", func() {
		buf := new(planner.ExecutionSteps)
		// "1.0.1+01", "1.0.0+02"
		dbModel := &planner.GraphVersion{
			Version:  v102,
			Revision: 2,
		}
		changed, err := p.Downgrade(vf, dbModel, v100, 1, p.CreateBuilder(buf, false))
		Ω(err).To(Succeed())
		Ω(changed).To(Not(BeNil()))
		plan := buf.String()
		Ω(plan).To(Equal(`// Running down of folder schema - ver:1.0.2+02
:source testdata/import/schema/v1.0.2/02_down_session.cypher;
:param version => '1.0.2';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running down of folder schema - ver:1.0.2+01
:source testdata/import/schema/v1.0.2/01_down_plan.cypher;
:param version => '1.0.1';
:param revision => 2;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running command for folder schema - ver:1.0.1+02
>>> graph-tool jkl --text "some with spaces" --address ***** --username ***** --password *****
:param version => '1.0.1';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running down of folder schema - ver:1.0.1+01
:source testdata/import/schema/v1.0.1/01_down_plan.cypher;
:param version => '1.0.0';
:param revision => 2;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

// Running down of folder schema - ver:1.0.0+02
:source testdata/import/schema/v1.0.0/02_down_test_cmd.cypher;
:param version => '1.0.0';
:param revision => 1;
MERGE (sm:GraphToolMigration:SchemaVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();

`))
	})
})
