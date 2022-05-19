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

package config_test

import (
	"github.com/indykite/neo4j-graph-tool-core/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("LoadFile", func() {
	It("Load correct data from file", func() {
		res, err := config.LoadFile("testdata/configData.toml")
		Expect(err).To(Succeed())
		Expect(res).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(config.DefaultPort),
				"LogLevel": Equal(config.DefaultLogLevel),
			})),
			"Planner": PointTo(MatchAllFields(Fields{
				"BaseFolder":     Equal(config.DefaultBaseFolder),
				"DropCypherFile": Equal(config.DefaultDropCypherFile),
				"Batches": MatchAllKeys(Keys{
					"data": PointTo(MatchAllFields(Fields{
						"Folders": ConsistOf("data"),
					})),
					"performance": PointTo(MatchAllFields(Fields{
						"Folders": ConsistOf("data", "perf"),
					})),
				}),
				"Folders": MatchAllKeys(Keys{
					"data": PointTo(MatchAllFields(Fields{
						"MigrationType": Equal(config.DefaultFolderMigrationType),
						"NodeLabels":    ConsistOf("DataVersion"),
					})),
					"perf": PointTo(MatchAllFields(Fields{
						"MigrationType": Equal(config.DefaultFolderMigrationType),
						"NodeLabels":    ConsistOf("PerfVersion"),
					})),
				}),
				"SchemaFolder": PointTo(MatchAllFields(Fields{
					"FolderName":    Equal(config.DefaultSchemaFolderName),
					"MigrationType": Equal(config.DefaultSchemaMigrationType),
					"NodeLabels":    ConsistOf(config.DefaultNodeLabel, "SchemaVersion"),
				})),
			})),
		})))
	})

	It("Default data", func() {
		res, err := config.New()
		Expect(err).To(Succeed())
		Expect(res).To(PointTo(MatchAllFields(Fields{
			"Supervisor": PointTo(MatchAllFields(Fields{
				"Port":     Equal(config.DefaultPort),
				"LogLevel": Equal(config.DefaultLogLevel),
			})),
			"Planner": PointTo(MatchAllFields(Fields{
				"BaseFolder":     Equal(config.DefaultBaseFolder),
				"DropCypherFile": Equal(config.DefaultDropCypherFile),
				"Batches":        HaveLen(0),
				"SchemaFolder": PointTo(MatchAllFields(Fields{
					"FolderName":    Equal(config.DefaultSchemaFolderName),
					"MigrationType": Equal(config.DefaultSchemaMigrationType),
					"NodeLabels":    ConsistOf(config.DefaultNodeLabel, "SchemaVersion"),
				})),
				"Folders": HaveLen(0),
			})),
		})))
	})

	It("Load file with incorrect data", func() {
		_, err := config.LoadFile("testdata/userIncorrectData.toml")
		Expect(err).To(MatchError("port number must be in range 1024 - 65535"))
	})

	It("Load non existing file error", func() {
		_, err := config.LoadFile("testdata/nonExisting.toml")
		Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
	})

	It("Invalid data type in file", func() {
		_, err := config.LoadFile("testdata/invalidType.toml")
		Expect(err).To(MatchError(
			ContainSubstring("expected type 'int', got unconvertible type '[]interface {}', value: '[10]'")),
		)
	})

	It("Unsupported config file", func() {
		_, err := config.LoadFile("testdata/invalid.toml.file")
		Expect(err).To(MatchError(ContainSubstring("Unsupported Config Type")))
	})

})

