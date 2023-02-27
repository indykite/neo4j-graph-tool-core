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

package supervisor

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
	"github.com/sirupsen/logrus"

	"github.com/indykite/neo4j-graph-tool-core/migrator"
)

// Driver returns new Neo4j driver for custom Cypher calls with set up authorization based on config in Wrapper.
func (w *Neo4jWrapper) Driver() (neo4j.Driver, error) {
	return neo4j.NewDriver(boltAddr, neo4j.BasicAuth(w.getNeo4jBasicAuth()))
}

func (w *Neo4jWrapper) getImportDir() string {
	var path string
	if strings.HasPrefix(w.cfg.Planner.BaseFolder, "/") {
		path = w.cfg.Planner.BaseFolder
	} else {
		path = initialDataDir + w.cfg.Planner.BaseFolder
	}

	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

// getNeo4jBasicAuth returns username, password and realm just like neo4j.BasicAuth() requires.
// nolint:unparam
func (w *Neo4jWrapper) getNeo4jBasicAuth() (username string, password string, realm string) {
	auth := strings.SplitN(w.cfg.Supervisor.Neo4jAuth, "/", 2)
	if len(auth) == 2 {
		return auth[0], auth[1], ""
	}

	return "", "", ""
}

func (w *Neo4jWrapper) drop(steps *migrator.ExecutionSteps) error {
	if w.cfg.Planner.DropCypherFile == "" {
		return errors.New("drop cypher file is not specified")
	}
	fp, err := filepath.Abs(w.getImportDir() + w.cfg.Planner.DropCypherFile)
	if err != nil {
		return err
	}
	w.log.WithField("file", fp).Trace("Adding drop file to execution steps")
	steps.AddCypher(
		"// wipe out the entire database\n",
		":source ", fp, "\n",
	)
	return nil
}

func (w *Neo4jWrapper) utilsLog(utilName string) *logrus.Entry {
	return w.log.WithField(componentLogKey, utilName)
}
