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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type (
	Scanner      string
	Kind         int
	FileType     int
	GraphVersion struct {
		version   *semver.Version
		update    []*CypherFile
		downgrade []*CypherFile
		data      []*CypherFile
		largeData []*CypherFile
		// snapshot  *CypherFile
	}
	GraphState struct {
		Version  *semver.Version `json:"version,omitempty"`
		Revision uint64          `json:"rev,omitempty"`
	}
	GraphModel struct {
		Model *GraphState `json:"model,omitempty"`
		Data  *GraphState `json:"data,omitempty"`
		Perf  *GraphState `json:"perf,omitempty"`
	}
	GraphVersions []*GraphVersion
	CypherFile    struct {
		name     string
		path     string
		fileType FileType
		kind     Kind
		commit   uint64
		upgrade  int8
	}
	StateError struct {
		cause error
		state *GraphModel
	}
)

const (
	Model Kind = iota
	Data
	Perf
	// Snapshot
)
const (
	Cypher FileType = iota
	Command
)

var (
	coreCypher = regexp.MustCompile(`(?i)^(?P<commit>\d{1,3})_(?P<direction>up|down)_(?P<name>\w+)\.(?P<type>cypher|run)$`) // nolint:lll
	dataCypher = regexp.MustCompile(`(?i)^(?P<commit>\d{1,3})_(?P<name>\w+)\.(?P<type>cypher|run)$`)
	zero, _    = semver.NewVersion("0.0.0")
)

func (e *StateError) Error() string {
	return e.cause.Error()
}

func (e *StateError) Unwrap() error {
	return e.cause
}
func (e *StateError) As(i interface{}) bool {
	_, ok := i.(*StateError)
	return ok
}

func (e *StateError) Wrap(model, data, perf *GraphState, err error) error {
	return &StateError{
		state: &GraphModel{
			Model: model,
			Data:  data,
			Perf:  perf,
		},
		cause: err,
	}
}

// ParseGraphModel parses the SemVer versions of DB model and data version.
func ParseGraphModel(model, data, perf string) (*GraphModel, error) {
	v := new(GraphModel)
	var err error
	if model != "" {
		v.Model, err = ParseGraphVersion(model)
		if err != nil {
			return nil, err
		}
	}
	if data != "" {
		v.Data, err = ParseGraphVersion(data)
		if err != nil {
			return nil, err
		}
	}
	if perf != "" {
		v.Perf, err = ParseGraphVersion(perf)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (a *GraphState) Compare(b *GraphState) int {
	c := a.Version.Compare(b.Version)
	if c != 0 {
		return c
	}
	switch {
	case a.Revision == b.Revision:
		return 0
	case a.Revision == 0 && b.Revision != 0:
		// 0 (default) means the highest
		return 1
	case a.Revision != 0 && b.Revision == 0:
		// 0 (default) means the highest
		return -1
	case a.Revision > b.Revision:
		return 1
	case a.Revision < b.Revision:
		return -1
	default:
		return 0
	}
}

func (a *GraphState) String() string {
	if a == nil || a.Version == nil {
		return ""
	}
	if a.Revision != 0 {
		v, _ := a.Version.SetMetadata(fmt.Sprintf("%02d", a.Revision))
		return v.String()
	}
	return a.Version.String()
}
func (a *GraphState) Set(v string) error {
	var err error
	if a == nil {
		return errors.New("null value")
	}
	a.Version, err = semver.NewVersion(v)
	if err != nil {
		return err
	}
	if a.Version.Metadata() != "" {
		a.Revision, err = strconv.ParseUint(a.Version.Metadata(), 10, 0)
		if err != nil {
			return fmt.Errorf("invalid metadata: %v", err)
		}
	}
	return nil
}
func (a *GraphState) Type() string {
	return "GraphSemVer"
}

func (cf *CypherFile) FilePath() string {
	return cf.path
}

func (cf *CypherFile) String() string {
	switch {
	case cf.upgrade > 0:
		return fmt.Sprintf("upgrade# cypher-shell -f %s", cf.path)
	case cf.upgrade < 0:
		return fmt.Sprintf("downgrade# cypher-shell -f %s", cf.path)
	default:
		return fmt.Sprintf("execute# cypher-shell -f %s", cf.path)
	}
}

func NewScanner(root string) (Scanner, error) {
	fi, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory not exists: '%s'", root)
		}
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("scanner must point to a directory '%s'", root)
	}
	return Scanner(root), err
}

