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
	"flag"
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
	"github.com/spf13/pflag"

	"github.com/indykite/neo4j-graph-tool-core/config"
)

type (
	Scanner struct {
		config  *config.Config
		baseDir string
	}
	Batch    string
	FileType int

	// LocalFolders handles all migration files for all versions across all folders.
	LocalFolders []*LocalVersionFolder

	// LocalVersionFolder handles all migration files for single version across all folders.
	LocalVersionFolder struct {
		Version *semver.Version
		// SchemaFolder contains all up/down files of base schema folder.
		SchemaFolder *MigrationScripts
		// ExtraFolders contains all up/down files per folder, which name is key of the map.
		ExtraFolders map[string]*MigrationScripts
		// Snapshots contains all snapshot files for given version per batch, which name is key of the map.
		Snapshots map[Batch]*MigrationFile
	}

	// MigrationScripts handles all up and potentially down files.
	// If folder is 'change' type, only up files are used as this is only applied during up action.
	MigrationScripts struct {
		Up   []*MigrationFile
		Down []*MigrationFile
	}

	// MigrationFile specify all details about single file to run during migration.
	MigrationFile struct {
		FolderName  string
		Path        string
		FileType    FileType
		Timestamp   int64
		IsDowngrade bool
		IsSnapshot  bool
	}

	// DatabaseModel holds database version of all migrations of all folders.
	// Key is folder name and value contains all applied versions with all executed files.
	DatabaseModel map[string][]DatabaseGraphVersion
	// DatabaseGraphVersion holds all executed files per version.
	DatabaseGraphVersion struct {
		Version        *semver.Version
		FileTimestamps map[int64]bool
	}

	// TargetVersion holds version of a graph.
	TargetVersion struct {
		Version  *semver.Version `json:"version,omitempty"`
		Revision int64           `json:"rev,omitempty"`
	}
)

const (
	Cypher FileType = iota
	Command
)

var (
	upDownFilePattern   = regexp.MustCompile(`(?i)^(?P<commit>\d+)_(?P<direction>up|down)_(?P<name>\w+)\.(?P<type>cypher|run)$`) // nolint:lll
	changeFilePattern   = regexp.MustCompile(`(?i)^(?P<commit>\d+)_(?P<name>\w+)\.(?P<type>cypher|run)$`)
	snapshotFilePattern = regexp.MustCompile(`^(.*)_(v[0-9.]+)\.(cypher|run)$`)

	_ fmt.Stringer = DatabaseModel{}  // Be sure DatabaseModel implements String method
	_ flag.Value   = &TargetVersion{} // Be sure GraphVersion can be set as flag value in CLI tools
	_ pflag.Value  = &TargetVersion{} // Be sure GraphVersion can be set as pflag (used with Cobra) value in CLI tools
)

// String converts DatabaseModel into JSON which is shrink.
func (dbm DatabaseModel) String() string {
	return dbm.toJSON(3)
}

func (dbm DatabaseModel) toJSON(limitFiles int) string {
	builder := &strings.Builder{}
	builder.WriteByte('{')

	folderEls := 0
	for folderName, versions := range dbm {
		if folderEls > 0 {
			builder.WriteByte(',')
		}
		folderEls++
		builder.WriteString(`"` + folderName + `":{`)

		versionEls := 0
		for _, ver := range versions {
			if versionEls > 0 {
				builder.WriteByte(',')
			}
			versionEls++
			builder.WriteString(`"` + ver.Version.String() + `":`)

			executed := make([]int64, 0, len(ver.FileTimestamps))
			for k := range ver.FileTimestamps {
				executed = append(executed, k)
			}
			sort.Slice(executed, func(i, j int) bool { return executed[i] < executed[j] })
			executedCnt := len(executed)
			builder.WriteByte('[')

			startAt := 0
			if executedCnt >= limitFiles {
				builder.WriteString(fmt.Sprintf(`"... %d more"`, executedCnt-limitFiles))
				startAt = executedCnt - limitFiles
			}

			for i := startAt; i < executedCnt; i++ {
				if i > 0 {
					builder.WriteByte(',')
				}
				builder.WriteString(strconv.FormatInt(executed[i], 10))
			}
			builder.WriteByte(']')
		}
		builder.WriteByte('}')
	}
	builder.WriteByte('}')

	return builder.String()
}

