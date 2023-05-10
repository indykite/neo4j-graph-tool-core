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
	"encoding/json"
	"errors"

	gomock "github.com/golang/mock/gomock"
	neo4j "github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j/db"

	"github.com/indykite/neo4j-graph-tool-core/config"
	"github.com/indykite/neo4j-graph-tool-core/migrator"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	var (
		mockCtrl    *gomock.Controller
		transaction *MockTransaction
		mockResult  *MockResult
		driver      *MockDriver

		p *migrator.Planner
	)

	type mockedRecord struct {
		version string
		files   []int64
		// file shouldn't be float ever. But if someone updates it manually, it might get into this point.
		floatFiles []float64
	}

	mockVersionCall := func(labels string, records ...*mockedRecord) {
		transaction.EXPECT().Run(
			"MATCH (sm"+labels+") WHERE sm.deleted_at IS NULL RETURN sm.version AS version, collect(sm.file) AS files", //nolint:lll
			nil,
		).DoAndReturn(func(_, _ interface{}) (neo4j.Result, error) {
			for _, r := range records {
				mockResult.EXPECT().Next().Return(true)
				mockResult.EXPECT().Err().Return(nil)

				var record *db.Record
				if r != nil {
					var files []interface{}
					for _, f := range r.files {
						files = append(files, f)
					}
					for _, f := range r.floatFiles {
						files = append(files, f)
					}
					record = &db.Record{
						Keys:   []string{"version", "files"},
						Values: []interface{}{r.version, files},
					}
				}
				mockResult.EXPECT().Record().Return(record)
			}
			mockResult.EXPECT().Next().Return(false)
			mockResult.EXPECT().Err().Return(nil)
			mockResult.EXPECT().Consume().Return(nil, nil)

			return mockResult, nil
		})
	}

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		transaction = NewMockTransaction(mockCtrl)
		mockResult = NewMockResult(mockCtrl)

		driver = NewMockDriver(mockCtrl)
		driver.EXPECT().
			NewSession(gomock.Eq(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})).
			Return(Neo4jSession(transaction))

		plannerCfg := &config.Config{Planner: &config.Planner{
			BaseFolder: "import",
			SchemaFolder: &config.SchemaFolder{
				FolderName:    "schema",
				MigrationType: config.DefaultSchemaMigrationType,
				NodeLabels:    []string{"MySchema", "ExtraSchemaLabel"},
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

		p, err = migrator.NewPlanner(plannerCfg)
		Expect(err).To(Succeed())
	})

	It("Fail to run cypher", func() {
		transaction.EXPECT().Run(gomock.Any(), nil).Return(nil, errors.New("cannot run cypher"))
		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("cannot run cypher"))
		Expect(dbm).To(BeNil())
	})

	It("Fail to fetch result", func() {
		mockVersionCall(":MySchema:ExtraSchemaLabel",
			&mockedRecord{version: "1.0.0", files: []int64{1100, 1500, 2400}},
			&mockedRecord{version: "1.1.0", files: []int64{1800}},
			&mockedRecord{version: "2.0.0", files: []int64{2300, 2800}},
		)

		transaction.EXPECT().
			Run(gomock.Any(), nil).
			DoAndReturn(func(_, _ interface{}) (neo4j.Result, error) {
				mockResult.EXPECT().Next().Return(true)
				mockResult.EXPECT().Err().Return(errors.New("cannot fetch result"))
				mockResult.EXPECT().Consume().Return(nil, nil)
				return mockResult, nil
			})
		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("cannot fetch result"))
		Expect(dbm).To(BeNil())
	})

	It("Empty version", func() {
		mockVersionCall(":MySchema:ExtraSchemaLabel",
			&mockedRecord{},
		)

		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("invalid version '' from response"))
		Expect(dbm).To(BeNil())
	})

	It("Invalid version", func() {
		mockVersionCall(":MySchema:ExtraSchemaLabel",
			&mockedRecord{version: "non-version"},
		)

		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("invalid version 'non-version' from response"))
		Expect(dbm).To(BeNil())
	})

	It("Invalid files", func() {
		transaction.EXPECT().
			Run(gomock.Any(), nil).
			DoAndReturn(func(_, _ interface{}) (neo4j.Result, error) {
				mockResult.EXPECT().Next().Return(true)
				mockResult.EXPECT().Err().Return(nil)
				mockResult.EXPECT().Record().Return(&db.Record{
					Keys:   []string{"files"},
					Values: []interface{}{159},
				})
				mockResult.EXPECT().Consume().Return(nil, nil)

				return mockResult, nil
			})

		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("invalid version files from the response"))
		Expect(dbm).To(BeNil())
	})

	It("Invalid file number", func() {
		transaction.EXPECT().
			Run(gomock.Any(), nil).
			DoAndReturn(func(_, _ interface{}) (neo4j.Result, error) {
				mockResult.EXPECT().Next().Return(true)
				mockResult.EXPECT().Err().Return(nil)
				mockResult.EXPECT().Record().Return(&db.Record{
					Keys:   []string{"files"},
					Values: []interface{}{[]interface{}{"hello"}},
				})
				mockResult.EXPECT().Consume().Return(nil, nil)

				return mockResult, nil
			})

		dbm, err := p.Version(driver)
		Expect(err).To(MatchError("file number 'hello' is of type string, expect int64"))
		Expect(dbm).To(BeNil())
	})

	It("Fetch all versions", func() {
		mockVersionCall(":MySchema:ExtraSchemaLabel",
			&mockedRecord{version: "1.0.0", files: []int64{1100, 1500, 2400}},
			&mockedRecord{version: "1.1.0", files: []int64{1800}},
			&mockedRecord{version: "2.0.0", floatFiles: []float64{2300, 2800}},
			nil,
		)
		mockVersionCall(":DataVersion",
			&mockedRecord{version: "1.0.0", files: []int64{1250, 1800}},
		)
		mockVersionCall(":GraphToolMigration:PerfVersion",
			&mockedRecord{version: "1.0.0", files: []int64{1300}},
			&mockedRecord{version: "1.1.0", files: []int64{1950}},
		)

		dbm, err := p.Version(driver)
		Expect(err).To(Succeed())

		result, err := json.Marshal(dbm)
		Expect(err).To(Succeed())

		Expect(result).To(MatchJSON(`{
			"schema": {
			  "1.0.0": [1100, 1500, 2400],
			  "1.1.0": [1800],
			  "2.0.0": [2300, 2800]
			},
			"data": {
			  "1.0.0": [1250, 1800]
			},
			"perf": {
			  "1.0.0": [1300],
			  "1.1.0": [1950]
			}
		  }
		  `))
	})
})