func (root Scanner) resolve(dir string) string {
	// Clean the path so that it cannot possibly begin with ../.
	// If it did, the result of filepath.Join would be outside the
	// tree rooted at root.  We probably won't ever see a path
	// with .. in it, but be safe anyway.
	dir = path.Clean(dir)
	return filepath.Join(string(root), dir)
}

// func (gv GraphVersions) UpgradeRev(low *GraphModel, high *GraphState, op Planner) (*GraphState, error) {
//	return gv.Upgrade(low, high.Version, func(ver *semver.Version, cf *CypherFile) (bool, error) {
//		if cf.kind == Model && high.Revision > 0 && high.Revision < cf.commit && ver.Equal(high.Version) {
//			return false, nil
//		}
//		return op(ver, cf)
//	})
// }

func (gv GraphVersions) Upgrade(low *GraphModel,
	high *semver.Version, hRev uint64, kind Kind, op Planner) (*GraphState, error) {
	var err error
	var modelVer, dataVer, perfVer *semver.Version
	var lowRev, dataRev, perfRev uint64
	if low != nil {
		if low.Model != nil && low.Model.Version != nil {
			modelVer = low.Model.Version
			lowRev = low.Model.Revision
		}
		if low.Data != nil && low.Data.Version != nil {
			dataVer = low.Data.Version
			dataRev = low.Data.Revision
		}
		if low.Perf != nil && low.Perf.Version != nil {
			perfVer = low.Perf.Version
			perfRev = low.Perf.Revision
		}
	}

	modelVer, high, err = gv.verifyRange(modelVer, high)
	if err != nil {
		return nil, err
	}
	dataVer, high, err = gv.verifyRange(dataVer, high)
	if err != nil {
		return nil, err
	}
	perfVer, high, err = gv.verifyRange(perfVer, high)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, vv := range gv {
		if vv.version.GreaterThan(high) {
			break
		}
		if vv.version.LessThan(modelVer) {
			continue
		}
		var start, stop uint64 = 0, math.MaxUint64
		if vv.version.Equal(modelVer) && lowRev != 0 {
			start = lowRev
		}
		if vv.version.Equal(high) && hRev != 0 {
			stop = hRev
		}
		for _, v := range vv.update {
			if start >= v.commit {
				continue
			}
			if stop < v.commit {
				break
			}
			var b bool
			b, err = op(vv.version, v)
			if err != nil {
				return nil, err
			}
			changed = changed || b
		}
		if kind >= Data && !vv.version.LessThan(dataVer) {
			var start uint64
			if vv.version.Equal(dataVer) && dataRev != 0 {
				start = dataRev
			} else {
				start = 0
			}
			for _, v := range vv.data {
				if start >= v.commit {
					continue
				}
				var b bool
				b, err = op(vv.version, v)
				if err != nil {
					return nil, err
				}
				changed = changed || b
			}
		}
		if kind >= Perf && !vv.version.LessThan(perfVer) {
			var start uint64
			if vv.version.Equal(perfVer) && perfRev != 0 {
				start = perfRev
			} else {
				start = 0
			}
			for _, v := range vv.largeData {
				if start >= v.commit {
					continue
				}
				var b bool
				b, err = op(vv.version, v)
				if err != nil {
					return nil, err
				}
				changed = changed || b
			}
		}
	}
	if changed {
		return &GraphState{Version: high}, err
	}
	return nil, nil
}

// func (gv GraphVersions) Downgrade(model *GraphState, target *semver.Version, op Planner) (*GraphState, error) {
//	return gv.DowngradeRev(model, target, 0, op)
// }