// MarshalJSON returns the JSON encoding of DatabaseModel.
func (dbm DatabaseModel) MarshalJSON() ([]byte, error) {
	return []byte(dbm.toJSON(math.MaxInt)), nil
}

// ContainsHigherVersion check if there is folder with higher version than specified.
func (dbm DatabaseModel) ContainsHigherVersion(folder string, version *semver.Version) bool {
	folderVersion, hasFolder := dbm[folder]
	if !hasFolder {
		return false
	}
	for _, v := range folderVersion {
		if version.LessThan(v.Version) {
			return true
		}
	}

	return false
}

// GetFileTimestamps returns all executed files from DB.
func (dbm DatabaseModel) GetFileTimestamps(folder string, version *semver.Version) map[int64]bool {
	folderVersion, hasFolder := dbm[folder]
	if !hasFolder {
		return nil
	}
	for _, v := range folderVersion {
		if version.Equal(v.Version) {
			return v.FileTimestamps
		}
	}

	return nil
}

// HasAnyVersion checks if there is any version stored in DB.
func (dbm DatabaseModel) HasAnyVersion() bool {
	for _, f := range dbm {
		for _, gv := range f {
			if len(gv.FileTimestamps) > 0 {
				return true
			}
		}
	}
	return false
}

// ParseTargetVersion parse string as semver version with revision.
func ParseTargetVersion(v string) (*TargetVersion, error) {
	ver, err := semver.NewVersion(v)
	if err != nil {
		return nil, err
	}
	vs := &TargetVersion{Version: ver}
	if ver.Metadata() != "" {
		vs.Revision, err = strconv.ParseInt(ver.Metadata(), 10, 0)
		if err != nil {
			return nil, fmt.Errorf("metadata are not numeric: '%s'", ver.Metadata())
		}
	}
	return vs, nil
}

func (a *TargetVersion) String() string {
	if a == nil || a.Version == nil {
		return ""
	}
	if a.Revision != 0 {
		v, _ := a.Version.SetMetadata(fmt.Sprintf("%02d", a.Revision))
		return v.String()
	}
	return a.Version.String()
}

// Set sets version from string. Can be used with flag package.
func (a *TargetVersion) Set(v string) error {
	if a == nil {
		return errors.New("object is not initialized")
	}
	gv, err := ParseTargetVersion(v)
	if err != nil {
		return err
	}

	a.Version = gv.Version
	a.Revision = gv.Revision
	return nil
}

// Type returns type name, so it can be used with flag package.
func (a *TargetVersion) Type() string {
	return "GraphVersion"
}

// NewScanner creates file scanner.
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

// ScanFolders start scanning and returns all up and down files divided per version and revision.
// Result is not sorted by default, use SortByVersion() to sort it by semver version.
func (s *Scanner) ScanFolders() (LocalFolders, error) {
	localFolders, err := s.scanSchemaAndExtraFolders()
	if err != nil {
		return nil, err
	}

	if err = s.addSnapshotsTo(localFolders); err != nil {
		return nil, err
	}

	return localFolders, err
}

