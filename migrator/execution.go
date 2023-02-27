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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type (
	ExecutionStep struct {
		cypher  *bytes.Buffer
		command []string
	}
	ExecutionSteps []ExecutionStep
)

// IsCypher returns true if current step is Cypher. But does not check if cypher really contains something.
// So returns true for empty Cypher too.
func (s ExecutionStep) IsCypher() bool {
	return s.cypher != nil
}

// Cypher returns buffer with all cyphers in current step.
func (s ExecutionStep) Cypher() *bytes.Buffer {
	return s.cypher
}

// Command returns current command with all parameters.
func (s ExecutionStep) Command() []string {
	return s.command
}

// IsEmpty checks if there are steps to do.
func (e *ExecutionSteps) IsEmpty() bool {
	if e == nil {
		return true
	}
	return len(*e) == 0
}

// AddCypher adds all Cyphers into one buffer. If current step is Cypher as well, it is reused.
// Otherwise new buffer is created.
func (e *ExecutionSteps) AddCypher(cypher ...string) {
	if len(cypher) == 0 {
		return
	}

	l := len(*e)
	// There are some entries, and the last one is buffer
	if l > 0 && (*e)[l-1].IsCypher() {
		buf := (*e)[l-1].cypher
		for _, c := range cypher {
			_, _ = buf.WriteString(c)
		}
		return
	}

	buf := bytes.NewBufferString(cypher[0])
	for i := 1; i < len(cypher); i++ {
		_, _ = buf.WriteString(cypher[i])
	}
	*e = append(*e, ExecutionStep{
		cypher: buf,
	})
}

// AddCommand adds command with parameters to step list.
func (e *ExecutionSteps) AddCommand(args []string) {
	if len(args) == 0 {
		return
	}

	*e = append(*e, ExecutionStep{
		command: args,
	})
}

// String converts all cyphers and command calls into single long string.
// Is not really suitable for Cypher shell, but can be used for debug print.
func (e ExecutionSteps) String() string {
	s := strings.Builder{}
	for _, v := range e {
		switch {
		case v.IsCypher():
			s.Write(v.cypher.Bytes())
		case v.command[0] == "exit":
			// Exit is present here only, if there is nothing else in the file
			s.WriteString("// Nothing to do in this file\n")
		default:
			s.WriteString(">>> ")
			s.WriteString(argsToString(v.command))
			s.WriteRune('\n')
		}
	}
	return s.String()
}
func argsToString(args []string) string {
	for i, v := range args {
		if strings.Contains(v, " ") {
			args[i] = "\"" + v + "\""
		}
	}
	return strings.Join(args, " ")
}

// CreateBuilder creates default Cypher builder.
func (p *Planner) CreateBuilder(steps *ExecutionSteps, abs bool) Builder {
	return func(cf *MigrationFile, version *semver.Version) error {
		header := "Importing"
		switch {
		case cf.IsSnapshot:
			header = "Starting on"
		case cf.FileType == Command && cf.IsDowngrade:
			header = "Downgrading with command from"
		case cf.FileType == Command:
			header = "Running command from"
		case cf.IsDowngrade:
			header = "Downgrading"
		}

		steps.AddCypher(fmt.Sprintf(
			"// %s folder %s - ver:%s\n",
			header,
			cf.FolderName,
			(&TargetVersion{Version: version, Revision: cf.Timestamp}).String(),
		))

		if cf.FileType == Command {
			if err := p.addCommand(steps, cf); err != nil {
				return err
			}
		} else {
			steps.AddCypher(":source ")
			if abs {
				fp, err := filepath.Abs(cf.Path)
				if err != nil {
					return err
				}
				steps.AddCypher(fp)
			} else {
				steps.AddCypher(cf.Path)
			}
			steps.AddCypher(";\n")
		}

		// For snapshot do not store any extra version, as that should be already part of snapshot.
		if cf.IsSnapshot {
			steps.AddCypher("\n")
			return nil
		}

		var nodeLabels []string
		if cf.FolderName == p.config.Planner.SchemaFolder.FolderName {
			nodeLabels = p.config.Planner.SchemaFolder.NodeLabels
		} else {
			folder := p.config.Planner.Folders[cf.FolderName]
			if folder != nil {
				nodeLabels = folder.NodeLabels
			}
		}

		if len(nodeLabels) == 0 {
			return fmt.Errorf("fail to import folder '%s', cannot determine DB labels", cf.FolderName)
		}
		steps.AddCypher(
			`:params {"version": "`, version.String(), `", "file": `, strconv.FormatInt(cf.Timestamp, 10), "}\n")
		if cf.IsDowngrade {
			// Try to find version and then remove current file from files.
			// Or delete whole node, when there are no more files left.
			steps.AddCypher(
				`MATCH (sm:`, strings.Join(nodeLabels, ":"), ` {version: $version, file: $file}) `,
				`SET sm.deleted_at = timestamp();`,
			)
		} else {
			// Match or create node by version and set files or add current file.
			steps.AddCypher(
				`MERGE (sm:`, strings.Join(nodeLabels, ":"), ` {version: $version, file: $file}) `,
				`ON CREATE SET sm.created_at = timestamp() `,
				`SET sm.updated_at = timestamp(), sm.deleted_at = null;`,
			)
		}
		steps.AddCypher("\n\n")
		return nil
	}
}

func parseArgs(line string) []string {
	args := parseCmd.FindAllString(line, -1)
	for i, a := range args {
		args[i] = strings.Trim(a, "\"")
	}
	return args
}

func (p *Planner) addCommand(steps *ExecutionSteps, cf *MigrationFile) error {
	content, err := os.ReadFile(cf.Path)
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

		fullPath, exists := p.config.Planner.AllowedCommands[args[0]]
		if !exists {
			return fmt.Errorf("command '%s' from file '%s' is not listed in configuration allowed command section",
				args[0], cf.Path)
		}

		args[0] = fullPath

		newCommands++
		steps.AddCommand(args)
	}
	if newCommands == 0 {
		return fmt.Errorf("no commands to run in file %s, use 'exit' command to ignore file", cf.Path)
	}
	return nil
}
