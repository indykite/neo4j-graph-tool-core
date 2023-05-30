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

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecutionStep methods", func() {
	var ess migrator.ExecutionSteps
	BeforeEach(func() {
		ess = migrator.ExecutionSteps{}
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
		ess := migrator.ExecutionSteps{}
		ess.AddCypher()
		Expect(ess).To(HaveLen(0))
	})

	It("AddCommand with empty parameters does nothing", func() {
		ess := migrator.ExecutionSteps{}
		ess.AddCommand([]string{})
		Expect(ess).To(HaveLen(0))
	})

	It("AddCypher is always adding to previous buffer", func() {
		ess := migrator.ExecutionSteps{}
		ess.AddCypher("first cypher;", "second cypher;")
		ess.AddCypher("third cypher;", "last cypher;")

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal("first cypher;second cypher;third cypher;last cypher;"))
	})

	It("AddCypher creates new step when there is command between", func() {
		ess := migrator.ExecutionSteps{}
		ess.AddCypher("first cypher;", "second cypher;")
		ess.AddCommand([]string{"my-super-command"})
		ess.AddCypher("third cypher;", "last cypher;")

		Expect(ess).To(HaveLen(3))
		Expect(ess.String()).To(Equal(
			"first cypher;second cypher;" +
				">>> my-super-command\n" +
				"third cypher;last cypher;"))
	})

	It("Exit command is special when printing out", func() {
		ess := migrator.ExecutionSteps{}
		ess.AddCommand([]string{"exit", "useless-arguments"})

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal("// Nothing to do in this file\n"))
	})

	It("Printing out command with spaces works properly", func() {
		var ess migrator.ExecutionSteps
		Expect(ess.IsEmpty()).To(BeTrue())
		ess = migrator.ExecutionSteps{}
		Expect(ess.IsEmpty()).To(BeTrue())

		ess.AddCommand([]string{"my-cmd", "with spaces", "another text with spaces"})

		Expect(ess).To(HaveLen(1))
		Expect(ess.String()).To(Equal(`>>> my-cmd "with spaces" "another text with spaces"` + "\n"))
	})
})

var _ = Describe("Default Builder with testing data", func() {
	var (
		p            *migrator.Planner
		plannerCfg   *config.Config
		localFolders migrator.LocalFolders
	)

	BeforeEach(func() {
		plannerCfg = &config.Config{Planner: &config.Planner{
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
		Expect(plannerCfg.Normalize()).To(Succeed())

		var err error
		p, err = migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		s, err := p.NewScanner("testdata/import")
		Expect(err).To(Succeed())

		localFolders, err = s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(localFolders).To(HaveLen(4))
	})

	It("Fails on not allowed command", func() {
		// Little hack to change config
		plannerCfg.Planner.AllowedCommands = nil

		pp, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		buf := new(migrator.ExecutionSteps)
		err = pp.Plan(localFolders, migrator.DatabaseModel{
			"schema": []migrator.DatabaseGraphVersion{
				getDBGraphVersion(v100, 1000, 2000),
			},
		}, nil, "perf-seed", p.CreateBuilder(buf, false))
		Expect(err).To(MatchError("command 'graph-tool' from file 'testdata/import/data/v1.0.1/4800_test_cmd.run' " +
			"is not listed in configuration allowed command section"))
	})

	It("Fails on missing labels", func() {
		// Little hack to change config
		plannerCfg.Planner.Folders["data"].NodeLabels = nil

		pp, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		buf := new(migrator.ExecutionSteps)
		err = pp.Plan(localFolders, nil, nil, "seed", p.CreateBuilder(buf, false))
		Expect(err).To(MatchError("fail to import folder 'data', cannot determine DB labels"))
	})

	It("Fails on empty command file", func() {
		// Little hack to change config
		plannerCfg.Planner.Folders = nil
		plannerCfg.Planner.Batches = nil

		pp, err := migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())

		s, err := p.NewScanner("testdata/import_err_case01")
		Expect(err).To(Succeed())

		localFolders, err = s.ScanFolders()
		Expect(err).To(Succeed())
		Expect(localFolders).To(HaveLen(1))

		buf := new(migrator.ExecutionSteps)
		err = pp.Plan(localFolders, nil, nil, "schema", p.CreateBuilder(buf, false))
		Expect(err).To(MatchError(
			"no commands to run in file testdata/import_err_case01/schema/v0.0.1/100_up_empty.run, use 'exit' command to ignore file", // nolint:lll
		))
	})

	It("Create Upgrade plan with snapshot and absolute path", func() {
		buf := new(migrator.ExecutionSteps)
		err := p.Plan(localFolders, nil, &migrator.TargetVersion{Version: v100}, "schema", p.CreateBuilder(buf, true))
		Expect(err).To(Succeed())

		plan := buf.String()
		Expect(plan).To(MatchRegexp(
			"// Starting on folder snapshots - ver:1.0.0\n:source .*/migrator/testdata/import/snapshots/schema_v1.0.0.cypher;", // nolint:lll
		))
	})

	It("Create Upgrade plan with snapshot", func() {
		buf := new(migrator.ExecutionSteps)
		err := p.Plan(localFolders, nil, nil, "seed", p.CreateBuilder(buf, false))
		Expect(err).To(Succeed())

		expectedContent, err := os.ReadFile("testdata/plans/seed-batch-with-snapshot.txt")
		Expect(err).To(Succeed())

		plan := buf.String()
		Expect(plan).To(Equal(string(expectedContent)))
	})

	It("Create Upgrade plan with DB version without snapshot", func() {
		buf := new(migrator.ExecutionSteps)
		err := p.Plan(localFolders, migrator.DatabaseModel{
			"schema": []migrator.DatabaseGraphVersion{
				getDBGraphVersion(v100, 1000, 2000),
			},
		}, nil, "perf-seed", p.CreateBuilder(buf, false))
		Expect(err).To(Succeed())

		expectedContent, err := os.ReadFile("testdata/plans/perf-seed-no-snapshot.txt")
		Expect(err).To(Succeed())

		plan := buf.String()
		Expect(plan).To(Equal(string(expectedContent)))
	})

	It("Create Downgrade plan", func() {
		buf := new(migrator.ExecutionSteps)
		err := p.Plan(localFolders, migrator.DatabaseModel{
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
		}, &migrator.TargetVersion{Version: v100}, "perf-seed", p.CreateBuilder(buf, false))
		Expect(err).To(Succeed())

		expectedContent, err := os.ReadFile("testdata/plans/downgrade-perf-seed.txt")
		Expect(err).To(Succeed())

		plan := buf.String()
		Expect(plan).To(Equal(string(expectedContent)))
	})

	It("Create Upgrade+Downgrade plan", func() {
		buf := new(migrator.ExecutionSteps)
		err := p.Plan(localFolders, migrator.DatabaseModel{
			"schema": []migrator.DatabaseGraphVersion{
				getDBGraphVersion(v100, 1000, 2000),
				getDBGraphVersion(v101, 1200, 1500),
			},
		}, &migrator.TargetVersion{Version: v100}, "seed", p.CreateBuilder(buf, false))
		Expect(err).To(Succeed())

		expectedContent, err := os.ReadFile("testdata/plans/upgrade-and-downgrade.txt")
		Expect(err).To(Succeed())

		plan := buf.String()
		Expect(plan).To(Equal(string(expectedContent)))
	})
})
