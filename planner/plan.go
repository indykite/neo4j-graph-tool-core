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

package planner

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"

	"github.com/indykite/neo4j-graph-tool-core/config"
)

var parseCmd = regexp.MustCompile(`\"[^\"]+\"|\S+`)

type (
	Planner struct {
		config *config.Config
	}
	Builder func(folderName string, cf *MigrationFile, fileVer, writeVersion *GraphVersion) (bool, error)
)

// NewPlanner creates Planner instance and returns error if provided config is not valid.
func NewPlanner(config *config.Config) (*Planner, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Planner{
		config: config,
	}, nil
}

// ParseGraphVersion parse string as semver version with revision
func ParseGraphVersion(v string) (*GraphVersion, error) {
	ver, err := semver.NewVersion(v)
	if err != nil {
		return nil, err
	}
	vs := &GraphVersion{Version: ver}
	if ver.Metadata() != "" {
		vs.Revision, err = strconv.ParseUint(ver.Metadata(), 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata: %v", err)
		}
	}
	return vs, nil
}

// Plan prepares execution plan with given builder
func (p *Planner) Plan(
	versionFolders VersionFolders,
	dbModel DatabaseModel,
	target *GraphVersion,
	batch Batch,
	builder Builder,
) (*GraphVersion, error) {
	schemaFolderVersion := dbModel[p.config.Planner.SchemaFolder.FolderName]

	switch {
	case target == nil || target.Version == nil:
		// Upgrade
		fallthrough
	case schemaFolderVersion == nil || schemaFolderVersion.Version == nil ||
		schemaFolderVersion.Version.Compare(target.Version) < 0:
		// Upgrade
		return p.Upgrade(versionFolders, dbModel, target, batch, builder)
	case schemaFolderVersion.Version.Compare(target.Version) > 0:
		// Downgrade
		return p.Downgrade(versionFolders, schemaFolderVersion, target.Version, target.Revision, builder)
	default:
		for _, v := range versionFolders {
			if v.version.Equal(target.Version) {
				max := v.schemaFolder.down[0].commit
				if max < schemaFolderVersion.Revision {
					return nil, fmt.Errorf(
						"unsupported file version. current: %d > max supported: %d", schemaFolderVersion.Revision, max)
				}
				if target.Revision != 0 && max < target.Revision {
					return nil, fmt.Errorf(
						"unsupported file version. target: %d > max supported: %d", target.Revision, max)
				} else if target.Revision == 0 {
					target.Revision = max
				}
				if target.Revision < schemaFolderVersion.Revision {
					return p.Downgrade(versionFolders, schemaFolderVersion, target.Version, target.Revision, builder)
				}
				// Upgrade or Nothing to do
				return p.Upgrade(versionFolders, dbModel, target, batch, builder)
			}
		}
		return nil, fmt.Errorf("unsupported version - out of range: %v", target.Version)
	}
}

// CreateBuilder creates default Cypher builder
func (p *Planner) CreateBuilder(steps *ExecutionSteps, abs bool) Builder {
	return func(folderName string, cf *MigrationFile, fileVer, writeVersion *GraphVersion) (bool, error) {
		header := "Importing"
		switch {
		case cf.fileType == Command:
			header = "Running command for"
		case fileVer.Compare(writeVersion) > 0:
			header = "Running down of"
		}

		steps.AddCypher(fmt.Sprintf("// %s folder %s - ver:%s\n", header, folderName, fileVer.String()))

		if cf.fileType == Command {
			if err := addCommand(steps, cf); err != nil {
				return false, err
			}
		} else {
			steps.AddCypher(":source ")
			if abs {
				fp, err := filepath.Abs(cf.FilePath())
				if err != nil {
					return false, err
				}
				steps.AddCypher(fp)
			} else {
				steps.AddCypher(cf.FilePath())
			}
			steps.AddCypher(";\n")
		}

		// Version
		steps.AddCypher(":param version => '", writeVersion.Version.String(), "';\n")
		// Revision
		steps.AddCypher(":param revision => ", strconv.FormatUint(writeVersion.Revision, 10), ";\n")

		var nodeLabels []string
		if folderName == p.config.Planner.SchemaFolder.FolderName {
			nodeLabels = p.config.Planner.SchemaFolder.NodeLabels
		} else {
			folder := p.config.Planner.Folders[folderName]
			if folder != nil {
				nodeLabels = folder.NodeLabels
			}
		}

		if len(nodeLabels) == 0 {
			return false, fmt.Errorf("fail to import folder '%s', cannot determine DB labels", folderName)
		}
		steps.AddCypher(fmt.Sprintf(
			"MERGE (sm:%s {version: $version})\nSET sm.file = $revision, sm.ts = datetime();",
			strings.Join(nodeLabels, ":"),
		))

		steps.AddCypher("\n\n")
		return true, nil
	}
}

func parseArgs(line string) []string {
	args := parseCmd.FindAllString(line, -1)
	for i, a := range args {
		args[i] = strings.Trim(a, "\"")
	}
	return args
}

