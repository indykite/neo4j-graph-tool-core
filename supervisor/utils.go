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

package supervisor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/indykite/neo4j-graph-tool-core/planner"
)

func utilsLog(log *logrus.Entry, utilName string) *logrus.Entry {
	return log.WithField(ComponentLogKey, utilName)
}

func getNeo4jAuthForCLI() (string, string) {
	auth := strings.SplitN(os.Getenv("NEO4J_AUTH"), "/", 2)
	if len(auth) == 2 {
		return auth[0], auth[1]
	}

	// Pass really empty string to avoid asking
	return "\"\"", "\"\""
}

func getNeo4jBasicAuth() (string, string, string) {
	auth := strings.SplitN(os.Getenv("NEO4J_AUTH"), "/", 2)
	if len(auth) == 2 {
		return auth[0], auth[1], ""
	}

	return "", "", ""
}

func drop(dir string, steps *planner.ExecutionSteps) error {
	fp, err := filepath.Abs(dir + "/drop.cypher")
	if err != nil {
		return err
	}
	steps.AddCypher(
		"// wipe out the entire database\n",
		":source ", fp, "\n",
	)
	return nil
}
