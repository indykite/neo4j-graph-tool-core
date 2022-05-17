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

var _ = Describe("Config", func() {
	var _ = Describe("LoadTOML", func() {
		It("Invalid path", func() {
			_, err := config.LoadConfig("testdata/none")
			Expect(err).To(MatchError(ContainSubstring("open testdata/none: no such file or directory")))
		})
		It("Invalid TOML", func() {
			_, err := config.LoadConfig("testdata/invalid.toml.file")

			Expect(err).To(MatchError(ContainSubstring(
				"toml: line 2 (last key \"supervisor\"): expected '.' or '=', but got ':' instead")))
		})
		It("Invalid data", func() {
			_, err := config.LoadConfig("testdata/invalidData.toml")
			Expect(err).To(MatchError(MatchRegexp("planner.batches array is missing")))

		})
		It("Loading data from file", func() {
			res, err := config.LoadConfig("testdata/configData.toml")
			Expect(err).To(Succeed())
			Expect(res).To(PointTo(MatchAllFields(Fields{
				"Supervisor": PointTo(MatchAllFields(Fields{
					"Port":     Equal(config.DefaultPort),
					"LogLevel": Equal(config.DefaultLogLevel),
				})),
				"Planner": PointTo(MatchAllFields(Fields{
					"BaseFolder": Equal(config.DefaultBaseFolder),
					"Batches": ConsistOf(PointTo(MatchAllFields(Fields{
						"Name":    Equal("schema"),
						"Folders": ConsistOf("schema"),
					})), PointTo(MatchAllFields(Fields{
						"Name":    Equal("data"),
						"Folders": ConsistOf("schema", "data"),
					})), PointTo(MatchAllFields(Fields{
						"Name":    Equal("performance"),
						"Folders": ConsistOf("schema", "data", "perf"),
					})),
					),
				})),
			})))
		})

	})
	var _ = Describe("Validation", func() {
		var configStruct *config.Config

		BeforeEach(func() {
			configStruct = &config.Config{
				Supervisor: &config.Supervisor{
					Port:     config.DefaultPort,
					LogLevel: config.DefaultLogLevel,
				},
				Planner: &config.Planner{
					BaseFolder: config.DefaultBaseFolder,
					Batches: []*config.Batch{
						{
							Name:    "schema",
							Folders: []string{"schema"},
						},
						{
							Name:    "data",
							Folders: []string{"schema", "data"},
						},
						{
							Name:    "performance",
							Folders: []string{"schema", "data", "perf"},
						},
					},
				},
			}
		})

		It("Success validation", func() {
			err := configStruct.Validate()
			Expect(err).To(Succeed())
		})

		It("Fill missing data", func() {
			configStruct.Supervisor.Port = 0
			configStruct.Supervisor.LogLevel = ""
			configStruct.Planner.BaseFolder = ""
			err := configStruct.Validate()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchAllFields(Fields{
				"Supervisor": PointTo(MatchAllFields(Fields{
					"Port":     Equal(config.DefaultPort),
					"LogLevel": Equal(config.DefaultLogLevel),
				})),
				"Planner": PointTo(MatchFields(IgnoreExtras, Fields{
					"BaseFolder": Equal(config.DefaultBaseFolder),
				})),
			})))

		})
		It("Case insensitive", func() {
			configStruct.Supervisor.LogLevel = "fAtAL"
			err := configStruct.Validate()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Supervisor": PointTo(MatchFields(IgnoreExtras, Fields{
					"LogLevel": Equal("fatal"),
				})),
			})))
		})

		It("ConfigStruct.Supervisor missing", func() {
			configStruct.Supervisor = nil
			err := configStruct.Validate()
			Expect(err).To(Succeed())
			Expect(configStruct).To(PointTo(MatchFields(IgnoreExtras, Fields{
				"Supervisor": PointTo(MatchAllFields(Fields{
					"Port":     Equal(config.DefaultPort),
					"LogLevel": Equal(config.DefaultLogLevel),
				})),
			})))

		})
		var _ = Describe("Error tests", func() {
			It("Port Number", func() {
				configStruct.Supervisor.Port = 1000
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp("port number must be in range 1024 - 65535")))

			})
			It("Log Level", func() {
				configStruct.Supervisor.LogLevel = "debugfatal"
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp(
					"logLevel value 'debugfatal' invalid, must be one of 'fatal,error,warn,info,debug,trace'")))

			})
			It("Planner Batches missing", func() {
				configStruct.Planner.Batches = []*config.Batch{}
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp("planner.batches array is missing")))
			})
			It("Duplicate Names", func() {
				configStruct.Planner.Batches[0].Name = configStruct.Planner.Batches[1].Name
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp("duplicate name 'data' in planner.batches")))
			})
			It("Duplicate elements in Batch.Folders", func() {
				configStruct.Planner.Batches[1].Folders = []string{"schema", "schema", "data"}
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp(
					"duplicate element 'schema' in folders array in planner.batch named 'data'")))
			})
			It("Config.Planner missing", func() {
				configStruct.Planner = nil
				err := configStruct.Validate()
				Expect(err).To(MatchError(MatchRegexp("planner field is missing")))
			})
		})
	})
})
