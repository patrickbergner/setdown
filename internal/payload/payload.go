// Package payload holds what setdown carries inside itself: the template, the fonts and the cmarker package, which
// Typst reads as files, the license texts, and the Typst release for the platform (see embed_<os>_<arch>.go).
//
// tools/fetch puts everything but the template into assets/, which is not committed; without it the build fails
// here, which is the intended way to notice. It also writes assets/manifest.json with the versions and a payload id
// per platform, so starting up hashes nothing.
package payload

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Template is setdown's Typst template. It reaches Typst on stdin.
//
//go:embed template.typ
var Template []byte

var (
	//go:embed all:assets/files
	filesRoot embed.FS
	//go:embed all:assets/licenses
	licensesRoot embed.FS
	//go:embed assets/manifest.json
	manifestJSON []byte
)

// Manifest is assets/manifest.json, as tools/fetch writes it.
type Manifest struct {
	Typst string `json:"typst"`
	Fonts []struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	} `json:"fonts"`
	Cmarker  string            `json:"cmarker"`
	IDs      map[string]string `json:"ids"` // payload ids by platform, e.g. windows-amd64
	Licenses []struct {
		Title string `json:"title"`
		File  string `json:"file"` // for License
	} `json:"licenses"`
}

// ReadManifest returns the embedded manifest.
func ReadManifest() (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return nil, fmt.Errorf("embedded manifest: %w", err)
	}
	return &m, nil
}

// License returns one of the embedded license texts, by the file name the manifest gives.
func License(file string) ([]byte, error) {
	return licensesRoot.ReadFile("assets/licenses/" + file)
}

// A Payload is what Typst needs as files: Typst itself, and the fonts and packages with their sizes.
type Payload struct {
	ID    string // the payload id for this platform; cache directories carry it in their names
	FS    fs.FS  // fonts/ and packages/
	Files []File
	Typst []byte
}

// A File is one of the fonts and packages.
type File struct {
	Path string // slash-separated, below the root of FS
	Size int64
}

// Embedded returns the payload built into this binary. Listing it reads only the embedded directory, so it hashes
// nothing.
func Embedded() (*Payload, error) {
	m, err := ReadManifest()
	if err != nil {
		return nil, err
	}
	platform := runtime.GOOS + "-" + runtime.GOARCH
	id := m.IDs[platform]
	if id == "" {
		return nil, fmt.Errorf("built without a payload id for %s; run go run ./tools/fetch before building", platform)
	}
	sub, err := fs.Sub(filesRoot, "assets/files")
	if err != nil {
		return nil, err
	}
	return New(id, sub, typst)
}

// New makes a payload of the files in fsys and the given Typst.
func New(id string, fsys fs.FS, typst []byte) (*Payload, error) {
	p := &Payload{ID: id, FS: fsys, Typst: typst}
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		p.Files = append(p.Files, File{name, info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(p.Files) == 0 {
		return nil, errors.New("the payload holds no fonts or packages")
	}
	return p, nil
}

// Write writes the fonts and packages into dir, but not Typst, which the platforms treat differently. stop is asked
// before each file and ends the writing with its error.
func (p *Payload) Write(dir string, mode fs.FileMode, stop func() error) error {
	for _, f := range p.Files {
		if err := stop(); err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		src, err := p.FS.Open(f.Path)
		if err != nil {
			return err
		}
		err = WriteNew(dst, src, mode)
		src.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// WriteNew writes a file that must not exist yet. setdown never rewrites a file in place: on macOS that can get a
// program killed whose signature check the system has cached.
func WriteNew(file string, r io.Reader, mode fs.FileMode) error {
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
