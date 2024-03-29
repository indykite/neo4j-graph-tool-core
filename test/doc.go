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

// Package test specifies some Neo4j interfaces without unexported functions, so it is possible to mock it.
// Only usage of that is to generate mocks and use it in tests.
package test

//go:generate mockgen -copyright_file ../doc/LICENSE -package test -destination ./neo4j_mock.go github.com/neo4j/neo4j-go-driver/v5/neo4j ExplicitTransaction,ResultWithContext
