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
	"bytes"
	"strings"
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