func (s *Scanner) scanSchemaAndExtraFolders() (LocalFolders, error) {
	schemaFolderName := s.config.Planner.SchemaFolder.FolderName
	localFolders, err := s.open(schemaFolderName, func(ver *semver.Version, path string) (*LocalVersionFolder, error) {
		v := &LocalVersionFolder{
			Version:      ver,
			ExtraFolders: make(map[string]*MigrationScripts), // Prepare map to avoid if statement in Open function
			Snapshots:    make(map[Batch]*MigrationFile),     // Prepare map to avoid if statement in Open function
		}

		var err error
		if s.config.Planner.SchemaFolder.MigrationType == "up_down" {
			v.SchemaFolder, err = s.scanUpDownTypeFolder(schemaFolderName, path)
		} else {
			v.SchemaFolder, err = s.scanChangeTypeFolder(schemaFolderName, path)
		}

		// Don't need to check down, it is checked in scanUpDownTypeFolder method, or is empty for Change type.
		if err != nil || len(v.SchemaFolder.Up) == 0 {
			return nil, err
		}
		return v, nil
	})
	if err != nil {
		return nil, err
	}

	// Iterate over all defined folders and if same version exists under schema folder, add it there as extra folders.
	for folderName, folderDetail := range s.config.Planner.Folders {
		_, err := s.open(folderName, func(ver *semver.Version, path string) (*LocalVersionFolder, error) {
			// Verify that any additional folder has always schema folder for same version
			for _, v := range localFolders {
				if !v.Version.Equal(ver) {
					continue
				}

				var err error
				if folderDetail.MigrationType == "up_down" {
					v.ExtraFolders[folderName], err = s.scanUpDownTypeFolder(folderName, path)
				} else {
					v.ExtraFolders[folderName], err = s.scanChangeTypeFolder(folderName, path)
				}

				return nil, err
			}
			return nil, fmt.Errorf("unspecified schema for version of '%s'", path)
		})
		if err != nil {
			return nil, err
		}
	}

	return localFolders, nil
}

func (s *Scanner) addSnapshotsTo(localFolders LocalFolders) error {
	dirPath := s.resolve(filepath.Clean("snapshots"))
	f, err := os.Open(filepath.Clean(dirPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("open: %s is not a directory", dirPath)
	}
	files, err := f.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, fileName := range files {
		if strings.HasPrefix(fileName, ".") {
			// Ignore hidden files
			continue
		}
		match := snapshotFilePattern.FindStringSubmatch(fileName)
		if len(match) != 4 {
			return fmt.Errorf("invalid snapshot name '%s'", fileName)
		}
		batchName := match[1]
		if batchName != "schema" {
			if _, exists := s.config.Planner.Batches[batchName]; !exists {
				return fmt.Errorf("unknown batch name '%s' based on snapshot name '%s'", batchName, fileName)
			}
		}
		version, err := semver.NewVersion(match[2])
		if err != nil {
			return fmt.Errorf("invalid snapshot version '%s': %s", fileName, err.Error())
		}

		var fileType FileType
		if match[3] == "run" {
			fileType = Command
		} else {
			fileType = Cypher
		}

		matchSchemaVersion := false
		for _, localFolder := range localFolders {
			if !localFolder.Version.Equal(version) {
				continue
			}
			localFolder.Snapshots[Batch(batchName)] = &MigrationFile{
				FolderName: "snapshots",
				Path:       path.Join(dirPath, fileName),
				FileType:   fileType,
				IsSnapshot: true,
			}
			matchSchemaVersion = true
			break
		}
		if !matchSchemaVersion {
			return fmt.Errorf("version '%s' in snapshot '%s' is not defined in schema", version.String(), fileName)
		}
	}
	return nil
}

func (s *Scanner) open(
	folderName string,
	op func(ver *semver.Version, path string) (*LocalVersionFolder, error),
) (LocalFolders, error) {
	dirPath := s.resolve(filepath.Clean(folderName))
	f, err := os.Open(filepath.Clean(dirPath))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("open: %s is not a directory", dirPath)
	}
	dirNames, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	var versions LocalFolders

	for _, dn := range dirNames {
		if strings.HasPrefix(dn, ".") {
			// Ignore hidden files
			continue
		}
		if !strings.HasPrefix(dn, "v") {
			return nil, fmt.Errorf("folder name '%s' does not start with letter 'v' at %s", dn, dirPath)
		}
		ver, err := semver.NewVersion(dn)
		if err != nil {
			return nil, fmt.Errorf("%v - %s", err, path.Join(dirPath, dn))
		}
		// Scan files
		v, err := op(ver, path.Join(dirPath, dn))
		if err != nil {
			return nil, err
		}
		if v != nil {
			versions = append(versions, v)
		}
	}

	return versions, nil
}

func (s *Scanner) scanChangeTypeFolder(folderName, dirPath string) (*MigrationScripts, error) {
	scripts, err := s.scanFolder(folderName, dirPath, changeFilePattern)
	return scripts, err
}

