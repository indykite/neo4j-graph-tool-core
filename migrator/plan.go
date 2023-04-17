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
	"math"
	"regexp"

	"github.com/Masterminds/semver/v3"

	"github.com/indykite/neo4j-graph-tool-core/config"
)

var parseCmd = regexp.MustCompile(`\"[^\"]+\"|\S+`)

type (
	Planner struct {
		config *config.Config
	}

	Builder func(cf *MigrationFile, version *semver.Version) error
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

// Plan prepares execution plan with given builder.
func (p *Planner) Plan(
	localFolders LocalFolders,
	dbModel DatabaseModel,
	targetVersion *TargetVersion,
	batch Batch,
	builder Builder,
) error {
	var batchFolders []string
	if batch != "schema" {
		// schema is implicit batch
		b, hasBatch := p.config.Planner.Batches[string(batch)]
		if !hasBatch || b == nil {
			return errors.New("unknown batch name '" + string(batch) + "'")
		}
		batchFolders = b.Folders
	}

	type versionPlan struct {
		*MigrationScripts
		version *semver.Version
	}
	plan := []versionPlan{}

	// preventSnapshot can disable using snapshots even they exists.
	preventSnapshot := false // TODO: read from configuration

	localFolders.SortByVersion() // Sort by version first, so we iterate from oldest to newest

	// This causes troubles later in the code. Just set to nil for easier checks deeper in the code.
	// If semver version is not set, target version is invalid anyway.
	if targetVersion != nil && targetVersion.Version == nil {
		targetVersion = nil
	}

	if len(localFolders) > 0 && targetVersion != nil &&
		localFolders[len(localFolders)-1].Version.LessThan(targetVersion.Version) {
		return fmt.Errorf("specified target %s version does not exist", targetVersion.Version.String())
	}

	// Iterate over local migration, version by version
	for _, lf := range localFolders {
		if !preventSnapshot && !dbModel.HasAnyVersion() && lf.Snapshots[batch] != nil {
			switch {
			case targetVersion == nil:
				// If there is no target version, planning till the end.
				fallthrough
			case lf.Version.LessThan(targetVersion.Version):
				// If target is higher than current folder, we can use that.
				fallthrough
			case lf.Version.Equal(targetVersion.Version) && targetVersion.Revision == 0:
				// If target version is equal to snapshot version and no revision is specified,
				// we can assume that snapshot contains all revisions.
				// Always override all files before snapshot as that is starting point always.
				plan = []versionPlan{{
					MigrationScripts: &MigrationScripts{
						Up: []*MigrationFile{lf.Snapshots[batch]},
					},
					version: lf.Version,
				}}
				continue
			}
		}

		filesToRun := p.planFolder(
			p.config.Planner.SchemaFolder.FolderName,
			lf.Version,
			lf.SchemaFolder,
			dbModel,
			targetVersion,
		)

		for _, bf := range batchFolders {
			filesToRun.Add(p.planFolder(bf, lf.Version, lf.ExtraFolders[bf], dbModel, targetVersion))
		}

		if filesToRun.ContainsMigrations() {
			plan = append(plan, versionPlan{
				MigrationScripts: filesToRun,
				version:          lf.Version,
			})
		}
	}

	planLength := len(plan)
	// Do upgrade
	for i := 0; i < planLength; i++ {
		plan[i].SortUpFiles()
		currentVersion := plan[i].version
		for _, mf := range plan[i].Up {
			if err := builder(mf, currentVersion); err != nil {
				return err
			}
		}
	}
	// Do downgrade
	for i := planLength - 1; i >= 0; i-- {
		plan[i].SortDownFiles()
		currentVersion := plan[i].version
		for _, mf := range plan[i].Down {
			if err := builder(mf, currentVersion); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Planner) planFolder(
	folderName string,
	folderVersion *semver.Version,
	folderScripts *MigrationScripts,
	dbModel DatabaseModel,
	targetVersion *TargetVersion,
) *MigrationScripts {
	if folderScripts == nil {
		return nil
	}
	// runOutdated can force to run missing migrations in older version, that shouldn't be affected anymore.
	runOutdated := false // TODO: read from configuration
	// preventRollback can be used to ignore rollback even it would be executed.
	preventRollback := false // TODO: read from configuration

	filesToRun := &MigrationScripts{}
	executedFiles := dbModel.GetFileTimestamps(folderName, folderVersion)

	switch {
	case targetVersion == nil:
		fallthrough

	case folderVersion.LessThan(targetVersion.Version):
		if dbModel.ContainsHigherVersion(folderName, folderVersion) && !runOutdated {
			break
		}
		filesToRun.Up = p.planUpgrade(folderScripts, executedFiles, 0)

	case folderVersion.Equal(targetVersion.Version):
		filesToRun.Up = p.planUpgrade(folderScripts, executedFiles, targetVersion.Revision)
		filesToRun.Down = p.planDowngrade(folderScripts, executedFiles, targetVersion.Revision)

	case folderVersion.GreaterThan(targetVersion.Version) && !preventRollback:
		filesToRun.Down = p.planDowngrade(folderScripts, executedFiles, -1)
	}

	return filesToRun
}

func (p *Planner) planUpgrade(
	folderScripts *MigrationScripts,
	executedFiles map[int64]bool,
	targetCommit int64,
) []*MigrationFile {
	filesToRun := []*MigrationFile{}

	if targetCommit == 0 {
		targetCommit = math.MaxInt64
	}

	for _, upFile := range folderScripts.Up {
		fileWasExecuted := executedFiles[upFile.Timestamp]
		if upFile.Timestamp <= targetCommit && !fileWasExecuted {
			filesToRun = append(filesToRun, upFile)
		}
	}

	return filesToRun
}

func (p *Planner) planDowngrade(
	folderScripts *MigrationScripts,
	executedFiles map[int64]bool,
	targetCommit int64,
) []*MigrationFile {
	filesToRun := []*MigrationFile{}

	if targetCommit == 0 {
		targetCommit = math.MaxInt64
	}

	for _, downFile := range folderScripts.Down {
		fileWasExecuted := executedFiles[downFile.Timestamp]
		if downFile.Timestamp > targetCommit && fileWasExecuted {
			filesToRun = append(filesToRun, downFile)
		}
	}

	return filesToRun
}
