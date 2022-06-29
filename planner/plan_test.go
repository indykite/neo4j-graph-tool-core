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

	"github.com/indykite/neo4j-graph-tool-core/config"
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

	It("No Change", func() {
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v101, Revision: 2},
				"data":   &planner.GraphVersion{Version: v101, Revision: 3},
				"perf":   &planner.GraphVersion{Version: v101, Revision: 2},
			},
			&planner.GraphVersion{Version: v101},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(BeNil())
	})

	It("Up one", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v102, Revision: 2},
				"data":   &planner.GraphVersion{Version: v101, Revision: 2},
				"perf":   &planner.GraphVersion{Version: v101, Revision: 1},
			},
			&planner.GraphVersion{Version: v102},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v102}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_up_test.cypher", "1.0.2+03", "1.0.2+03"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}))
	})

	It("Down one", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v102, Revision: 3},
			},
			&planner.GraphVersion{Version: v102, Revision: 2},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v102, Revision: 2}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
		}))
	})

	It("Down one version", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v102, Revision: 3},
			},
			&planner.GraphVersion{Version: v101, Revision: 2},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v101, Revision: 2}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_down_session.cypher", "1.0.2+02", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_down_plan.cypher", "1.0.2+01", "1.0.1+02"),
		}))

		ops = make([]builderOperation, 0)
		after, err = p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v102, Revision: 3},
			},
			&planner.GraphVersion{Version: v101},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v101, Revision: 2}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.2/03_down_test.cypher", "1.0.2+03", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_down_session.cypher", "1.0.2+02", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_down_plan.cypher", "1.0.2+01", "1.0.1+02"),
		}))
	})

	It("Up one from version", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v101, Revision: 2},
				"data":   &planner.GraphVersion{Version: v101, Revision: 3},
				"perf":   &planner.GraphVersion{Version: v101, Revision: 1},
			},
			nil,
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v102}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/01_up_plan.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/schema/v1.0.2/02_up_session.cypher", "1.0.2+02", "1.0.2+02"),
			newBuilderOp("testdata/import/schema/v1.0.2/03_up_test.cypher", "1.0.2+03", "1.0.2+03"),
			newBuilderOp("testdata/import/perf/v1.0.2/01_p100.cypher", "1.0.2+01", "1.0.2+01"),
			newBuilderOp("testdata/import/perf/v1.0.2/02_test_cmd.run", "1.0.2+02", "1.0.2+02"),
		}))
	})

	It("Up one to version", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			nil,
			&planner.GraphVersion{Version: v101, Revision: 1},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v101}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.0/01_up_core.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.0/02_up_test_cmd.run", "1.0.0+02", "1.0.0+02"),
			newBuilderOp("testdata/import/data/v1.0.0/01_test.cypher", "1.0.0+01", "1.0.0+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
		}))

		ops = make([]builderOperation, 0)
		after, err = p.Plan(
			vf,
			nil,
			&planner.GraphVersion{Version: v101},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v101}))
		Ω(ops).To(Equal([]builderOperation{
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
		}))
	})

	It("Out of supported", func() {
		_, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v103, Revision: 2},
				"data":   &planner.GraphVersion{Version: v103, Revision: 2},
			},
			&planner.GraphVersion{Version: v103},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				return true, nil
			})
		Ω(err).To(HaveOccurred())
	})

	It("Up with Data", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v101, Revision: 2},
				"data":   &planner.GraphVersion{Version: v100, Revision: 1},
				"perf":   &planner.GraphVersion{Version: v101, Revision: 0},
			},
			&planner.GraphVersion{Version: v102},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v102}))
		Ω(ops).To(Equal([]builderOperation{
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
		}))
	})

	It("Up with Unknown", func() {
		var ops []builderOperation
		after, err := p.Plan(
			vf,
			planner.DatabaseModel{
				"schema": &planner.GraphVersion{Version: v101, Revision: 0},
			}, // &planner.GraphVersion{Version: v100, Revision: 1},
			&planner.GraphVersion{Version: v101},
			"perf-seed",
			func(folder string, cf *planner.MigrationFile, fileVer, writeVer *planner.GraphVersion) (bool, error) {
				ops = append(ops, newBuilderOp(cf.FilePath(), fileVer.String(), writeVer.String()))
				return true, nil
			})
		Ω(err).To(Succeed())
		Ω(after).To(Equal(&planner.GraphVersion{Version: v101}))
		Ω(ops).To(Equal([]builderOperation{
			newBuilderOp("testdata/import/schema/v1.0.1/01_up_plan.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/schema/v1.0.1/02_up_contract.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/01_plans.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/data/v1.0.1/02_contracts.cypher", "1.0.1+02", "1.0.1+02"),
			newBuilderOp("testdata/import/data/v1.0.1/03_test_cmd.run", "1.0.1+03", "1.0.1+03"),
			newBuilderOp("testdata/import/perf/v1.0.1/01_plansx1000.cypher", "1.0.1+01", "1.0.1+01"),
			newBuilderOp("testdata/import/perf/v1.0.1/02_contracts_2000.cypher", "1.0.1+02", "1.0.1+02"),
		}))
	})
})
