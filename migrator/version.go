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

package migrator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
)

//nolint:lll
const versionCypher = `MATCH (sm:%s) WHERE sm.deleted_at IS NULL RETURN sm.version AS version, collect(sm.file) AS files`

// Version retrieves version of current state of DB.
func (p *Planner) Version(driver neo4j.Driver) (DatabaseModel, error) {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	var err error

	dbModel := make(DatabaseModel)
	var dgv []DatabaseGraphVersion
	dgv, err = queryVersion(session, fmt.Sprintf(
		versionCypher,
		strings.Join(p.config.Planner.SchemaFolder.NodeLabels, ":"),
	))
	if len(dgv) > 0 {
		dbModel[p.config.Planner.SchemaFolder.FolderName] = dgv
	}
	if err != nil {
		return nil, err
	}

	for folderName, folderDetail := range p.config.Planner.Folders {
		dgv, err = queryVersion(session, fmt.Sprintf(
			versionCypher,
			strings.Join(folderDetail.NodeLabels, ":"),
		))
		if len(dgv) > 0 {
			dbModel[folderName] = dgv
		}
		if err != nil {
			return nil, err
		}
	}

	return dbModel, nil
}

func queryVersion(session neo4j.Session, cypher string) ([]DatabaseGraphVersion, error) {
	gs := make([]DatabaseGraphVersion, 0)

	_, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		result, err := tx.Run(cypher, nil)
		if err != nil {
			return nil, err
		}
		defer func() {
			_, _ = result.Consume()
		}()

		for result.Next() {
			if err = result.Err(); err != nil {
				return nil, err
			}
			record := result.Record()
			if record == nil {
				continue
			}

			var version *semver.Version
			// var files []int64
			var files map[int64]bool

			for keyIndex, name := range record.Keys {
				switch name {
				case "version":
					v, ok := record.Values[keyIndex].(string)
					if !ok || v == "" {
						return nil, fmt.Errorf("invalid version '%s' from response", v)
					}
					version, err = semver.NewVersion(v)
					if err != nil {
						return nil, fmt.Errorf("invalid version '%s' from response", v)
					}
				case "files":
					rawFiles, ok := record.Values[keyIndex].([]interface{})
					if !ok {
						return nil, errors.New("invalid version files from the response")
					}
					// files = make([]int64, len(rawFiles))
					files = make(map[int64]bool)
					for _, v := range rawFiles {
						fileTime, ok := v.(int64)
						if !ok {
							return nil, fmt.Errorf("invalid file number '%v' from the response", v)
						}
						// files[i] = int64(fileTime)
						files[fileTime] = true
					}
				}
			}
			gs = append(gs, DatabaseGraphVersion{
				Version:        version,
				FileTimestamps: files,
			})
		}
		return nil, result.Err()
	})

	return gs, err
}
