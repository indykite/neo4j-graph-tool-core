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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/indykite/neo4j-graph-tool-core/config"
)

type (
	Scanner struct {
		config  *config.Config
		baseDir string
	}
	Batch    string
	FileType int

	VersionFolder struct {
		version      *semver.Version
		schemaFolder *folderScripts
		extraFolders map[string]*folderScripts
	}

	folderScripts struct {
		// up is used as 'change' type, as this is only applied during up action
		up   []*MigrationFile
		down []*MigrationFile
	}

	DatabaseModel map[string]*GraphVersion
	GraphVersion  struct {
		Version  *semver.Version `json:"version,omitempty"`
		Revision uint64          `json:"rev,omitempty"`
	}

	VersionFolders []*VersionFolder

	MigrationFile struct {
		name     string
		path     string
		fileType FileType
		commit   uint64
		upgrade  int8
	}
)

const (
	Cypher FileType = iota
	Command
)

var (
	coreCypher = regexp.MustCompile(`(?i)^(?P<commit>\d{1,3})_(?P<direction>up|down)_(?P<name>\w+)\.(?P<type>cypher|run)$`) // nolint:lll
	dataCypher = regexp.MustCompile(`(?i)^(?P<commit>\d{1,3})_(?P<name>\w+)\.(?P<type>cypher|run)$`)
	zero, _    = semver.NewVersion("0.0.0")

	_ fmt.Stringer = DatabaseModel{} // Be sure DatabaseModel implements String method
)

// Compare compares this version to another one. It returns -1, 0, or 1 if
// the version smaller, equal, or larger than the other version.
func (a *GraphVersion) Compare(b *GraphVersion) int {
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

func (dbm DatabaseModel) String() string {
	versions := make([]string, 0, len(dbm))
	for m, v := range dbm {
		versions = append(versions, fmt.Sprintf(`"%s": %s`, m, v))
	}
	return "{" + strings.Join(versions, ", ") + "}"
}

func (a *GraphVersion) String() string {
	if a == nil || a.Version == nil {
		return ""
	}
	if a.Revision != 0 {
		v, _ := a.Version.SetMetadata(fmt.Sprintf("%02d", a.Revision))
		return v.String()
	}
	return a.Version.String()
}

// Set sets version from string. Can be used with flag package
func (a *GraphVersion) Set(v string) error {
	if a == nil {
		return errors.New("null value")
	}
	gv, err := ParseGraphVersion(v)
	if err != nil {
		return err
	}

	a.Version = gv.Version
	a.Revision = gv.Revision
	return nil
}

// Type returns type name, so it can be used with flag package
func (a *GraphVersion) Type() string {
	return "GraphVersion"
}

// FilePath returns path of migration file
func (cf *MigrationFile) FilePath() string {
	return cf.path
}

// NewScanner creates file scanner
func (p *Planner) NewScanner(root string) (*Scanner, error) {
	fi, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not exists: '%s'", root)
		}
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("scanner must point to a directory '%s'", root)
	}
	return &Scanner{
		config:  p.config,
		baseDir: root,
	}, nil
}

func (s *Scanner) resolve(dir string) string {
	// Clean the path so that it cannot possibly begin with ../.
	// If it did, the result of filepath.Join would be outside the
	// tree rooted at root.  We probably won't ever see a path
	// with .. in it, but be safe anyway.
	dir = path.Clean(dir)
	return filepath.Join(s.baseDir, dir)
}

func (gv VersionFolders) verifyRange(low, high *semver.Version) (*semver.Version, *semver.Version, error) {
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

// ScanFolders start scanning and returns all up and down files divided per version and revision
func (s *Scanner) ScanFolders() (VersionFolders, error) {
	schemaFolderName := s.config.Planner.SchemaFolder.FolderName
	allFolders, err := s.open(schemaFolderName, func(ver *semver.Version, dirName string) (*VersionFolder, error) {
		v := &VersionFolder{
			version:      ver,
			schemaFolder: &folderScripts{},
			extraFolders: make(map[string]*folderScripts), // Prepare map to avoid if statement in Open function
		}

		var err error
		if s.config.Planner.SchemaFolder.MigrationType == "up_down" {
			v.schemaFolder.up, v.schemaFolder.down, err = s.scanUpDownTypeFolder(dirName)
		} else {
			v.schemaFolder.up, err = s.scanChangeTypeFolder(dirName)
		}

		// Don't need to check down, it is checked in UpDown method, or is empty for Change type
		if err != nil || len(v.schemaFolder.up) == 0 {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return nil, err
	}

	for folderName, folderDetail := range s.config.Planner.Folders {
		_, err := s.open(folderName, func(ver *semver.Version, dirName string) (*VersionFolder, error) {
			for _, v := range allFolders {
				if v.version.Equal(ver) {
					fs := &folderScripts{}
					v.extraFolders[folderName] = fs

					var err error
					if folderDetail.MigrationType == "up_down" {
						fs.up, fs.down, err = s.scanUpDownTypeFolder(dirName)
					} else {
						fs.up, err = s.scanChangeTypeFolder(dirName)
					}

					// Don't need to check down, it is checked in UpDown method, or is empty for Change type
					if err != nil || len(v.schemaFolder.up) == 0 {
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
	}

	return allFolders, nil
}

func (s *Scanner) open(dir string,
	op func(ver *semver.Version, dirName string) (*VersionFolder, error),
) (VersionFolders, error) {
	dPath := s.resolve(filepath.Clean(dir))
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

	var versions VersionFolders

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

func (s *Scanner) scanChangeTypeFolder(dir string) ([]*MigrationFile, error) {
	f, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	files := make([]*MigrationFile, 0)
	for _, info := range list {
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			continue
		}

		match := dataCypher.FindStringSubmatch(info.Name())
		if len(match) != len(dataCypher.SubexpNames()) {
			return nil, fmt.Errorf("file '%s' has invalid name", path.Join(dir, info.Name()))
		}

		cf := &MigrationFile{
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
					return nil, fmt.Errorf("forbidden number '0' at file '%s'", cf.path)
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

func (s *Scanner) scanUpDownTypeFolder(dir string) ([]*MigrationFile, []*MigrationFile, error) {
	var ups, downs []*MigrationFile
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
			return fmt.Errorf("file '%s' has invalid name", path)
		}

		cf := &MigrationFile{
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
					return fmt.Errorf("forbidden number '0' at file '%s'", cf.path)
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
