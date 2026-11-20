// Command fetch downloads what setdown embeds: the Typst releases, fonts and cmarker pinned in tools/versions.json.
// It keeps the downloads in tools/download-cache/, checks each against its SHA-256, and unpacks what the binaries
// embed into internal/payload/assets/, with a manifest that holds the versions and one payload id per platform.
// build.sh and run-tests.sh run it; nothing it writes is committed.
//
//	go run ./tools/fetch                            all five platforms
//	go run ./tools/fetch -target linux/arm64        only some (repeatable); the others stay if still current
//	go run ./tools/fetch -bump                      pin the latest releases of Typst, the fonts and cmarker
//	go run ./tools/fetch -check-signature <file>    check that a macOS binary carries a code signature
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// A target is a platform setdown is built for.
type target struct {
	goos, goarch string
	triple       string // the Rust target in the name of Typst's release asset
}

var targets = []target{
	{"linux", "amd64", "x86_64-unknown-linux-musl"},
	{"linux", "arm64", "aarch64-unknown-linux-musl"},
	{"windows", "amd64", "x86_64-pc-windows-msvc"},
	{"windows", "arm64", "aarch64-pc-windows-msvc"},
	{"darwin", "arm64", "aarch64-apple-darwin"},
}

func (t target) String() string { return t.goos + "/" + t.goarch }

// key names the target in the assets: its directory under typst/ and its payload id in the manifest.
func (t target) key() string { return t.goos + "-" + t.goarch }

func (t target) typstName() string {
	if t.goos == "windows" {
		return "typst.exe"
	}
	return "typst"
}

// Paths below the module root.
const (
	versionsFile = "tools/versions.json"
	downloadsDir = "tools/download-cache"
	assetsDir    = "internal/payload/assets"
	templateFile = "internal/payload/template.typ"
)

// versions is tools/versions.json: the pinned releases with their SHA-256.
type versions struct {
	Typst   typstPin   `json:"typst"`
	Fonts   []fontPin  `json:"fonts"`
	Cmarker cmarkerPin `json:"cmarker"`
}

type typstPin struct {
	Repo   string           `json:"repo"`
	Tag    string           `json:"tag"`
	Assets map[string]asset `json:"assets"` // by target, e.g. linux/amd64
}

type asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// A fontPin is one font family. The first fields say where it comes from and are edited by hand; -bump rewrites the
// rest. A family comes either from a release archive (asset and files) or as one file of a repository that makes no
// releases, such as google/fonts (path); then tag is the commit that last changed the file.
type fontPin struct {
	Key           string `json:"key"`               // its directory under fonts/
	Name          string `json:"name"`              // as setdown --version shows it
	Repo          string `json:"repo"`              // the GitHub repository
	Asset         string `json:"asset,omitempty"`   // a regular expression for the release asset's name
	Files         string `json:"files,omitempty"`   // the files to keep from the archive, a find -path pattern
	Path          string `json:"path,omitempty"`    // instead of asset and files: the font file's path in the repository
	License       string `json:"license"`           // the license file's path in the repository
	Tag           string `json:"tag"`               // the release's tag, or with path the commit
	Version       string `json:"version,omitempty"` // with path: the font's version, from its name table
	URL           string `json:"url"`
	SHA256        string `json:"sha256"`
	LicenseSHA256 string `json:"license_sha256"`
}

func (f fontPin) licenseURL() string {
	return "https://raw.githubusercontent.com/" + f.Repo + "/" + f.Tag + "/" + f.License
}

