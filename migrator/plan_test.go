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
	"errors"

	"github.com/Masterminds/semver/v3"

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	v100 = semver.MustParse("v1.0.0")
	v101 = semver.MustParse("v1.0.1")
	v102 = semver.MustParse("v1.0.2")
	v103 = semver.MustParse("v1.0.3")
	v110 = semver.MustParse("v1.1.0")
)

type builderOperation struct {
	version    string
	folderName string
	path       string
	timestamp  int64
	isSnapshot bool
}

func newBuilderOp(version, folderName, path string, timestamp int64, isSnapshot bool) builderOperation {
	return builderOperation{
		version:    version,
		folderName: folderName,
		path:       path,
		timestamp:  timestamp,
		isSnapshot: isSnapshot,
	}
}

func getDBGraphVersion(version *semver.Version, files ...int64) migrator.DatabaseGraphVersion {
	v := migrator.DatabaseGraphVersion{
		Version:        version,
		FileTimestamps: make(map[int64]bool),
	}
	for _, f := range files {
		v.FileTimestamps[f] = true
	}
	return v
}

var _ = Describe("Plan", func() {
	var (
		vf      migrator.LocalFolders
		planner *migrator.Planner
	)

	BeforeEach(func() {
		c := &config.Config{Planner: &config.Planner{
			BaseFolder:   "import",
			SchemaFolder: &config.SchemaFolder{FolderName: "schema", MigrationType: config.DefaultSchemaMigrationType},
			Folders: map[string]*config.FolderDetail{
				"data": {MigrationType: config.DefaultFolderMigrationType, NodeLabels: []string{"DataVersion"}},
				"perf": {MigrationType: "up_down"},
			},
			Batches: map[string]*config.BatchDetail{
				"seed":      {Folders: []string{"data"}},
				"perf-seed": {Folders: []string{"data", "perf"}},
			},
		}}
		err := c.Normalize()
		Expect(err).To(Succeed())

		planner, err = migrator.NewPlanner(c)
		Expect(err).To(Succeed())

		s, err := planner.NewScanner("testdata/import")
		Expect(err).To(Succeed())

		vf, err = s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(vf).To(HaveLen(4))
	})

	It("With no config", func() {
		p, err := migrator.NewPlanner(nil)
		Expect(err).To(MatchError("missing config"))
		Expect(p).To(BeNil())
	})

	It("With unknown batch", func() {
		err := planner.Plan(
			vf,
			nil,
			&migrator.TargetVersion{Version: v101},
			"super-duper-batch",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				return nil
			})
		Expect(err).To(MatchError("unknown batch name 'super-duper-batch'"))
	})

	It("When builder fails", func() {
		err := planner.Plan(
			vf,
			nil,
			&migrator.TargetVersion{Version: v101},
			"schema",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				return errors.New("something went wrong")
			})
		Expect(err).To(MatchError("something went wrong"))

		err = planner.Plan(
			vf,
			migrator.DatabaseModel{
				"schema": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1000, 2000),
				},
			},
			&migrator.TargetVersion{Version: v100, Revision: 100},
			"schema",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				return errors.New("something went wrong again")
			})
		Expect(err).To(MatchError("something went wrong again"))
	})

	It("No Change", func() {
		migrations := 0
		err := planner.Plan(
			vf,
			migrator.DatabaseModel{
				"schema": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1000, 2000),
					getDBGraphVersion(v101, 1200, 1500),
				},
				"data": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1400),
					getDBGraphVersion(v101, 1300, 1400, 4800),
				},
				"perf": []migrator.DatabaseGraphVersion{getDBGraphVersion(v101, 1350, 2800)},
			},
			&migrator.TargetVersion{Version: v101},
			"perf-seed",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				migrations++
				return nil
			})
		Expect(err).To(Succeed())
		Expect(migrations).To(Equal(0))
	})

	It("Up one", func() {
		var ops []builderOperation
		err := planner.Plan(
			vf,
			migrator.DatabaseModel{
				"schema": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1000, 2000),
					getDBGraphVersion(v101, 1200, 1500),
					getDBGraphVersion(v102, 1850, 2200),
				},
				"data": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1400),
					getDBGraphVersion(v101, 1300, 1400, 4800),
				},
				"perf": []migrator.DatabaseGraphVersion{getDBGraphVersion(v101, 1350, 2800)},
			},
			&migrator.TargetVersion{Version: v102},
			"perf-seed",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				ops = append(ops, newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
				return nil
			})
		Expect(err).To(Succeed())
		Expect(ops).To(Equal([]builderOperation{
			newBuilderOp("1.0.2", "perf", "testdata/import/perf/v1.0.2/2010_up_p100.cypher", 2010, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2100_up_session.cypher", 2100, false),
			newBuilderOp("1.0.2", "perf", "testdata/import/perf/v1.0.2/2500_up_test_cmd.run", 2500, false),
		}))
	})

	It("Down one revision", func() {
		var ops []builderOperation
		err := planner.Plan(
			vf,
			migrator.DatabaseModel{
				"schema": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1000, 2000),
					getDBGraphVersion(v101, 1200, 1500),
					getDBGraphVersion(v102, 1850, 2100, 2200),
				},
			},
			&migrator.TargetVersion{Version: v102, Revision: 2100},
			"schema",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				ops = append(ops, newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
				return nil
			})
		Expect(err).To(Succeed())
		Expect(ops).To(Equal([]builderOperation{
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2200_down_test.cypher", 2200, false),
		}))
	})

	DescribeTable("Down one version",
		func(revision int, builderOps []builderOperation) {
			var ops []builderOperation
			err := planner.Plan(
				vf,
				migrator.DatabaseModel{
					"schema": []migrator.DatabaseGraphVersion{
						getDBGraphVersion(v100, 1000, 2000),
						getDBGraphVersion(v101, 1200, 1500),
						getDBGraphVersion(v102, 1850, 2100, 2200),
					},
					"data": []migrator.DatabaseGraphVersion{
						getDBGraphVersion(v100, 1400),
						getDBGraphVersion(v101, 1300, 1400, 4800),
					},
					"perf": []migrator.DatabaseGraphVersion{getDBGraphVersion(v101, 1350, 2800)},
				},
				&migrator.TargetVersion{Version: v101, Revision: int64(revision)},
				"perf-seed",
				func(cf *migrator.MigrationFile, version *semver.Version) error {
					ops = append(
						ops,
						newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
					return nil
				})
			Expect(err).To(Succeed())
			Expect(ops).To(Equal(builderOps))
		},
		Entry("With revision", 2000, []builderOperation{
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2200_down_test.cypher", 2200, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2100_down_session.cypher", 2100, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/1850_down_plan.cypher", 1850, false),
			newBuilderOp("1.0.1", "perf", "testdata/import/perf/v1.0.1/2800_down_contracts_2000.cypher", 2800, false),
		}),
		Entry("Without revision - taking latest", 0, []builderOperation{
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2200_down_test.cypher", 2200, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2100_down_session.cypher", 2100, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/1850_down_plan.cypher", 1850, false),
		}),
	)

	It("Up with all from version - ignore outdated data v1.0.0", func() {
		var ops []builderOperation
		err := planner.Plan(
			vf,
			migrator.DatabaseModel{
				"schema": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v100, 1000, 2000),
					getDBGraphVersion(v101, 1200, 1500),
				},
				"data": []migrator.DatabaseGraphVersion{
					getDBGraphVersion(v101, 1300, 1400),
				},
				"perf": []migrator.DatabaseGraphVersion{getDBGraphVersion(v101, 2800)},
			},
			nil,
			"perf-seed",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				ops = append(ops, newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
				return nil
			})
		Expect(err).To(Succeed())
		Expect(ops).To(Equal([]builderOperation{
			newBuilderOp("1.0.1", "perf", "testdata/import/perf/v1.0.1/1350_up_plansx1000.cypher", 1350, false),
			newBuilderOp("1.0.1", "data", "testdata/import/data/v1.0.1/4800_test_cmd.run", 4800, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/1850_up_plan.cypher", 1850, false),
			newBuilderOp("1.0.2", "perf", "testdata/import/perf/v1.0.2/2010_up_p100.cypher", 2010, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2100_up_session.cypher", 2100, false),
			newBuilderOp("1.0.2", "schema", "testdata/import/schema/v1.0.2/2200_up_test.cypher", 2200, false),
			newBuilderOp("1.0.2", "perf", "testdata/import/perf/v1.0.2/2500_up_test_cmd.run", 2500, false),
		}))
	})

	DescribeTable("Up version with empty DB without snapshots",
		func(revision int) {
			var ops []builderOperation
			err := planner.Plan(
				vf,
				nil,
				&migrator.TargetVersion{Version: v101, Revision: int64(revision)},
				"perf-seed",
				func(cf *migrator.MigrationFile, version *semver.Version) error {
					ops = append(
						ops,
						newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
					return nil
				})
			Expect(err).To(Succeed())
			Expect(ops).To(Equal([]builderOperation{
				newBuilderOp("1.0.0", "schema", "testdata/import/schema/v1.0.0/1000_up_core.cypher", 1000, false),
				newBuilderOp("1.0.0", "data", "testdata/import/data/v1.0.0/1400_test.cypher", 1400, false),
				newBuilderOp("1.0.0", "schema", "testdata/import/schema/v1.0.0/2000_up_test_cmd.run", 2000, false),
				newBuilderOp("1.0.1", "schema", "testdata/import/schema/v1.0.1/1200_up_plan.cypher", 1200, false),
				newBuilderOp("1.0.1", "data", "testdata/import/data/v1.0.1/1300_plans.cypher", 1300, false),
				newBuilderOp("1.0.1", "perf", "testdata/import/perf/v1.0.1/1350_up_plansx1000.cypher", 1350, false),
				newBuilderOp("1.0.1", "data", "testdata/import/data/v1.0.1/1400_contracts.cypher", 1400, false),
				newBuilderOp("1.0.1", "schema", "testdata/import/schema/v1.0.1/1500_up_contract.cypher", 1500, false),
				newBuilderOp("1.0.1", "perf", "testdata/import/perf/v1.0.1/2800_up_contracts_2000.cypher", 2800, false),
				newBuilderOp("1.0.1", "data", "testdata/import/data/v1.0.1/4800_test_cmd.run", 4800, false),
			}))
		},
		Entry("With revision", 5000),
		Entry("Without revision", 0),
	)

	DescribeTable("Testing snapshots in edge cases",
		func(batch string, targetVersion *migrator.TargetVersion, builderOps []builderOperation) {
			var ops []builderOperation
			err := planner.Plan(
				vf,
				nil,
				targetVersion,
				migrator.Batch(batch),
				func(cf *migrator.MigrationFile, version *semver.Version) error {
					ops = append(
						ops,
						newBuilderOp(version.String(), cf.FolderName, cf.Path, cf.Timestamp, cf.IsSnapshot))
					return nil
				})
			Expect(err).To(Succeed())
			Expect(ops).To(Equal(builderOps))
		},
		Entry("For schema batch planner can use snapshot if revision is not specified", "schema",
			&migrator.TargetVersion{Version: v100},
			[]builderOperation{
				newBuilderOp("1.0.0", "snapshots", "testdata/import/snapshots/schema_v1.0.0.cypher", 0, true),
			},
		),
		Entry("For schema batch planner cannot use snapshot if revision is specified", "schema",
			&migrator.TargetVersion{Version: v100, Revision: 5000},
			[]builderOperation{
				newBuilderOp("1.0.0", "schema", "testdata/import/schema/v1.0.0/1000_up_core.cypher", 1000, false),
				newBuilderOp("1.0.0", "schema", "testdata/import/schema/v1.0.0/2000_up_test_cmd.run", 2000, false),
			},
		),
		Entry("Schema uses only snapshot", "schema",
			&migrator.TargetVersion{Version: v101, Revision: 100},
			[]builderOperation{
				newBuilderOp("1.0.0", "snapshots", "testdata/import/snapshots/schema_v1.0.0.cypher", 0, true),
			},
		),
		Entry("Schema uses snapshot and some additional files", "schema",
			&migrator.TargetVersion{Version: v101, Revision: 1300},
			[]builderOperation{
				newBuilderOp("1.0.0", "snapshots", "testdata/import/snapshots/schema_v1.0.0.cypher", 0, true),
				newBuilderOp("1.0.1", "schema", "testdata/import/schema/v1.0.1/1200_up_plan.cypher", 1200, false),
			},
		),
		Entry("Schema+data uses snapshot and some additional files", "seed",
			&migrator.TargetVersion{Version: v101, Revision: 1300},
			[]builderOperation{
				newBuilderOp("1.0.0", "snapshots", "testdata/import/snapshots/seed_v1.0.0.run", 0, true),
				newBuilderOp("1.0.1", "schema", "testdata/import/schema/v1.0.1/1200_up_plan.cypher", 1200, false),
				newBuilderOp("1.0.1", "data", "testdata/import/data/v1.0.1/1300_plans.cypher", 1300, false),
			},
		),
		Entry("Schema+data+perf uses snapshot as there is no revision", "perf-seed",
			&migrator.TargetVersion{Version: v102},
			[]builderOperation{
				newBuilderOp("1.0.2", "snapshots", "testdata/import/snapshots/perf-seed_v1.0.2.cypher", 0, true),
			},
		),
		Entry("Schema+data+perf uses snapshot as there is no target version", "perf-seed",
			nil,
			[]builderOperation{
				newBuilderOp("1.0.2", "snapshots", "testdata/import/snapshots/perf-seed_v1.0.2.cypher", 0, true),
			},
		),
		Entry("Schema+data+perf uses snapshot as there is higher target version, but empty", "perf-seed",
			&migrator.TargetVersion{Version: v103},
			[]builderOperation{
				newBuilderOp("1.0.2", "snapshots", "testdata/import/snapshots/perf-seed_v1.0.2.cypher", 0, true),
			},
		),
	)

	It("Out of supported", func() {
		err := planner.Plan(
			vf,
			nil,
			&migrator.TargetVersion{Version: v110},
			"perf-seed",
			func(cf *migrator.MigrationFile, version *semver.Version) error {
				return nil
			})
		Expect(err).To(MatchError("specified target 1.1.0 version does not exist"))
	})
})
