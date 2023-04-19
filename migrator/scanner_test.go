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
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/goccy/go-json"

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func newMigration(
	folder, path string,
	fileType migrator.FileType,
	timestamp int64,
	downgrade bool,
	isSnapshot bool,
) *migrator.MigrationFile {
	return &migrator.MigrationFile{
		FolderName:  folder,
		Path:        path,
		FileType:    fileType,
		Timestamp:   timestamp,
		IsDowngrade: downgrade,
		IsSnapshot:  isSnapshot,
	}
}

var _ = Describe("Scanner Errors", func() {
	var p *migrator.Planner
	BeforeEach(func() {
		c, err := config.New()
		Expect(err).To(Succeed())

		p, err = migrator.NewPlanner(c)
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

	It("Test folder is out of schema range", func() {
		c, err := config.New()
		Expect(err).To(Succeed())
		c.Planner.Folders = map[string]*config.FolderDetail{"test": {MigrationType: "change"}}

		p, err = migrator.NewPlanner(c)
		Expect(err).To(Succeed())

		t, err := p.NewScanner("testdata/case10")
		Expect(err).To(Succeed())
		_, err = t.ScanFolders()
		Expect(err).To(MatchError("unspecified schema for version of 'testdata/case10/test/v2.0.0'"))
	})

	DescribeTable("Invalid Dir content",
		func(folder, strErr string) {
			t, err := p.NewScanner(folder)
			Expect(err).To(Succeed())
			_, err = t.ScanFolders()
			Expect(err).To(MatchError(strErr))
		},
		Entry("No down for up file", "testdata/case01",
			"inconsistent state: missing down part of 'testdata/case01/schema/v1.0.1/200_up_contract.cypher'"),

		Entry("Invalid name", "testdata/case02",
			"file 'testdata/case02/schema/v1.0.1/-1_up_plan.cypher' has invalid name"),

		Entry("Starting with 0", "testdata/case03",
			"forbidden number '0' at file 'testdata/case03/schema/v1.0.1/00_up_plan.cypher'"),

		Entry("Is file, not directory", "testdata/case04", "open: testdata/case04/schema is not a directory"),

		Entry("Invalid semver name",
			"testdata/case05", "Invalid Semantic Version - testdata/case05/schema/v-not-semver"),

		Entry("More up files", "testdata/case06",
			"inconsistent state in 'testdata/case06/schema/v1.0.1': found 1 up and 0 down script"),

		Entry("Two files with same commit", "testdata/case07",
			"can't have two commit match '1' in folder 'testdata/case07/schema/v1.0.1'"),

		Entry("Two down files with same commit",
			"testdata/case08", "can't have two down commit match '1' in folder 'testdata/case08/schema/v1.0.1'"),

		Entry("Commit number too big", "testdata/case09",
			"strconv.ParseInt: parsing \"922337203685477580777\": value out of range"),

		Entry("Invalid folder name", "testdata/case11",
			"folder name '1.0.1' does not start with letter 'v' at testdata/case11/schema"),

		Entry("Non existing folder", "testdata/case12",
			"open testdata/case12/schema: no such file or directory"),

		Entry("Invalid folder name", "testdata/snapshot_case01",
			"open: testdata/snapshot_case01/snapshots is not a directory"),

		Entry("Invalid snapshot file", "testdata/snapshot_case02",
			"invalid snapshot name 'invalid_name.cypher'"),

		Entry("Invalid snapshot version", "testdata/snapshot_case03",
			"invalid snapshot version 'schema_v1.1.1.1.1.1.1.cypher': Invalid Semantic Version"),

		Entry("Invalid batch name", "testdata/snapshot_case04",
			"unknown batch name 'my_snapshot' based on snapshot name 'my_snapshot_v1.0.0.cypher'"),

		Entry("Unknown version", "testdata/snapshot_case05",
			"version '5.0.0' in snapshot 'schema_v5.0.0.cypher' is not defined in schema"),
	)
})

var _ = Describe("GraphVersion", func() {
	var v110, v200 *semver.Version
	var dbModel migrator.DatabaseModel

	BeforeEach(func() {
		v110 = semver.MustParse("1.1.0")
		v200 = semver.MustParse("2.0.0")

		dbModel = migrator.DatabaseModel{
			"schema": []migrator.DatabaseGraphVersion{
				{
					Version: v100,
					FileTimestamps: map[int64]bool{
						1677050000: true, 1677060000: true, 1677070001: true, 1677080000: true, 1677090001: true,
					},
				},
				{Version: v110, FileTimestamps: map[int64]bool{1677070000: true}},
				{Version: v200, FileTimestamps: map[int64]bool{1677090002: true}},
			},
			"test": []migrator.DatabaseGraphVersion{
				{Version: v100, FileTimestamps: map[int64]bool{1677090000: true}},
			},
		}
	})

	DescribeTable("ContainsHigherVersion",
		func(folder string, version *semver.Version, expectedResult bool) {
			res := dbModel.ContainsHigherVersion(folder, version)
			Expect(res).To(Equal(expectedResult))
		},
		Entry("Unknown folder", "abc", semver.MustParse("1.0.0"), false),
		Entry("Has higher version", "schema", semver.MustParse("1.0.0"), true),
		Entry("Has higher version", "schema", semver.MustParse("1.0.99"), true),
		Entry("Has higher version", "schema", semver.MustParse("1.1.0"), true),
		Entry("Doesn't have higher version", "schema", semver.MustParse("2.0.0"), false),
		Entry("Doesn't have higher test version", "test", semver.MustParse("1.0.0"), false),
	)
	It("ContainsHigherVersion returns false when DatabaseModel is nil", func() {
		var dbm migrator.DatabaseModel
		res := dbm.ContainsHigherVersion("schema", v100)
		Expect(res).To(BeFalse())
	})

	DescribeTable("GetFileTimestamps",
		func(folder string, version *semver.Version, expectedResult OmegaMatcher) {
			res := dbModel.GetFileTimestamps(folder, version)
			Expect(res).To(expectedResult)
		},
		Entry("Unknown folder", "abc", semver.MustParse("1.0.0"), BeNil()),
		Entry("Unknown version", "schema", semver.MustParse("1.0.99"), BeNil()),
		Entry("Has timestamps", "schema", semver.MustParse("1.0.0"), MatchAllKeys(Keys{
			int64(1677050000): BeTrue(),
			int64(1677060000): BeTrue(),
			int64(1677070001): BeTrue(),
			int64(1677080000): BeTrue(),
			int64(1677090001): BeTrue(),
		})),
	)
	It("GetFileTimestamps returns nil, when DatabaseModel is nil", func() {
		var dbm migrator.DatabaseModel
		res := dbm.GetFileTimestamps("schema", v100)
		Expect(res).To(BeNil())
	})

	It("HasAnyVersion", func() {
		Expect(dbModel.HasAnyVersion()).To(BeTrue())

		var nilDBModel migrator.DatabaseModel
		Expect(nilDBModel.HasAnyVersion()).To(BeFalse())
	})

	It("DatabaseModel to short String and MarshalJSON", func() {
		result := dbModel.String()
		Expect(result).To(MatchJSON(`{
			"schema": {
			  "1.0.0": ["... 2 more", 1677070001, 1677080000, 1677090001],
			  "1.1.0": [1677070000],
			  "2.0.0": [1677090002]
			},
			"test": {
			  "1.0.0": [1677090000]
			}
		  }`))
		Expect(result).NotTo(ContainSubstring("\n"))

		jsonResult, err := json.Marshal(dbModel)
		Expect(err).To(Succeed())
		Expect(jsonResult).To(MatchJSON(`{
			"schema": {
			  "1.0.0": [1677050000, 1677060000, 1677070001, 1677080000, 1677090001],
			  "1.1.0": [1677070000],
			  "2.0.0": [1677090002]
			},
			"test": {
			  "1.0.0": [1677090000]
			}
		  }`))
		Expect(jsonResult).NotTo(ContainSubstring("\n"))
	})
})

var _ = Describe("TargetVersion", func() {
	DescribeTable("ParseTargetVersion and String()",
		func(in string, errMatcher, valueMatcher OmegaMatcher) {
			ver, err := migrator.ParseTargetVersion(in)
			Expect(err).To(errMatcher)
			// There is no easy way how to test semver.Version. Just convert to String.
			Expect(ver.String()).To(valueMatcher)
		},
		Entry("invalid", "abc", MatchError("Invalid Semantic Version"), Equal("")),
		Entry("not numeric metadata", "1.2.3+beta1", MatchError("metadata are not numeric: 'beta1'"), Equal("")),
		Entry("no metadata", "11.750.22", Succeed(), Equal("11.750.22")),
		Entry("short metadata", "2.14.7+1", Succeed(), Equal("2.14.7+01")),
		Entry("long metadata", "14.0.578+1676650094", Succeed(), Equal("14.0.578+1676650094")),
	)

	It("Flag and PFlag helpers", func() {
		var gv *migrator.TargetVersion
		Expect(gv.Set("1.0.0")).To(MatchError("object is not initialized"))

		gv = &migrator.TargetVersion{}
		Expect(gv.Set("bla")).To(MatchError("Invalid Semantic Version"))

		Expect(gv.Set("22.45.99+456")).To(Succeed())
		Expect(gv.String()).To(Equal("22.45.99+456"))

		Expect(gv.Type()).To(Equal("GraphVersion"))
	})
})

var _ = Describe("Scanner", func() {
	It("ScanFolders", func() {
		plannerCfg := &config.Config{Planner: &config.Planner{
			BaseFolder: "import",
			SchemaFolder: &config.SchemaFolder{
				FolderName:    "schema",
				MigrationType: config.DefaultSchemaMigrationType,
			},
			AllowedCommands: map[string]string{"graph-tool": "/app/graph-tool"},
			Folders: map[string]*config.FolderDetail{
				"data": {MigrationType: config.DefaultFolderMigrationType, NodeLabels: []string{"DataVersion"}},
				"perf": {MigrationType: "up_down"},
			},
			Batches: map[string]*config.BatchDetail{
				"seed":      {Folders: []string{"data"}},
				"perf-seed": {Folders: []string{"data", "perf"}},
			},
		}}
		err := plannerCfg.Normalize()
		Expect(err).To(Succeed())

		p, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		s, err := p.NewScanner("testdata/import")
		Expect(err).To(Succeed())

		vf, err := s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(vf).To(HaveLen(4))

		vf.SortByVersion()

		Expect(vf[0]).To(Equal(
			&migrator.LocalVersionFolder{
				Version: semver.MustParse("v1.0.0"),
				SchemaFolder: &migrator.MigrationScripts{
					Up: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.0/1000_up_core.cypher",
							migrator.Cypher, 1000, false, false),
						newMigration("schema", "testdata/import/schema/v1.0.0/2000_up_test_cmd.run",
							migrator.Command, 2000, false, false),
					},
					Down: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.0/2000_down_test_cmd.cypher",
							migrator.Cypher, 2000, true, false),
						newMigration("schema", "testdata/import/schema/v1.0.0/1000_down_core.cypher",
							migrator.Cypher, 1000, true, false),
					},
				},
				ExtraFolders: map[string]*migrator.MigrationScripts{
					"data": {
						Up: []*migrator.MigrationFile{
							newMigration("data", "testdata/import/data/v1.0.0/1400_test.cypher",
								migrator.Cypher, 1400, false, false),
						},
					},
				},
				Snapshots: map[migrator.Batch]*migrator.MigrationFile{
					"seed": newMigration("snapshots", "testdata/import/snapshots/seed_v1.0.0.run",
						migrator.Command, 0, false, true),
					"schema": newMigration("snapshots", "testdata/import/snapshots/schema_v1.0.0.cypher",
						migrator.Cypher, 0, false, true),
				},
			},
		))

		Expect(vf[1]).To(Equal(
			&migrator.LocalVersionFolder{
				Version: semver.MustParse("v1.0.1"),
				SchemaFolder: &migrator.MigrationScripts{
					Up: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.1/1200_up_plan.cypher",
							migrator.Cypher, 1200, false, false),
						newMigration("schema", "testdata/import/schema/v1.0.1/1500_up_contract.cypher",
							migrator.Cypher, 1500, false, false),
					},
					Down: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.1/1500_down_contract.run",
							migrator.Command, 1500, true, false),
						newMigration("schema", "testdata/import/schema/v1.0.1/1200_down_plan.cypher",
							migrator.Cypher, 1200, true, false),
					},
				},
				ExtraFolders: map[string]*migrator.MigrationScripts{
					"data": {
						Up: []*migrator.MigrationFile{
							newMigration("data", "testdata/import/data/v1.0.1/1300_plans.cypher",
								migrator.Cypher, 1300, false, false),
							newMigration("data", "testdata/import/data/v1.0.1/1400_contracts.cypher",
								migrator.Cypher, 1400, false, false),
							newMigration("data", "testdata/import/data/v1.0.1/4800_test_cmd.run",
								migrator.Command, 4800, false, false),
						},
					},
					"perf": {
						Up: []*migrator.MigrationFile{
							newMigration("perf", "testdata/import/perf/v1.0.1/1350_up_plansx1000.cypher",
								migrator.Cypher, 1350, false, false),
							newMigration("perf", "testdata/import/perf/v1.0.1/2800_up_contracts_2000.cypher",
								migrator.Cypher, 2800, false, false),
						},
						Down: []*migrator.MigrationFile{
							newMigration("perf", "testdata/import/perf/v1.0.1/2800_down_contracts_2000.cypher",
								migrator.Cypher, 2800, true, false),
							newMigration("perf", "testdata/import/perf/v1.0.1/1350_down_plansx1000.cypher",
								migrator.Cypher, 1350, true, false),
						},
					},
				},
				Snapshots: make(map[migrator.Batch]*migrator.MigrationFile),
			},
		))

		Expect(vf[2]).To(Equal(
			&migrator.LocalVersionFolder{
				Version: semver.MustParse("v1.0.2"),
				SchemaFolder: &migrator.MigrationScripts{
					Up: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.2/1850_up_plan.cypher",
							migrator.Cypher, 1850, false, false),
						newMigration("schema", "testdata/import/schema/v1.0.2/2100_up_session.cypher",
							migrator.Cypher, 2100, false, false),
						newMigration("schema", "testdata/import/schema/v1.0.2/2200_up_test.cypher",
							migrator.Cypher, 2200, false, false),
					},
					Down: []*migrator.MigrationFile{
						newMigration("schema", "testdata/import/schema/v1.0.2/2200_down_test.cypher",
							migrator.Cypher, 2200, true, false),
						newMigration("schema", "testdata/import/schema/v1.0.2/2100_down_session.cypher",
							migrator.Cypher, 2100, true, false),
						newMigration("schema", "testdata/import/schema/v1.0.2/1850_down_plan.cypher",
							migrator.Cypher, 1850, true, false),
					},
				},
				ExtraFolders: map[string]*migrator.MigrationScripts{
					"perf": {
						Up: []*migrator.MigrationFile{
							newMigration("perf", "testdata/import/perf/v1.0.2/2010_up_p100.cypher",
								migrator.Cypher, 2010, false, false),
							newMigration("perf", "testdata/import/perf/v1.0.2/2500_up_test_cmd.run",
								migrator.Command, 2500, false, false),
						},
						Down: []*migrator.MigrationFile{
							newMigration("perf", "testdata/import/perf/v1.0.2/2500_down_test_cmd.run",
								migrator.Command, 2500, true, false),
							newMigration("perf", "testdata/import/perf/v1.0.2/2010_down_p100.cypher",
								migrator.Cypher, 2010, true, false),
						},
					},
				},
				Snapshots: map[migrator.Batch]*migrator.MigrationFile{
					"perf-seed": newMigration("snapshots", "testdata/import/snapshots/perf-seed_v1.0.2.cypher",
						migrator.Cypher, 0, false, true),
				},
			},
		))
	})

	It("ScanFolders with no Down for Schema, Up+Down for test and no snapshots", func() {
		plannerCfg := &config.Config{Planner: &config.Planner{
			BaseFolder: "schema_no_down_no_snapshots",
			SchemaFolder: &config.SchemaFolder{
				FolderName:    "base_schema",
				MigrationType: "change",
			},
			Folders: map[string]*config.FolderDetail{
				"seeding": {MigrationType: "up_down"},
			},
		}}
		err := plannerCfg.Normalize()
		Expect(err).To(Succeed())

		p, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		s, err := p.NewScanner("testdata/schema_no_down_no_snapshots")
		Expect(err).To(Succeed())

		vf, err := s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(vf).To(HaveLen(1))

		vf.SortByVersion()

		Expect(vf[0]).To(Equal(
			&migrator.LocalVersionFolder{
				Version: semver.MustParse("v1.0.0"),
				SchemaFolder: &migrator.MigrationScripts{
					Up: []*migrator.MigrationFile{
						newMigration("base_schema",
							"testdata/schema_no_down_no_snapshots/base_schema/v1.0.0/01_core.cypher",
							migrator.Cypher, 1, false, false),
					},
					Down: nil,
				},
				ExtraFolders: map[string]*migrator.MigrationScripts{
					"seeding": {
						Up: []*migrator.MigrationFile{
							newMigration("seeding",
								"testdata/schema_no_down_no_snapshots/seeding/v1.0.0/01_up_test.cypher",
								migrator.Cypher, 1, false, false),
							newMigration("seeding",
								"testdata/schema_no_down_no_snapshots/seeding/v1.0.0/02_up_test.cypher",
								migrator.Cypher, 2, false, false),
						},
						Down: []*migrator.MigrationFile{
							newMigration("seeding",
								"testdata/schema_no_down_no_snapshots/seeding/v1.0.0/02_down_test.cypher",
								migrator.Cypher, 2, true, false),
							newMigration("seeding",
								"testdata/schema_no_down_no_snapshots/seeding/v1.0.0/01_down_test.cypher",
								migrator.Cypher, 1, true, false),
						},
					},
				},
				Snapshots: map[migrator.Batch]*migrator.MigrationFile{},
			},
		))
	})
})