// fileURL is where a font pinned by path lies at the commit in tag. google/fonts names variable fonts after their
// axes, as in NotoEmoji[wght].ttf, so each segment is escaped.
func (f fontPin) fileURL() string {
	segments := strings.Split(f.Path, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return "https://raw.githubusercontent.com/" + f.Repo + "/" + f.Tag + "/" + strings.Join(segments, "/")
}

// version is what setdown --version shows for the family: the release's tag, or the version of a file pinned by path,
// since its commit says nothing to a reader.
func (f fontPin) version() string {
	if f.Path != "" {
		return f.Version
	}
	return f.Tag
}

type cmarkerPin struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

// manifest is the manifest.json that setdown embeds; keep it in step with Manifest in internal/payload.
type manifest struct {
	Typst    string            `json:"typst"`
	Fonts    []fontVersion     `json:"fonts"`
	Cmarker  string            `json:"cmarker"`
	IDs      map[string]string `json:"ids"`
	Licenses []licenseFile     `json:"licenses"`
}

type fontVersion struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

type licenseFile struct {
	Title string `json:"title"`
	File  string `json:"file"` // below licenses/
}

// targetList collects the -target flags.
type targetList []target

func (l *targetList) String() string { return fmt.Sprint(*l) }

func (l *targetList) Set(s string) error {
	for _, t := range targets {
		if t.String() == s {
			*l = append(*l, t)
			return nil
		}
	}
	names := make([]string, len(targets))
	for i, t := range targets {
		names[i] = t.String()
	}
	return fmt.Errorf("unknown target %s (one of %s)", s, strings.Join(names, ", "))
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("fetch: ")
	var only targetList
	flag.Var(&only, "target", "fetch only what the binary for `os/arch` embeds (repeatable)")
	bump := flag.Bool("bump", false, "pin the latest releases of Typst, the fonts and cmarker, then stop")
	signed := flag.String("check-signature", "", "check that the macOS binary `file` carries a code signature, then stop")
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("unexpected argument %q (see -h)", flag.Arg(0))
	}
	if *signed != "" {
		// Apple Silicon runs only signed code. Go's linker signs darwin/arm64 binaries ad hoc, also when
		// cross-compiling; build.sh checks that it did.
		if err := checkSignature(*signed); err != nil {
			log.Fatalf("%s: %v", *signed, err)
		}
		return
	}
	root, err := moduleRoot()
	if err != nil {
		log.Fatal(err)
	}
	if *bump {
		if len(only) > 0 {
			log.Fatal("-bump pins all targets and takes no -target")
		}
		err = runBump(root)
	} else {
		if len(only) == 0 {
			only = targets
		}
		err = runFetch(root, only)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// moduleRoot finds setdown's directory, the closest one at or above the working directory that holds go.mod and the
// pinned versions.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(at(dir, versionsFile)); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no " + versionsFile + " at or above the working directory; run this in setdown's directory")
		}
		dir = parent
	}
}

func at(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

func readVersions(root string) (*versions, error) {
	data, err := os.ReadFile(at(root, versionsFile))
	if err != nil {
		return nil, err
	}
	var v versions
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", versionsFile, err)
	}
	return &v, nil
}

func writeVersions(root string, v *versions) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(at(root, versionsFile), append(data, '\n'), 0o644)
}