func addCommand(steps *ExecutionSteps, cf *MigrationFile) error {
	content, err := os.ReadFile(cf.path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	newCommands := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) == 0 || strings.HasPrefix(l, "//") || strings.HasPrefix(l, "#") {
			continue
		}

		args := parseArgs(l)
		if args[0] == "exit" {
			// Add exit command when no other commands are added
			if newCommands == 0 {
				newCommands++
				steps.AddCommand([]string{"exit"})
			}
			break
		}

		switch {
		case args[0] != "graph-tool":
			return errors.New("only graph-tool is now supported")
		case len(args) < 2:
			return errors.New("graph-tool requires command to run")
		case args[1] == "plan", args[1] == "apply":
			return errors.New("'plan' and 'apply' is not allowed to run recursively")
		}

		newCommands++
		steps.AddCommand(args)
	}
	if newCommands == 0 {
		return errors.New("no commands to run in file, use 'exit' command to ignore file")
	}
	return nil
}

const versionCypher = `MATCH (sm:%s) RETURN sm.version AS version, sm.file AS rev
ORDER BY COALESCE(sm.ts, datetime({year: 0})) DESC, sm.version DESC LIMIT 1`

// Version retrieves version of current state of DB
func (p *Planner) Version(driver neo4j.Driver) (DatabaseModel, error) {
	session := driver.NewSession(neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	var err error

	dbModel := make(DatabaseModel)
	dbModel[p.config.Planner.SchemaFolder.FolderName], err = queryVersion(session, fmt.Sprintf(
		versionCypher,
		strings.Join(p.config.Planner.SchemaFolder.NodeLabels, ":"),
	))
	if err != nil {
		return nil, err
	}

	for folderName, folderDetail := range p.config.Planner.Folders {
		dbModel[folderName], err = queryVersion(session, fmt.Sprintf(
			versionCypher,
			strings.Join(folderDetail.NodeLabels, ":"),
		))
		if err != nil {
			return nil, err
		}
	}

	return dbModel, nil
}

func queryVersion(session neo4j.Session, cypher string) (*GraphVersion, error) {
	result, err := session.ReadTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		result, err := tx.Run(cypher, nil)
		if err != nil {
			return nil, err
		}
		if result.Next() {
			if result.Err() != nil {
				return nil, result.Err()
			}
			record := result.Record()

			gs := new(GraphVersion)

			for i, name := range record.Keys {
				switch name {
				case "version":
					v, ok := record.Values[i].(string)
					if !ok {
						return nil, fmt.Errorf("invalid version filed from the response")
					}

					gs.Version, err = semver.NewVersion(v)
					if err != nil {
						return nil, err
					}
				case "rev":
					v, ok := record.Values[i].(int64)
					if !ok {
						return nil, fmt.Errorf("invalid rev filed from the response")
					}
					gs.Revision = uint64(v)
				}
			}
			return gs, nil
		}
		return nil, result.Err()
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	mr := result.(*GraphVersion)
	return mr, nil
}

// Upgrade creates plan for upgrading
func (p *Planner) Upgrade(
	versionFolders VersionFolders,
	dbModel DatabaseModel,
	target *GraphVersion,
	batch Batch,
	op Builder,
) (*GraphVersion, error) {
	var (
		err     error
		highVer *semver.Version
		hRev    uint64
	)
	if target != nil {
		highVer = target.Version
		hRev = target.Revision
	}

	type orderedVersion struct {
		version    *GraphVersion
		folderName string
	}

	versions := []orderedVersion{}

	// Init base schema
	schemaVersion := &GraphVersion{}
	if dbm := dbModel[p.config.Planner.SchemaFolder.FolderName]; dbm != nil {
		schemaVersion.Version = dbm.Version
		schemaVersion.Revision = dbm.Revision
	}
	schemaVersion.Version, highVer, err = versionFolders.verifyRange(schemaVersion.Version, highVer)
	if err != nil {
		return nil, err
	}

	batchFolders, hasBatch := p.config.Planner.Batches[string(batch)]
	if !hasBatch {
		return nil, errors.New("unknown batch name '" + string(batch) + "'")
	}

	for _, folderName := range batchFolders.Folders {
		gv := &GraphVersion{}
		versions = append(versions, orderedVersion{
			folderName: folderName,
			version:    gv,
		})
		if lgv := dbModel[folderName]; lgv != nil {
			gv.Version = lgv.Version
			gv.Revision = lgv.Revision
		}

		gv.Version, highVer, err = versionFolders.verifyRange(gv.Version, highVer)
		if err != nil {
			return nil, err
		}
	}

	changed := false

	for _, vv := range versionFolders {
		if vv.version.GreaterThan(highVer) {
			break
		}
		if vv.version.LessThan(schemaVersion.Version) {
			continue
		}
		var start, stop uint64 = 0, math.MaxUint64
		if vv.version.Equal(schemaVersion.Version) && schemaVersion.Revision != 0 {
			start = schemaVersion.Revision
		}
		if vv.version.Equal(highVer) && hRev != 0 {
			stop = hRev
		}
		for _, v := range vv.schemaFolder.up {
			if start >= v.commit {
				continue
			}
			if stop < v.commit {
				break
			}
			var b bool
			fileVer := &GraphVersion{
				Version:  vv.version,
				Revision: v.commit,
			}
			// For Upgrade the current file version and write version is the same
			b, err = op(p.config.Planner.SchemaFolder.FolderName, v, fileVer, fileVer)
			if err != nil {
				return nil, err
			}
			changed = changed || b
		}

		for _, ver := range versions {
			if !vv.version.LessThan(ver.version.Version) {
				var start uint64
				if vv.version.Equal(ver.version.Version) && ver.version.Revision != 0 {
					start = ver.version.Revision
				} else {
					start = 0
				}

				fs := vv.extraFolders[ver.folderName]
				// When cannot find folder, it might not be defined for current version. Skip that one
				if fs == nil {
					continue
				}
				for _, v := range fs.up {
					if start >= v.commit {
						continue
					}
					var b bool
					fileVer := &GraphVersion{
						Version:  vv.version,
						Revision: v.commit,
					}
					// For Upgrade the current file version and write version is the same
					b, err = op(ver.folderName, v, fileVer, fileVer)
					if err != nil {
						return nil, err
					}
					changed = changed || b
				}
			}
		}
	}
	if changed {
		return &GraphVersion{Version: highVer}, err
	}
	return nil, nil
}

// Downgrade creates plan for downgrading
func (p *Planner) Downgrade(
	versionFolders VersionFolders,
	high *GraphVersion,
	low *semver.Version, // Pass as single argument when supporting downgrade for all folders
	hRev uint64,
	op Builder,
) (*GraphVersion, error) {
	var err error
	if low == nil {
		low = versionFolders[0].version
	}
	var highVer *semver.Version
	var highRev uint64
	if high != nil && high.Version != nil {
		highVer = high.Version
		highRev = high.Revision
	}
	low, highVer, err = versionFolders.verifyRange(low, highVer)
	if err != nil {
		return nil, err
	}

	changed := false
	for i := len(versionFolders) - 1; i >= 0; i-- {
		vv := versionFolders[i]
		if vv.version.GreaterThan(highVer) {
			continue
		}
		if max := vv.schemaFolder.down[0].commit; vv.version.Equal(highVer) && highRev > max {
			return nil, fmt.Errorf(
				"out of range: can't downgrade ver %s from %d only from %d", vv.version, highRev, max)
		}
		if vv.version.Equal(low) {
			switch {
			case changed && hRev == 0:
				return &GraphVersion{
					Version:  vv.version,
					Revision: vv.schemaFolder.down[0].commit,
				}, nil
			case hRev == 0:
				// empty operations
				return nil, nil
			default:
				after := &GraphVersion{
					Version:  vv.version,
					Revision: vv.schemaFolder.down[0].commit,
				}
				for downI, v := range vv.schemaFolder.down[:len(vv.schemaFolder.down)-1] {
					if v.commit <= hRev {
						break
					}
					var b bool
					fileVer := &GraphVersion{
						Version:  vv.version,
						Revision: v.commit,
					}
					// For Downgrade the current file version is different than write version.
					// Which is actually next file version, as current version is removed from DB.
					b, err = op(
						p.config.Planner.SchemaFolder.FolderName,
						v,
						fileVer,
						getNextDownVersion(versionFolders, i, downI),
					)
					if err != nil {
						return nil, err
					}
					changed = changed || b
					if changed {
						after.Revision = vv.schemaFolder.down[downI+1].commit
					}
				}
				if changed {
					return after, err
				}
				return nil, nil
			}
		}
		var limit uint64 = math.MaxUint64
		if vv.version.Equal(highVer) && highRev != 0 {
			limit = highRev
		}
		for downI, v := range vv.schemaFolder.down {
			if v.commit > limit {
				continue
			}

			fileVer := &GraphVersion{
				Version:  vv.version,
				Revision: v.commit,
			}
			// For Downgrade the current file version is different than write version.
			// Which is actually next file version, as current version is removed from DB.
			b, err := op(
				p.config.Planner.SchemaFolder.FolderName,
				v,
				fileVer,
				getNextDownVersion(versionFolders, i, downI),
			)
			if err != nil {
				return nil, err
			}
			changed = changed || b
		}
	}
	return nil, nil
}

func getNextDownVersion(vf VersionFolders, vfIndex, fileIndex int) *GraphVersion {
	fileIndex++
	if fileIndex < len(vf[vfIndex].schemaFolder.down) {
		return &GraphVersion{
			Version:  vf[vfIndex].version,
			Revision: vf[vfIndex].schemaFolder.down[fileIndex].commit,
		}
	}

	vfIndex--
	fileIndex = 0

	return &GraphVersion{
		Version:  vf[vfIndex].version,
		Revision: vf[fileIndex].schemaFolder.down[0].commit,
	}
}