func (gv GraphVersions) Downgrade(high *GraphState, low *semver.Version, hRev uint64, op Planner) (*GraphState, error) {
	var err error
	if low == nil {
		low = gv[0].version
	}
	var highVer *semver.Version
	var highRev uint64
	if high != nil && high.Version != nil {
		highVer = high.Version
		highRev = high.Revision
	}
	low, highVer, err = gv.verifyRange(low, highVer)
	if err != nil {
		return nil, err
	}

	changed := false
	for i := len(gv) - 1; i >= 0; i-- {
		vv := gv[i]
		if vv.version.GreaterThan(highVer) {
			continue
		}
		if max := vv.downgrade[0].commit; vv.version.Equal(highVer) && highRev > max {
			return nil, fmt.Errorf(
				"out of range: can't downgrade ver %s from %d only from %d", vv.version, highRev, max)
		}
		if vv.version.Equal(low) {
			switch {
			case changed && hRev == 0:
				return &GraphState{
					Version:  vv.version,
					Revision: vv.downgrade[0].commit,
				}, nil
			case hRev == 0:
				// empty operations
				return nil, nil
			default:
				after := &GraphState{
					Version:  vv.version,
					Revision: vv.downgrade[0].commit,
				}
				for i, v := range vv.downgrade[:len(vv.downgrade)-1] {
					if v.commit <= hRev {
						break
					}
					var b bool
					b, err = op(vv.version, v)
					if err != nil {
						return nil, err
					}
					changed = changed || b
					if changed {
						after.Revision = vv.downgrade[i+1].commit
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
		for _, v := range vv.downgrade {
			if v.commit > limit {
				continue
			}
			b, err := op(vv.version, v)
			if err != nil {
				return nil, err
			}
			changed = changed || b
		}
	}
	return nil, nil
}

func (gv GraphVersions) verifyRange(low, high *semver.Version) (*semver.Version, *semver.Version, error) {
	if low == nil {
		low = zero
	} else if low.LessThan(gv[0].version) {
		return nil, nil, fmt.Errorf("out of range min:&%s low:%s", gv[0].version, low)
	}
	if high == nil {
		high = gv[len(gv)-1].version
	} else if high.GreaterThan(gv[len(gv)-1].version) {
		return nil, nil, fmt.Errorf("out of range max:%s high:%s", gv[len(gv)-1].version, high)
	}
	if high.LessThan(low) {
		return nil, nil, fmt.Errorf("invalid range low:%s > high:%s", low, high)
	}
	return low, high, nil
}

func (root Scanner) ScanGraphModel() (GraphVersions, error) {
	return root.Open("schema", func(ver *semver.Version, dirName string) (*GraphVersion, error) {
		v := &GraphVersion{
			version: ver,
		}
		var err error
		v.update, v.downgrade, err = root.scanGraphModelFolder(dirName)
		if err != nil || len(v.update) == 0 || len(v.downgrade) == 0 {
			return nil, err
		}
		return v, nil
	})
}

func (root Scanner) ScanData(versions GraphVersions) (GraphVersions, error) {
	_, err := root.Open("data", func(ver *semver.Version, dirName string) (*GraphVersion, error) {
		for _, v := range versions {
			if v.version.Equal(ver) {
				var err error
				v.data, err = root.scanTestDataFolder(dirName, Data)
				if err != nil {
					return nil, err
				}
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unspecified graph model for data %s", ver)
	})
	if err != nil {
		return nil, err
	}
	return versions, nil
}

func (root Scanner) ScanPerfData(versions GraphVersions) (GraphVersions, error) {
	_, err := os.Stat(root.resolve("perf"))
	switch {
	case err != nil && os.IsNotExist(err):
		return versions, nil
	case err != nil:
		return versions, err
	}
	_, err = root.Open("perf", func(ver *semver.Version, dirName string) (*GraphVersion, error) {
		for _, v := range versions {
			if v.version.Equal(ver) {
				v.largeData, err = root.scanTestDataFolder(dirName, Perf)
				if err != nil {
					return nil, err
				}
				return nil, nil
			}
		}
		return nil, fmt.Errorf("unspecified graph model for perf %s", ver)
	})
	if err != nil {
		return nil, err
	}
	return versions, nil
}
func (root Scanner) Open(dir string,
	op func(ver *semver.Version, dirName string) (*GraphVersion, error)) ([]*GraphVersion, error) {
	dPath := root.resolve(filepath.Clean(dir))
	f, err := os.Open(filepath.Clean(dPath))
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if !fi.IsDir() {
		_ = f.Close()
		return nil, fmt.Errorf("open: %s is not a directory", dPath)
	}
	dirNames, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var versions []*GraphVersion

	for _, dn := range dirNames {
		if strings.HasPrefix(dn, ".") {
			// Ignore hidden files
			continue
		}
		ver, err := semver.NewVersion(dn)
		if err != nil {
			return nil, fmt.Errorf("%v - %s", err, path.Join(dPath, dn))
		}
		// Scan files
		v, err := op(ver, path.Join(dPath, dn))
		if err != nil {
			return nil, err
		}
		if v != nil {
			versions = append(versions, v)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].version.LessThan(versions[j].version)
	})
	return versions, nil
}

func (root Scanner) scanTestDataFolder(dir string, kind Kind) ([]*CypherFile, error) {
	f, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	files := make([]*CypherFile, 0)
	for _, info := range list {
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			continue
		}

		match := dataCypher.FindStringSubmatch(info.Name())
		if len(match) != len(dataCypher.SubexpNames()) {
			return nil, fmt.Errorf("file %s does not match with the name", path.Join(dir, info.Name()))
		}

		cf := &CypherFile{
			kind: kind,
			path: path.Join(dir, info.Name()),
		}

		for i, name := range dataCypher.SubexpNames() {
			switch name {
			case "commit":
				cf.commit, err = strconv.ParseUint(match[i], 10, 0)
				if err != nil {
					return nil, err
				}
				if cf.commit == 0 {
					return nil, fmt.Errorf("forbidden number '0' at file %s", cf.path)
				}
			case "name":
				cf.name = match[i]
			case "type":
				if match[i] == "run" {
					cf.fileType = Command
				} else {
					cf.fileType = Cypher
				}
			}
		}

		for _, v := range files {
			if v.commit == cf.commit {
				return nil, fmt.Errorf("can't have two commit match")
			}
		}
		files = append(files, cf)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].commit < files[j].commit
	})
	return files, err
}

func (root Scanner) scanGraphModelFolder(dir string) ([]*CypherFile, []*CypherFile, error) {
	var ups, downs []*CypherFile
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		switch {
		case err != nil:
			return err
		case strings.HasPrefix(info.Name(), "."):
			return nil
		case info.IsDir() && path == dir:
			return nil
		case info.IsDir():
			// Skip subdirectories
			return filepath.SkipDir
		}
		match := coreCypher.FindStringSubmatch(info.Name())
		if len(match) != len(coreCypher.SubexpNames()) {
			return fmt.Errorf("file %s does not match with the name", path)
		}

		cf := &CypherFile{
			kind: Model,
			path: path,
		}

		for i, name := range coreCypher.SubexpNames() {
			switch name {
			case "commit":
				cf.commit, err = strconv.ParseUint(match[i], 10, 0)
				if err != nil {
					return err
				}
				if cf.commit == 0 {
					return fmt.Errorf("forbidden number '0' at file %s", cf.path)
				}
			case "direction":
				if match[i] == "up" {
					cf.upgrade = 1
				} else if match[i] == "down" {
					cf.upgrade = -1
				}
			case "name":
				cf.name = match[i]
			case "type":
				if match[i] == "run" {
					cf.fileType = Command
				} else {
					cf.fileType = Cypher
				}
			default:
				// ignore
			}
		}
		if cf.upgrade > 0 {
			for _, v := range ups {
				if v.commit == cf.commit {
					return fmt.Errorf("can't have two commit match")
				}
			}
			ups = append(ups, cf)
		} else if cf.upgrade < 0 {
			for _, v := range downs {
				if v.commit == cf.commit {
					return fmt.Errorf("can't have two commit match")
				}
			}
			downs = append(downs, cf)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	u, d := len(ups), len(downs)
	if u != d {
		return nil, nil, fmt.Errorf("inconsistent state: found %d up and %d down script", u, d)
	}
	if u == 0 {
		return nil, nil, nil
	}
	sort.Slice(ups, func(i, j int) bool {
		// Ascending
		return ups[i].commit < ups[j].commit
	})
	sort.Slice(downs, func(i, j int) bool {
		// Descending
		return downs[i].commit > downs[j].commit
	})
	for i, v := range ups {
		if downs[d-1-i].commit != v.commit {
			return nil, nil, fmt.Errorf("inconsistent state: missing down part of '%s'", v.path)
		}
	}

	return ups, downs, err
}
