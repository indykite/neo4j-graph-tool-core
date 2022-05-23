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
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type (
	Planner func(ver *semver.Version, cf *CypherFile) (bool, error)
)

var parseCmd = regexp.MustCompile(`\"[^\"]+\"|\S+`)

func ParseGraphVersion(v string) (*GraphState, error) {
	ver, err := semver.NewVersion(v)
	if err != nil {
		return nil, err
	}
	vs := &GraphState{Version: ver}
	if ver.Metadata() != "" {
		vs.Revision, err = strconv.ParseUint(ver.Metadata(), 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata: %v", err)
		}
	}
	return vs, nil
}

func (gv GraphVersions) Plan(model *GraphModel, target *GraphState, kind Kind, builder Planner) (*GraphState, error) {
	switch {
	case target == nil || target.Version == nil:
		// Upgrade
		return gv.Upgrade(model, nil, 0, kind, builder)
	case model == nil || model.Model == nil || model.Model.Version == nil ||
		model.Model.Version.Compare(target.Version) < 0:
		// Upgrade
		return gv.Upgrade(model, target.Version, target.Revision, kind, builder)
	case model.Model.Version.Compare(target.Version) > 0:
		// Downgrade
		return gv.Downgrade(model.Model, target.Version, target.Revision, builder)
	default:
		for _, v := range gv {
			if v.version.Equal(target.Version) {
				max := v.downgrade[0].commit
				if max < model.Model.Revision {
					return nil, fmt.Errorf(
						"unsupported file version. current: %d > max supported: %d", model.Model.Revision, max)
				}
				if target.Revision != 0 && max < target.Revision {
					return nil, fmt.Errorf(
						"unsupported file version. target: %d > max supported: %d", target.Revision, max)
				} else if target.Revision == 0 {
					target.Revision = max
				}
				if target.Revision < model.Model.Revision {
					return gv.Downgrade(model.Model, target.Version, target.Revision, builder)
				}
				// Upgrade or Nothing to do
				return gv.Upgrade(model, target.Version, target.Revision, kind, builder)
			}
		}
		return nil, fmt.Errorf("unsupported version - out of range: %v", target.Version)
	}
}

func SetVersion(steps *ExecutionSteps, v *GraphState) {
	steps.AddCypher(fmt.Sprintf("// Set Model Version - ver:%s rev:%d\n", v.Version, v.Revision))
	// Version
	steps.AddCypher(
		":param version => '",
		v.Version.String(),
		"';\n")
	// Revision
	steps.AddCypher(
		":param revision => ",
		strconv.FormatUint(v.Revision, 10),
		";\n",
		`MERGE (sm:ModelVersion {version: $version}) SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();`,
		"\n\n")
}

func CreatePlan(steps *ExecutionSteps, abs bool) Planner {
	return func(ver *semver.Version, cf *CypherFile) (bool, error) {
		header := "Importing"
		if cf.fileType == Command {
			header = "Running command for"
		}

		switch cf.kind {
		case Data:
			steps.AddCypher(fmt.Sprintf("// %s data - ver:%s rev:%d\n", header, ver, cf.commit))
		case Perf:
			steps.AddCypher(fmt.Sprintf("// %s perf - ver:%s rev:%d\n", header, ver, cf.commit))
		case Model:
			steps.AddCypher(fmt.Sprintf("// %s model - ver:%s rev:%d\n", header, ver, cf.commit))
		}

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
		steps.AddCypher(
			":param version => '",
			ver.String(),
			"';\n")
		// Revision
		steps.AddCypher(
			":param revision => ",
			strconv.FormatUint(cf.commit, 10),
			";\n")
		switch cf.kind {
		case Data:
			steps.AddCypher(`MERGE (sm:DataVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();
`)
		case Model:
			steps.AddCypher(`MERGE (sm:ModelVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();
`)
		case Perf:
			steps.AddCypher(`MERGE (sm:PerfVersion {version: $version})
SET sm.file = $revision, sm.dirty = false, sm.ts = datetime();
`)
		}
		steps.AddCypher("\n")
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

func addCommand(steps *ExecutionSteps, cf *CypherFile) error {
	content, err := ioutil.ReadFile(cf.path)
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