var _ = Describe("Validation & Normalize", func() {
	var configStruct *config.Config

	BeforeEach(func() {
		configStruct = &config.Config{
			Supervisor: &config.Supervisor{
				Port:     config.DefaultPort,
				LogLevel: config.DefaultLogLevel,
			},
			Planner: &config.Planner{
				BaseFolder:     config.DefaultBaseFolder,
				DropCypherFile: config.DefaultDropCypherFile,
				SchemaFolder: &config.SchemaFolder{
					FolderName:    config.DefaultSchemaFolderName,
					MigrationType: config.DefaultFolderMigrationType,
					NodeLabels:    []string{"TestData"},
				},
				Folders: map[string]*config.FolderDetail{
					"data": {
						MigrationType: "change",
						NodeLabels:    []string{"DataTest"},
					},
					"perf": {
						MigrationType: "change",
						NodeLabels:    []string{"PerfTest"},
					},
				},
				Batches: map[string]*config.BatchDetail{
					"data": {
						Folders: []string{"data"},
					},
					"performance": {
						Folders: []string{"data", "perf"},
					},
				},
			},
		}
	})

	It("Success validation", func() {
		err := configStruct.Validate()
		Expect(err).To(Succeed())
	})

	It("Successfully validate manual config", func() {
		cfg := &config.Config{
			Supervisor: &config.Supervisor{Port: 2555, LogLevel: "debug"},
			Planner: &config.Planner{
				BaseFolder: "abc",
				SchemaFolder: &config.SchemaFolder{
					FolderName:    "jkl",
					MigrationType: "change",
				}}}
		err := cfg.Validate()
		Expect(err).To(Succeed())
	})

	It("Validate fails when config is empty", func() {
		configStruct = nil
		err := configStruct.Validate()
		Expect(err).To(MatchError("missing config"))
	})

	DescribeTable("Validate error cases",
		func(changeCfg func(*config.Config), errorMatcher OmegaMatcher) {
			changeCfg(configStruct)
			err := configStruct.Validate()
			Expect(err).To(errorMatcher)
		},

		Entry("Missing planner", func(cfg *config.Config) {
			cfg.Planner = nil
		}, MatchError("missing config.Planner")),

		Entry("Missing supervisor", func(cfg *config.Config) {
			cfg.Supervisor = nil
		}, MatchError("missing config.Supervisor")),

		Entry("Missing schema folder", func(cfg *config.Config) {
			cfg.Planner.SchemaFolder = nil
		}, MatchError("missing config.Planner.SchemaFolder")),

		Entry("Planner without base_folder", func(cfg *config.Config) {
			cfg.Planner.BaseFolder = ""
		}, MatchError("base_folder cannot be empty")),

		Entry("Empty Folder name", func(cfg *config.Config) {
			cfg.Planner.SchemaFolder.FolderName = ""
		}, MatchError("folder_name of schema_folder cannot be empty")),

		Entry("Schema folder name specified again", func(cfg *config.Config) {
			cfg.Planner.Folders = map[string]*config.FolderDetail{
				cfg.Planner.SchemaFolder.FolderName: {},
			}
		}, MatchError("folder 'schema' is used as schema folder and cannot be used again in planner.folders")),

		Entry("Empty batch config", func(cfg *config.Config) {
			cfg.Planner.Batches = map[string]*config.BatchDetail{
				"my-batch": nil,
			}
		}, MatchError("empty configuration for batch 'my-batch'")),

		Entry("Schema folder name in batch folders array", func(cfg *config.Config) {
			cfg.Planner.Batches = map[string]*config.BatchDetail{
				"my-batch": {Folders: []string{cfg.Planner.SchemaFolder.FolderName}},
			}
		}, MatchError("folder 'schema' is schema folder and thus implicit, cannot be specified in batch 'my-batch'")),

		Entry("Folder in batch but not in folders map", func(cfg *config.Config) {
			cfg.Planner.Batches = map[string]*config.BatchDetail{
				"my-batch": {Folders: []string{"my-super-duper-folder"}},
			}
		}, MatchError("folder 'my-super-duper-folder' in batch 'my-batch' is not defined in planner.folders")),

		Entry("Empty folder config", func(cfg *config.Config) {
			cfg.Planner.Folders = map[string]*config.FolderDetail{
				"my-folder": nil,
			}
		}, MatchError("empty configuration for folder 'my-folder'")),

		Entry("Port Number", func(cfg *config.Config) {
			cfg.Supervisor.Port = 1000
		}, MatchError("port number must be in range 1024 - 65535")),

		Entry("Log Level", func(cfg *config.Config) {
			cfg.Supervisor.LogLevel = "xxx"
		}, MatchError("logLevel value 'xxx' is invalid, must be one of 'fatal,error,warn,info,debug,trace'")),

		Entry("Folder MigrationType", func(cfg *config.Config) {
			cfg.Planner.Folders["data"].MigrationType = "invalidName"
		}, MatchError("in folder 'data' migration_type must be 'change' or 'up_down'")),

		Entry("SchemaFolder duplicate elements", func(cfg *config.Config) {
			cfg.Planner.SchemaFolder.NodeLabels = []string{"SchemaVersion", "SchemaVersion"}
		}, MatchError("duplicate label 'SchemaVersion' in schemaFolder")),

		Entry("Folder duplicate elements", func(cfg *config.Config) {
			cfg.Planner.Folders["data"].NodeLabels = []string{"DataVersion", "DataVersion"}
		}, MatchError("duplicate label 'DataVersion' in folder named 'data'")),

		Entry("Schema in planner.batches", func(cfg *config.Config) {
			cfg.Planner.Batches["schema"] = &config.BatchDetail{}
		}, MatchError("the batch 'schema' is set automatically and cannot be overwritten")),

		Entry("Schema MigrationType", func(cfg *config.Config) {
			cfg.Planner.SchemaFolder.MigrationType = "invalid"
		}, MatchError("in folder schema migration_type must be 'change' or 'up_down'")),
	)

	Describe("Normalize", func() {
		It("Normalize fails when config is not fully specified", func() {
			configStruct = nil
			err := configStruct.Normalize()
			Expect(err).To(MatchError("missing config"))
		})

		It("Successfully normalize manual config", func() {
			cfg := &config.Config{
				Supervisor: &config.Supervisor{Port: 2555, LogLevel: "debug"},
				Planner: &config.Planner{
					BaseFolder: "abc",
					Folders: map[string]*config.FolderDetail{
						"superdata": {},
					},
					SchemaFolder: &config.SchemaFolder{
						FolderName:    "jkl",
						MigrationType: "change",
					}}}
			err := cfg.Normalize()
			Expect(err).To(Succeed())
			Expect(cfg.Planner.SchemaFolder.NodeLabels).To(ConsistOf(config.DefaultNodeLabel, "JklVersion"))
			f := cfg.Planner.Folders["superdata"]
			Expect(f.MigrationType).To(Equal("change"))
			Expect(f.NodeLabels).To(ConsistOf(config.DefaultNodeLabel, "SuperdataVersion"))
		})

		It("Missing NodeLabels", func() {
			configStruct.Planner.Folders["data"].NodeLabels = []string{}
			err := configStruct.Normalize()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
					"Folders": MatchKeys(IgnoreExtras, Keys{
						"data": PointTo(MatchFields(IgnoreExtras, Fields{
							"NodeLabels": ConsistOf(config.DefaultNodeLabel, "DataVersion"),
						})),
					}),
				})),
			})))
		})

		It("Case insensitive", func() {
			configStruct.Supervisor.LogLevel = "fAtAL"
			err := configStruct.Normalize()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Supervisor": PointTo(MatchFields(IgnoreExtras, Fields{
					"LogLevel": Equal("fatal"),
				})),
			})))
		})

		It("Missing data in Planner.Folders", func() {
			configStruct.Planner.Folders["data"].MigrationType = ""
			err := configStruct.Normalize()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
					"Folders": MatchKeys(IgnoreExtras, Keys{
						"data": PointTo(MatchFields(IgnoreExtras, Fields{
							"MigrationType": Equal(config.DefaultFolderMigrationType),
						})),
					}),
				})),
			})))
		})
	})
})