var _ = Describe("Writing migrations", func() {
	var scanner *migrator.Scanner
	const baseFolder = "testdata/import"

	BeforeEach(func() {
		plannerCfg := &config.Config{Planner: &config.Planner{
			BaseFolder: "import",
			SchemaFolder: &config.SchemaFolder{
				FolderName:    "schema",
				MigrationType: config.DefaultSchemaMigrationType,
			},
			AllowedCommands: map[string]string{"graph-tool": "/app/graph-tool"},
			Folders: map[string]*config.FolderDetail{
				"data":      {MigrationType: config.DefaultFolderMigrationType, NodeLabels: []string{"DataVersion"}},
				"not-exist": {MigrationType: "up_down"},
			},
		}}
		err := plannerCfg.Normalize()
		Expect(err).To(Succeed())

		p, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		scanner, err = p.NewScanner(baseFolder)
		Expect(err).To(Succeed())
	})

	DescribeTable("Error cases",
		func(folderName string, version *migrator.TargetVersion, expectedError OmegaMatcher) {
			paths, err := scanner.GenerateMigrationFiles(folderName, version, "name", migrator.Cypher, migrator.Cypher)
			Expect(err).To(expectedError)
			Expect(paths).To(BeNil())
		},
		Entry("non existing folder", "cc", nil, MatchError("folder does not exist: cc")),
		Entry("nil version", "schema", nil, MatchError("invalid version or revision")),
		Entry("empty version", "schema", &migrator.TargetVersion{}, MatchError("invalid version or revision")),
		Entry("zero revision", "schema",
			&migrator.TargetVersion{Version: v100}, MatchError("invalid version or revision")),
		Entry("not folder in filesystem", "not-exist",
			&migrator.TargetVersion{Version: v101, Revision: 1},
			MatchError("folder does not exist: not-exist")),
	)

	It("Write up and down files into existing folder", func() {
		expectedUpFile := baseFolder + "/schema/v1.0.2/8050_up_my-new-migration.cypher"
		expectedDownFile := baseFolder + "/schema/v1.0.2/8050_down_my-new-migration.run"
		notExpectedChangeFile := baseFolder + "/schema/v1.0.2/8050_my-new-migration.run"

		DeferCleanup(func() error {
			err1 := os.Remove(expectedUpFile)
			err2 := os.Remove(expectedDownFile)

			if err1 != nil {
				GinkgoWriter.Printf("failed to delete '%s': %s", expectedUpFile, err1.Error())
			}
			if err2 != nil {
				GinkgoWriter.Printf("failed to delete '%s': %s", expectedDownFile, err2.Error())
			}

			// Return error to fail the It()
			if err1 != nil {
				return err1
			}
			return err2
		})

		paths, err := scanner.GenerateMigrationFiles("schema", &migrator.TargetVersion{
			Version:  v102,
			Revision: 8050,
		}, "my-new-migration", migrator.Cypher, migrator.Command)
		Expect(err).To(Succeed())
		Expect(paths).To(ConsistOf(expectedUpFile, expectedDownFile))

		val, err := os.ReadFile(expectedUpFile)
		Expect(err).To(Succeed())
		Expect(val).To(BeEquivalentTo("return 1;\n"))

		val, err = os.ReadFile(expectedDownFile)
		Expect(err).To(Succeed())
		Expect(val).To(BeEquivalentTo("exit\n"))

		_, err = os.Stat(notExpectedChangeFile)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("Write only up file but create new version folder", func() {
		notExpectedUpFile := baseFolder + "/data/v1.0.2/8050_up_my-new-migration.cypher"
		notExpectedDownFile := baseFolder + "/data/v1.0.2/8050_down_my-new-migration.run"
		expectedChangeFile := baseFolder + "/data/v1.0.2/8050_my-new-migration.run"

		DeferCleanup(func() error {
			return os.RemoveAll(baseFolder + "/data/v1.0.2")
		})

		paths, err := scanner.GenerateMigrationFiles("data", &migrator.TargetVersion{
			Version:  v102,
			Revision: 8050,
		}, "my-new-migration", migrator.Command, migrator.Command)
		Expect(err).To(Succeed())
		Expect(paths).To(ConsistOf(expectedChangeFile))

		val, err := os.ReadFile(expectedChangeFile)
		Expect(err).To(Succeed())
		Expect(val).To(BeEquivalentTo("exit\n"))

		_, err = os.Stat(notExpectedUpFile)
		Expect(os.IsNotExist(err)).To(BeTrue(), "file exists: "+notExpectedUpFile)

		_, err = os.Stat(notExpectedDownFile)
		Expect(os.IsNotExist(err)).To(BeTrue(), "file exists: "+notExpectedDownFile)
	})
})
