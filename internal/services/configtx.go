package services

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Config changes are transactional (TomasZmek/cpm#23): ApplyChange takes a
// snapshot of the configuration files, runs the change, validates and
// reloads Caddy and restores the snapshot when Caddy rejects the result, so
// an invalid rule is never left on disk.

// inPlaceFiles are written in place instead of via a temp file + rename:
// the example docker-compose.yml bind-mounts them into the Caddy container
// as single files, and a rename would leave the container on the old inode
// (Caddy would keep reading the previous version).
var inPlaceFiles = map[string]bool{
	"Caddyfile":      true,
	"snippets.caddy": true,
}

// writeConfigFile writes a configuration file so that a crash or a full disk
// never leaves it half-written: the data goes to a temp file in the same
// directory, is synced and then renamed over the target. Files Caddy may
// bind-mount individually (inPlaceFiles) are written in place after the
// same temp-file check, so only the final write touches them.
func writeConfigFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return err
	}

	if inPlaceFiles[filepath.Base(path)] {
		// The data fits on disk (the temp file proves it); now overwrite.
		cleanup()
		return os.WriteFile(path, data, perm)
	}
	if err := os.Rename(tmpName, path); err != nil {
		// A rename onto a bind-mounted single file fails with EBUSY:
		// fall back to writing in place.
		cleanup()
		return os.WriteFile(path, data, perm)
	}
	return nil
}

// snapshotTopLevelFiles are the files in the config dir a change can touch
// (besides the rule files under sites/).
var snapshotTopLevelFiles = []string{
	"Caddyfile",
	"snippets.caddy",
	".snippets_config.json",
	"wildcard.json",
	"settings.json",
}

type snapshotFile struct {
	data []byte
	perm fs.FileMode
}

// ConfigSnapshot is an in-memory copy of the configuration files that
// ApplyChange restores when Caddy rejects a change.
type ConfigSnapshot struct {
	configDir string
	sitesDir  string
	files     map[string]snapshotFile // absolute path -> content
}

// SnapshotConfig reads the managed top-level files of configDir and every
// file under sitesDir.
func SnapshotConfig(configDir, sitesDir string) (*ConfigSnapshot, error) {
	s := &ConfigSnapshot{configDir: configDir, sitesDir: sitesDir, files: map[string]snapshotFile{}}
	for _, name := range snapshotTopLevelFiles {
		if err := s.add(filepath.Join(configDir, name)); err != nil {
			return nil, err
		}
	}
	err := filepath.WalkDir(sitesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.Type().IsRegular() && !isTempFile(d.Name()) {
			return s.add(path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("snapshot of %s: %w", sitesDir, err)
	}
	return s, nil
}

func (s *ConfigSnapshot) add(path string) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s.files[path] = snapshotFile{data: data, perm: info.Mode().Perm()}
	return nil
}

func isTempFile(name string) bool {
	return strings.HasPrefix(name, ".") && strings.Contains(name, ".tmp-")
}

// Restore puts every snapshotted file back (only those that changed),
// recreates deleted ones and removes the managed top-level files and the
// files under sites/ that did not exist when the snapshot was taken.
func (s *ConfigSnapshot) Restore() error {
	var errs []error

	// Files created after the snapshot
	for _, name := range snapshotTopLevelFiles {
		path := filepath.Join(s.configDir, name)
		if _, ok := s.files[path]; !ok {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	_ = filepath.WalkDir(s.sitesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if _, ok := s.files[path]; !ok {
				if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
					errs = append(errs, err)
				}
			}
		}
		return nil
	})

	// Changed or deleted files
	for path, f := range s.files {
		current, err := os.ReadFile(path)
		if err == nil && bytes.Equal(current, f.data) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := writeConfigFile(path, f.data, f.perm); err != nil {
			errs = append(errs, fmt.Errorf("restore %s: %w", filepath.Base(path), err))
		}
	}
	return errors.Join(errs...)
}

// ApplyChange runs change and then validates and reloads Caddy. When change
// fails, or Caddy rejects the new configuration, every configuration file is
// restored to its state before the call: the error of change is returned, or
// the ReloadResult with RolledBack set. When Docker cannot be reached the
// change is kept (it could not be validated) and DockerUnreachable is set,
// as before.
func (c *CaddyService) ApplyChange(change func() error) (*ReloadResult, error) {
	c.changeMu.Lock()
	defer c.changeMu.Unlock()

	snap, err := SnapshotConfig(c.config.ConfigDir, c.config.SitesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to back up the configuration: %w", err)
	}

	if err := change(); err != nil {
		if rbErr := snap.Restore(); rbErr != nil {
			return nil, fmt.Errorf("%w (restoring the previous configuration failed: %v)", err, rbErr)
		}
		return nil, err
	}

	reload := c.reloadFn
	if reload == nil {
		reload = c.ReloadWithValidation
	}
	result := reload()
	if result.Success || result.DockerUnreachable {
		return result, nil
	}

	result.RolledBack = true
	if rbErr := snap.Restore(); rbErr != nil {
		result.RolledBack = false
		result.RollbackError = rbErr.Error()
	}
	return result, nil
}
