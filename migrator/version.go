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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Neo4jSession struct {
	neo4j.SessionWithContext
}

var _ neo4j.SessionWithContext = &Neo4jSession{}

//nolint:lll
const versionCypher = `MATCH (sm:%s) WHERE sm.deleted_at IS NULL RETURN sm.version AS version, collect(sm.file) AS files`

// Version retrieves version of current state of DB.
func (p *Planner) Version(ctx context.Context, session neo4j.SessionWithContext) (DatabaseModel, error) {
	var err error

	dbModel := make(DatabaseModel)
	var dgv []DatabaseGraphVersion
	dgv, err = queryVersion(ctx, session, fmt.Sprintf(
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
		dgv, err = queryVersion(ctx, session, fmt.Sprintf(
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

func queryVersion(
	ctx context.Context,
	session neo4j.SessionWithContext,
	cypher string,
) ([]DatabaseGraphVersion, error) {
	gs := make([]DatabaseGraphVersion, 0)

	_, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, cypher, nil)
		if err != nil {
			return nil, err
		}
		defer func() {
			_, _ = result.Consume(ctx)
		}()

		for result.Next(ctx) {
			if err = result.Err(); err != nil {
				return nil, err
			}
			record := result.Record()
			if record == nil {
				continue
			}

			var version *semver.Version
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
					rawFiles, ok := record.Values[keyIndex].([]any)
					if !ok {
						return nil, errors.New("invalid version files from the response")
					}
					files = make(map[int64]bool)
					for _, v := range rawFiles {
						switch fileTime := v.(type) {
						case int64:
							files[fileTime] = true
						case float64:
							files[int64(fileTime)] = true
						default:
							return nil, fmt.Errorf("file number '%v' is of type %T, expect int64", v, v)
						}
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