// runFetch fills the assets for the selected targets. Typst for other targets stays when it was unpacked from the
// pinned release, so building for one platform does not take the assets of another away; everything else is
// unpacked anew, which takes a second.
func runFetch(root string, selected []target) error {
	v, err := readVersions(root)
	if err != nil {
		return err
	}
	if err := checkTemplate(root, v.Cmarker.Version); err != nil {
		return err
	}
	downloads := at(root, downloadsDir)
	assets := at(root, assetsDir)
	files := filepath.Join(assets, "files")
	licenses := filepath.Join(assets, "licenses")
	for _, dir := range []string{files, licenses, filepath.Join(assets, "manifest.json")} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	m := manifest{Typst: v.Typst.Tag, Cmarker: v.Cmarker.Version, IDs: map[string]string{}}

	// Typst, one release asset per target. A marker records which archive a target's Typst came from.
	var present []target
	for _, t := range targets {
		a, pinned := v.Typst.Assets[t.String()]
		dir := filepath.Join(assets, "typst", t.key())
		marker := filepath.Join(dir, "source.sha256")
		current := false
		if data, err := os.ReadFile(marker); err == nil && pinned && strings.TrimSpace(string(data)) == a.SHA256 {
			_, err := os.Stat(filepath.Join(dir, t.typstName()))
			current = err == nil
		}
		if !isSelected(t, selected) {
			if current {
				present = append(present, t)
			} else if err := os.RemoveAll(dir); err != nil {
				return err
			}
			continue
		}
		if !pinned {
			return fmt.Errorf("%s pins no Typst for %s", versionsFile, t)
		}
		if !current {
			file, err := fetch(downloads, path.Base(a.URL), a.URL, a.SHA256)
			if err != nil {
				return err
			}
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
			if err := unpackTypst(file, t, dir); err != nil {
				return err
			}
			if t.goos == "darwin" {
				// Apple Silicon runs only signed code; Typst's linker signs the release binary ad hoc.
				if err := checkSignature(filepath.Join(dir, t.typstName())); err != nil {
					return fmt.Errorf("Typst for %s: %w", t, err)
				}
			}
			if err := os.WriteFile(marker, []byte(a.SHA256+"\n"), 0o644); err != nil {
				return err
			}
		}
		present = append(present, t)
	}
	// setdown's own license comes first: the releases are bare binaries, so it has to travel inside them.
	if err := copyFile(at(root, "LICENSE"), filepath.Join(licenses, "setdown-LICENSE"), 0o644); err != nil {
		return err
	}
	m.Licenses = append(m.Licenses, licenseFile{"setdown, LICENSE", "setdown-LICENSE"})

	// The license and notice are the same in every release asset.
	for _, name := range []string{"LICENSE", "NOTICE"} {
		src := filepath.Join(assets, "typst", present[0].key(), name)
		if err := copyFile(src, filepath.Join(licenses, "typst-"+name), 0o644); err != nil {
			return err
		}
		m.Licenses = append(m.Licenses, licenseFile{"Typst " + v.Typst.Tag + ", " + name, "typst-" + name})
	}

	// The fonts and their licenses.
	for _, f := range v.Fonts {
		file, err := fetch(downloads, path.Base(f.URL), f.URL, f.SHA256)
		if err != nil {
			return err
		}
		dir := filepath.Join(files, "fonts", f.Key)
		if f.Path != "" {
			err = unpackFontFile(file, f, dir)
		} else {
			err = unpackFonts(file, f, dir)
		}
		if err != nil {
			return err
		}
		licenseName := f.Key + "-" + path.Base(f.License)
		file, err = fetch(downloads, licenseName, f.licenseURL(), f.LicenseSHA256)
		if err != nil {
			return err
		}
		if err := copyFile(file, filepath.Join(licenses, licenseName), 0o644); err != nil {
			return err
		}
		m.Fonts = append(m.Fonts, fontVersion{f.Name, f.version()})
		m.Licenses = append(m.Licenses, licenseFile{f.Name + " " + f.version() + ", " + path.Base(f.License), licenseName})
	}

	// cmarker, as a package in the local package directory.
	file, err := fetch(downloads, path.Base(v.Cmarker.URL), v.Cmarker.URL, v.Cmarker.SHA256)
	if err != nil {
		return err
	}
	pkg := filepath.Join(files, "packages", "preview", "cmarker", v.Cmarker.Version)
	if err := unpackPackage(file, pkg); err != nil {
		return err
	}
	if err := checkPackage(filepath.Join(pkg, "typst.toml"), v.Cmarker.Version, v.Typst.Tag); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(pkg, "LICENSE"), filepath.Join(licenses, "cmarker-LICENSE"), 0o644); err != nil {
		return fmt.Errorf("cmarker's license: %w", err)
	}
	m.Licenses = append(m.Licenses, licenseFile{"cmarker " + v.Cmarker.Version + ", LICENSE", "cmarker-LICENSE"})

	// Go's standard library and the modules compiled into setdown, whose BSD and Apache licenses ask for their notices
	// to travel with the binary as well.
	goFiles, err := goLicenses(root)
	if err != nil {
		return err
	}
	for _, l := range goFiles {
		if err := copyFile(l.src, filepath.Join(licenses, l.file), 0o644); err != nil {
			return err
		}
		m.Licenses = append(m.Licenses, licenseFile{l.title, l.file})
	}

	// Payload ids: one per target, over Typst and the files next to it in the cache.
	common, err := hashTree(files)
	if err != nil {
		return err
	}
	for _, t := range present {
		h, err := hashFile(filepath.Join(assets, "typst", t.key(), t.typstName()))
		if err != nil {
			return err
		}
		m.IDs[t.key()] = payloadID(append(common[:len(common):len(common)], h.named(t.typstName())))
		log.Printf("%s: payload %s", t, m.IDs[t.key()])
	}
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(assets, "manifest.json"), append(data, '\n'), 0o644)
}

func isSelected(t target, selected []target) bool {
	for _, s := range selected {
		if s == t {
			return true
		}
	}
	return false
}