func (s *Scanner) scanUpDownTypeFolder(folderName, dirPath string) (*MigrationScripts, error) {
	scripts, err := s.scanFolder(folderName, dirPath, upDownFilePattern)
	if err != nil {
		return nil, err
	}

	u, d := len(scripts.Up), len(scripts.Down)
	if u != d {
		return nil, fmt.Errorf("inconsistent state in '%s': found %d up and %d down script", dirPath, u, d)
	}

	for i, v := range scripts.Up {
		if scripts.Down[d-i-1].Timestamp != v.Timestamp {
			return nil, fmt.Errorf("inconsistent state: missing down part of '%s'", v.Path)
		}
	}

	return scripts, err
}

func (s *Scanner) scanFolder(folderName, dirPath string, fileNamePattern *regexp.Regexp) (*MigrationScripts, error) {
	f, err := os.Open(filepath.Clean(dirPath))
	if err != nil {
		return nil, err
	}
	list, err := f.ReadDir(-1)
	_ = f.Close()
	if err != nil {
		return nil, err
	}

	scripts := &MigrationScripts{}
	for _, info := range list {
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			continue
		}
		fileName := info.Name()
		mf := &MigrationFile{
			FolderName: folderName,
			Path:       path.Join(dirPath, fileName),
		}

		match := fileNamePattern.FindStringSubmatch(fileName)
		if len(match) != len(fileNamePattern.SubexpNames()) {
			return nil, fmt.Errorf("file '%s' has invalid name", path.Join(dirPath, fileName))
		}

		if err = mf.parseFileName(match, fileNamePattern.SubexpNames()); err != nil {
			return nil, err
		}

		if mf.IsDowngrade {
			for _, v := range scripts.Down {
				if v.Timestamp == mf.Timestamp {
					return nil, fmt.Errorf("can't have two down commit match '%d' in folder '%s'", v.Timestamp, dirPath)
				}
			}
			scripts.Down = append(scripts.Down, mf)
		} else {
			for _, v := range scripts.Up {
				if v.Timestamp == mf.Timestamp {
					return nil, fmt.Errorf("can't have two commit match '%d' in folder '%s'", v.Timestamp, dirPath)
				}
			}
			scripts.Up = append(scripts.Up, mf)
		}
	}

	// Listing all files from folder might not be in lexical order. To be sure sort all files before further process.
	scripts.SortUpFiles()
	scripts.SortDownFiles()
	return scripts, nil
}

func (mf *MigrationFile) parseFileName(match, subExpNames []string) error {
	var err error
	for i, subExp := range subExpNames {
		switch subExp {
		case "commit":
			mf.Timestamp, err = strconv.ParseInt(match[i], 10, 0)
			if err != nil {
				return err
			}
			if mf.Timestamp == 0 {
				return fmt.Errorf("forbidden number '0' at file '%s'", mf.Path)
			}
		case "direction":
			mf.IsDowngrade = match[i] == "down"
		case "type":
			if match[i] == "run" {
				mf.FileType = Command
			} else {
				mf.FileType = Cypher
			}
		}
	}
	return nil
}

// SortByVersion all folders in ascending order.
func (lc LocalFolders) SortByVersion() {
	sort.Slice(lc, func(i, j int) bool {
		return lc[i].Version.LessThan(lc[j].Version)
	})
}

// ContainsMigrations verify if there is at least some up or down migration.
func (ms *MigrationScripts) ContainsMigrations() bool {
	if ms == nil {
		return false
	}
	return len(ms.Up) > 0 || len(ms.Down) > 0
}

// Add merge migrations scripts with in scripts.
func (ms *MigrationScripts) Add(in *MigrationScripts) {
	if in != nil {
		ms.Up = append(ms.Up, in.Up...)
		ms.Down = append(ms.Down, in.Down...)
	}
}

// SortUpFiles in ascending order.
func (ms *MigrationScripts) SortUpFiles() {
	if ms == nil {
		return
	}
	sort.Slice(ms.Up, func(i, j int) bool {
		return ms.Up[i].Timestamp < ms.Up[j].Timestamp
	})
}

// SortDownFiles in descending order.
func (ms *MigrationScripts) SortDownFiles() {
	if ms == nil {
		return
	}
	sort.Slice(ms.Down, func(i, j int) bool {
		return ms.Down[i].Timestamp > ms.Down[j].Timestamp
	})
}