// checkTemplate makes sure the template imports the cmarker that gets embedded: Typst would otherwise download the
// version it names.
func checkTemplate(root, version string) error {
	data, err := os.ReadFile(at(root, templateFile))
	if err != nil {
		return err
	}
	match := regexp.MustCompile(`#import "@preview/cmarker:([^"]+)"`).FindSubmatch(data)
	if match == nil {
		return errors.New(templateFile + " does not import @preview/cmarker")
	}
	if string(match[1]) != version {
		return fmt.Errorf("%s imports cmarker %s, but %s pins %s", templateFile, match[1], versionsFile, version)
	}
	return nil
}

// checkPackage compares a package's typst.toml with the pinned versions: its own and the Typst it needs at least.
func checkPackage(file, version, typstTag string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	field := func(name string) string {
		m := regexp.MustCompile(`(?m)^` + name + `\s*=\s*"([^"]*)"`).FindSubmatch(data)
		if m == nil {
			return ""
		}
		return string(m[1])
	}
	if got := field("version"); got != version {
		return fmt.Errorf("cmarker's typst.toml says version %q, versions.json pins %s", got, version)
	}
	if need := field("compiler"); need != "" && compareVersions(strings.TrimPrefix(typstTag, "v"), need) < 0 {
		return fmt.Errorf("cmarker %s needs Typst %s or later, versions.json pins %s", version, need, typstTag)
	}
	return nil
}

// compareVersions compares dotted version numbers such as 0.15.1 and 0.15.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// fetch returns the path of a download in dir, downloading it unless a file with the expected SHA-256 is already there.
func fetch(dir, name, url, want string) (string, error) {
	file := filepath.Join(dir, name)
	if h, err := hashFile(file); err == nil && h.sum == want {
		return file, nil
	}
	data, err := download(url)
	if err != nil {
		return "", err
	}
	if got := sha256Hex(data); got != want {
		return "", fmt.Errorf("checksum mismatch for %s: %s has %s, the download has %s", url, versionsFile, want, got)
	}
	return file, writeAtomic(file, data)
}

// fetchNew downloads a file whose SHA-256 is not known yet, keeps it in dir under name and returns its SHA-256.
func fetchNew(dir, name, url string) (string, error) {
	data, err := download(url)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), writeAtomic(filepath.Join(dir, name), data)
}

var client = &http.Client{Timeout: 10 * time.Minute}

func download(url string) ([]byte, error) {
	log.Printf("downloading %s", url)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		data, err := get(url, "")
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func get(url, accept string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "setdown-build")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func writeAtomic(file string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// eachFile calls fn for every regular file in a .zip, .tar.xz or .tar.gz archive, with its name made relative and
// clean.
func eachFile(archive string, fn func(name string, r io.Reader) error) error {
	clean := func(name string) string { return strings.TrimPrefix(path.Clean("/"+name), "/") }
	if strings.HasSuffix(archive, ".zip") {
		z, err := zip.OpenReader(archive)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			if !f.Mode().IsRegular() {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return err
			}
			err = fn(clean(f.Name), r)
			r.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	var r io.Reader
	switch {
	case strings.HasSuffix(archive, ".tar.xz"):
		if r, err = xz.NewReader(file); err != nil {
			return err
		}
	case strings.HasSuffix(archive, ".tar.gz"):
		gz, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	default:
		return fmt.Errorf("unknown archive type: %s", archive)
	}
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(archive), err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if err := fn(clean(h.Name), tr); err != nil {
			return err
		}
	}
}

// unpackTypst takes the executable, the license and the notice from a Typst release into dir.
func unpackTypst(archive string, t target, dir string) error {
	found := 0
	err := eachFile(archive, func(name string, r io.Reader) error {
		switch path.Base(name) {
		case t.typstName():
			found++
			return writeFile(filepath.Join(dir, t.typstName()), r, 0o755)
		case "LICENSE", "NOTICE":
			return writeFile(filepath.Join(dir, path.Base(name)), r, 0o644)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if found != 1 {
		return fmt.Errorf("%s holds %d files named %s, expected one", filepath.Base(archive), found, t.typstName())
	}
	for _, name := range []string{"LICENSE", "NOTICE"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("%s holds no %s", filepath.Base(archive), name)
		}
	}
	return nil
}

// unpackFonts copies the font files that match the family's pattern into dir, without their directories, and removes
// the hinting of TrueType fonts (see dehint). Typst searches the font directory recursively, so the layout does not
// matter to it, but a flat one is easy to check.
func unpackFonts(archive string, f fontPin, dir string) error {
	pattern := globRegexp(f.Files)
	seen := map[string]bool{}
	err := eachFile(archive, func(name string, r io.Reader) error {
		if !pattern.MatchString("/" + name) {
			return nil
		}
		base := path.Base(name)
		if seen[base] {
			return fmt.Errorf("%s holds two fonts named %s", filepath.Base(archive), base)
		}
		seen[base] = true
		font, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if font, err = dehint(font); err != nil {
			return fmt.Errorf("%s: removing the hinting: %w", base, err)
		}
		return writeFile(filepath.Join(dir, base), bytes.NewReader(font), 0o644)
	})
	if err != nil {
		return err
	}
	if len(seen) == 0 {
		// A new major version has a new family name (Source Serif 5, say): adjust files in tools/versions.json and fonts in
		// internal/payload/template.typ.
		return fmt.Errorf("no fonts matching %s in %s", f.Files, filepath.Base(archive))
	}
	return nil
}

// unpackFontFile copies a font pinned by path into dir, without its hinting, after checking that it is the version
// the pin names.
func unpackFontFile(file string, f fontPin, dir string) error {
	font, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	base := path.Base(f.Path)
	if got, err := fontVersionOf(font); err != nil {
		return fmt.Errorf("%s: %w", base, err)
	} else if got != f.Version {
		return fmt.Errorf("%s is version %s, %s pins %s", base, got, versionsFile, f.Version)
	}
	if font, err = dehint(font); err != nil {
		return fmt.Errorf("%s: removing the hinting: %w", base, err)
	}
	return writeFile(filepath.Join(dir, base), bytes.NewReader(font), 0o644)
}

// A goLicense is a license file of Go or of a module compiled into setdown.
type goLicense struct {
	title, file, src string
}

// goLicenses lists the license and notice files of Go's standard library and of every module that setdown's command
// imports on any target. go list reads the module cache, so the modules must have been downloaded (go mod download).
func goLicenses(root string) ([]goLicense, error) {
	goroot, err := goOutput(root, nil, "env", "GOROOT")
	if err != nil {
		return nil, err
	}
	out := []goLicense{{"Go, LICENSE", "go-LICENSE", filepath.Join(goroot, "LICENSE")}}
	modules := map[string]string{} // path@version: directory
	for _, t := range targets {
		list, err := goOutput(root, []string{"GOOS=" + t.goos, "GOARCH=" + t.goarch, "CGO_ENABLED=0"}, "list", "-e",
			"-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}\t{{.Dir}}{{end}}{{end}}", "./cmd/setdown")
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(list, "\n") {
			if mod, dir, ok := strings.Cut(strings.TrimSpace(line), "\t"); ok {
				if dir == "" {
					return nil, fmt.Errorf("module %s is not downloaded; run go mod download", mod)
				}
				modules[mod] = dir
			}
		}
	}
	names := make([]string, 0, len(modules))
	for mod := range modules {
		names = append(names, mod)
	}
	sort.Strings(names)
	for _, mod := range names {
		modPath, modVersion, _ := strings.Cut(mod, "@")
		found := false
		for _, name := range []string{"LICENSE", "NOTICE"} {
			src := filepath.Join(modules[mod], name)
			if _, err := os.Stat(src); err != nil {
				continue
			}
			found = found || name == "LICENSE"
			file := "go-" + strings.NewReplacer("/", "_", ".", "_").Replace(modPath) + "-" + name
			out = append(out, goLicense{modPath + " " + modVersion + ", " + name, file, src})
		}
		if !found {
			return nil, fmt.Errorf("module %s has no LICENSE file", mod)
		}
	}
	return out, nil
}

// goOutput runs the go command in root with extra environment variables and returns its trimmed output.
func goOutput(root string, env []string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// globRegexp turns a find -path pattern into a regular expression. As with find, * also matches slashes.
func globRegexp(pattern string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

// unpackPackage unpacks a Typst package archive into dir.
func unpackPackage(archive, dir string) error {
	return eachFile(archive, func(name string, r io.Reader) error {
		return writeFile(filepath.Join(dir, filepath.FromSlash(name)), r, 0o644)
	})
}

func writeFile(file string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func copyFile(src, dst string, mode os.FileMode) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeFile(dst, f, mode)
}

// A fileHash is one file of a payload: its path below the payload's directory, its size and its SHA-256.
type fileHash struct {
	path string
	size int64
	sum  string
}

func (h fileHash) named(p string) fileHash {
	h.path = p
	return h
}

func hashFile(file string) (fileHash, error) {
	f, err := os.Open(file)
	if err != nil {
		return fileHash{}, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return fileHash{}, err
	}
	return fileHash{filepath.Base(file), n, hex.EncodeToString(h.Sum(nil))}, nil
}

// hashTree hashes every file below dir, with slash-separated paths relative to it.
func hashTree(dir string) ([]fileHash, error) {
	var hashes []fileHash
	err := filepath.WalkDir(dir, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		h, err := hashFile(file)
		if err != nil {
			return err
		}
		hashes = append(hashes, h.named(filepath.ToSlash(rel)))
		return nil
	})
	return hashes, err
}

// payloadID is the first 8 hex digits of the SHA-256 over the sorted list of a payload's files, sizes and hashes.
// setdown names its cache directory after it, so a payload that differs in any file gets a directory of its own.
func payloadID(files []fileHash) string {
	sort.Slice(files, func(i, j int) bool { return files[i].path < files[j].path })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\t%d\t%s\n", f.path, f.size, f.sum)
	}
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// checkSignature makes sure a Mach-O binary carries a code signature (an LC_CODE_SIGNATURE load command).
func checkSignature(file string) error {
	const lcCodeSignature = 0x1d
	f, err := macho.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range f.Loads {
		if raw := l.Raw(); len(raw) >= 4 && f.ByteOrder.Uint32(raw) == lcCodeSignature {
			return nil
		}
	}
	return errors.New("no code signature, which Apple Silicon needs to run it")
}

// release is the part of a GitHub release that -bump reads.
type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name   string  `json:"name"`
		URL    string  `json:"browser_download_url"`
		Digest *string `json:"digest"`
	} `json:"assets"`
}

func latestRelease(repo string) (*release, error) {
	data, err := get("https://api.github.com/repos/"+repo+"/releases/latest", "application/vnd.github+json")
	if err != nil {
		return nil, fmt.Errorf("asking GitHub for the latest release of %s (set GITHUB_TOKEN if the rate limit is "+
			"reached): %w", repo, err)
	}
	var r release
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("the latest release of %s: %w", repo, err)
	}
	if r.TagName == "" {
		return nil, fmt.Errorf("the latest release of %s has no tag", repo)
	}
	return &r, nil
}

// runBump pins the latest releases of Typst and the fonts. Where GitHub publishes no digest for an asset (the font
// releases predate them), it downloads the asset and records its SHA-256, so later builds are checked against it.
func runBump(root string) error {
	v, err := readVersions(root)
	if err != nil {
		return err
	}
	downloads := at(root, downloadsDir)
	digestOf := func(name, url string, digest *string) (string, error) {
		if digest != nil && strings.HasPrefix(*digest, "sha256:") {
			return strings.TrimPrefix(*digest, "sha256:"), nil
		}
		return fetchNew(downloads, name, url)
	}

	r, err := latestRelease(v.Typst.Repo)
	if err != nil {
		return err
	}
	pinned := map[string]asset{}
	for _, t := range targets {
		found := false
		for _, a := range r.Assets {
			if a.Name == "typst-"+t.triple+".tar.xz" || a.Name == "typst-"+t.triple+".zip" {
				sum, err := digestOf(a.Name, a.URL, a.Digest)
				if err != nil {
					return err
				}
				pinned[t.String()] = asset{a.URL, sum}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Typst %s has no release asset for %s (typst-%s)", r.TagName, t, t.triple)
		}
	}
	report("Typst", v.Typst.Tag, r.TagName)
	v.Typst.Tag, v.Typst.Assets = r.TagName, pinned

	for i := range v.Fonts {
		f := &v.Fonts[i]
		if f.Path != "" {
			if err := bumpFile(downloads, f); err != nil {
				return err
			}
			continue
		}
		r, err := latestRelease(f.Repo)
		if err != nil {
			return err
		}
		pattern, err := regexp.Compile(f.Asset)
		if err != nil {
			return fmt.Errorf("asset pattern of %s: %w", f.Key, err)
		}
		found := false
		for _, a := range r.Assets {
			if !pattern.MatchString(a.Name) {
				continue
			}
			found = true
			report(f.Name, f.Tag, r.TagName)
			if a.URL == f.URL && r.TagName == f.Tag {
				break // unchanged: keep the recorded checksums
			}
			if f.SHA256, err = digestOf(a.Name, a.URL, a.Digest); err != nil {
				return err
			}
			f.Tag, f.URL = r.TagName, a.URL
			if f.LicenseSHA256, err = fetchNew(downloads, f.Key+"-"+path.Base(f.License), f.licenseURL()); err != nil {
				return err
			}
			break
		}
		if !found {
			return fmt.Errorf("no asset matching /%s/ in %s %s", f.Asset, f.Repo, r.TagName)
		}
	}
	latest, err := latestCmarker(strings.TrimPrefix(v.Typst.Tag, "v"))
	if err != nil {
		return err
	}
	report("cmarker", v.Cmarker.Version, latest)
	if latest != v.Cmarker.Version {
		url := "https://packages.typst.org/preview/cmarker-" + latest + ".tar.gz"
		sum, err := fetchNew(downloads, path.Base(url), url)
		if err != nil {
			return err
		}
		if err := setTemplateImport(root, latest); err != nil {
			return err
		}
		v.Cmarker = cmarkerPin{latest, url, sum}
	}
	if err := writeVersions(root, v); err != nil {
		return err
	}
	log.Printf("wrote %s", versionsFile)
	return nil
}

// bumpFile pins a font file of a repository without releases at the last commit that changed it, records its SHA-256
// and reads its version from the file.
func bumpFile(downloads string, f *fontPin) error {
	api := "https://api.github.com/repos/" + f.Repo + "/commits?per_page=1&path=" + url.QueryEscape(f.Path)
	data, err := get(api, "application/vnd.github+json")
	if err != nil {
		return fmt.Errorf("asking GitHub for the last commit of %s in %s (set GITHUB_TOKEN if the rate limit is "+
			"reached): %w", f.Path, f.Repo, err)
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &commits); err != nil {
		return fmt.Errorf("the commits of %s: %w", f.Path, err)
	}
	if len(commits) == 0 || commits[0].SHA == "" {
		return fmt.Errorf("%s has no commit that changed %s", f.Repo, f.Path)
	}
	if commits[0].SHA == f.Tag {
		report(f.Name, f.Version, f.Version)
		return nil // unchanged: keep the recorded checksums
	}
	f.Tag = commits[0].SHA
	f.URL = f.fileURL()
	if f.SHA256, err = fetchNew(downloads, path.Base(f.URL), f.URL); err != nil {
		return err
	}
	font, err := os.ReadFile(filepath.Join(downloads, path.Base(f.URL)))
	if err != nil {
		return err
	}
	version, err := fontVersionOf(font)
	if err != nil {
		return fmt.Errorf("%s: %w", path.Base(f.Path), err)
	}
	report(f.Name, f.Version, version)
	f.Version = version
	f.LicenseSHA256, err = fetchNew(downloads, f.Key+"-"+path.Base(f.License), f.licenseURL())
	return err
}

// latestCmarker returns the newest cmarker in Typst Universe's index that the given Typst version can compile.
// Universe publishes no checksums, so the bump records the one of the download.
func latestCmarker(typst string) (string, error) {
	data, err := get("https://packages.typst.org/preview/index.json", "")
	if err != nil {
		return "", fmt.Errorf("reading Typst Universe's index: %w", err)
	}
	var index []struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Compiler string `json:"compiler"`
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return "", fmt.Errorf("Typst Universe's index: %w", err)
	}
	latest := ""
	for _, p := range index {
		if p.Name != "cmarker" || (p.Compiler != "" && compareVersions(typst, p.Compiler) < 0) {
			continue
		}
		if latest == "" || compareVersions(p.Version, latest) > 0 {
			latest = p.Version
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no cmarker in Typst Universe's index works with Typst %s", typst)
	}
	return latest, nil
}

// setTemplateImport points the template's #import at another cmarker version.
func setTemplateImport(root, version string) error {
	file := at(root, templateFile)
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`#import "@preview/cmarker:[^"]+"`)
	if !re.Match(data) {
		return errors.New(templateFile + " does not import @preview/cmarker")
	}
	return os.WriteFile(file, re.ReplaceAll(data, []byte(`#import "@preview/cmarker:`+version+`"`)), 0o644)
}

func report(name, old, latest string) {
	if old == latest {
		log.Printf("%s: %s, unchanged", name, old)
	} else {
		log.Printf("%s: %s -> %s", name, old, latest)
	}
}
