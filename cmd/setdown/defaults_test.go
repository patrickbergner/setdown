package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitOptions(t *testing.T) {
	for in, want := range map[string][]string{
		"":                              nil,
		"  \t ":                         nil,
		"--sans --lang de":              {"--sans", "--lang", "de"},
		` --margin "30mm 20mm"  -l de `: {"--margin", "30mm 20mm", "-l", "de"},
		`--page-numbers='Page 1 of 5'`:  {"--page-numbers=Page 1 of 5"},
		`--bullet "" x`:                 {"--bullet", "", "x"},
		`--font "it's" --style 'a"b'`:   {"--font", "it's", "--style", `a"b`},
		"--font-path\n~/fonts":          {"--font-path", "~/fonts"},
	} {
		got, err := splitOptions(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("splitOptions(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := splitOptions(`--font "Inter`); err == nil || !strings.Contains(err.Error(), "not closed") {
		t.Errorf("an open quote: %v", err)
	}
}

// writeConfig writes a config file into a directory of its own and points SETDOWN_CONFIG at it.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SETDOWN_CONFIG", file)
	return file
}

// TestApplyDefaults merges a config file, SETDOWN_OPTIONS and the command line: the environment wins over the file,
// the command line over both, font paths add up, and paths in the file are relative to it.
func TestApplyDefaults(t *testing.T) {
	// The directories and the style file must exist: each source is checked on its own.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	envFonts := filepath.Join(home, "env-fonts")
	for _, dir := range []string{filepath.Join(home, "more"), envFonts} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := writeConfig(t, `# my defaults
lang: de
font: sans
bullet: [•, ◦]
scale: 0.95
heading-numbering: 1.1
toc: 2
landscape: yes
font-path: [fonts, ~/more]
style: styles/corp.typ
background: letterhead.pdf
first-background: none
first-margin-top: 60mm
pdf-level: ua1
quiet: on
`)
	dir := filepath.Dir(file)
	for _, d := range []string{"fonts", "styles"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(dir, "styles", "corp.typ"), filepath.Join(dir, "letterhead.pdf"),
		filepath.Join(home, "first.png")} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SETDOWN_OPTIONS", `--lang en-GB --margin "30mm 20mm" --font-path='`+envFonts+`' --no-toc `+
		`--first-background ~/first.png`)
	o, err := parseArgs([]string{"--serif", "--font-path", "cmd", "--portrait", "in.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := o.applyDefaults(); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"lang": "en-GB", "bullet": []string{"•", "◦"}, "scale": "0.95", "margin": "30mm 20mm",
		"first-margin-top": "60mm", "heading-numbering": "1.1", "toc": "false"}
	if !reflect.DeepEqual(o.defaults, want) {
		t.Errorf("defaults %v, want %v", o.defaults, want)
	}
	if wantPaths := []string{filepath.Join(dir, "fonts"), filepath.Join(home, "more"), envFonts, "cmd"}; !reflect.DeepEqual(o.fontPaths, wantPaths) {
		t.Errorf("font paths %q, want %q", o.fontPaths, wantPaths)
	}
	if o.font != "serif" || o.landscape != "false" || o.style != filepath.Join(dir, "styles", "corp.typ") ||
		o.pdfLevel != "ua1" || o.quiet != "on" {
		t.Errorf("font %q, landscape %q, style %q, pdf-level %q, quiet %q", o.font, o.landscape, o.style, o.pdfLevel,
			o.quiet)
	}
	// Paths relative to the config file, ~ expanded in SETDOWN_OPTIONS, which wins over the file's none.
	if o.background != filepath.Join(dir, "letterhead.pdf") || o.firstBackground != filepath.Join(home, "first.png") {
		t.Errorf("background %q, first-background %q", o.background, o.firstBackground)
	}
	args := typstArgs(&o, "ROOT", "FONTS", "PACKAGES")
	wantArg := `defaults={"bullet":["•","◦"],"first-margin-top":"60mm","heading-numbering":"1.1","lang":"en-GB",` +
		`"margin":"30mm 20mm","scale":"0.95","toc":"false"}`
	if !strings.Contains(strings.Join(args, "\n"), "\n"+wantArg+"\n") {
		t.Errorf("typstArgs lacks --input %s:\n%q", wantArg, args)
	}

	// The command line's own values stay, an e-invoice keeps its PDF/A-3b, and without defaults nothing changes.
	o, err = parseArgs([]string{"--style", "mine.typ", "--no-quiet", "--invoice", "x.xml", "in.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := o.applyDefaults(); err != nil {
		t.Fatal(err)
	}
	if o.style != "mine.typ" || o.quiet != "false" || o.pdfLevel != "" {
		t.Errorf("style %q, quiet %q, pdf-level %q", o.style, o.quiet, o.pdfLevel)
	}
	// none in the config file stays none, not a file next to it.
	t.Setenv("SETDOWN_OPTIONS", "")
	writeConfig(t, "first-background: none\nbackground: NONE\n")
	o, _ = parseArgs([]string{"in.md"})
	if err := o.applyDefaults(); err != nil {
		t.Fatal(err)
	}
	if o.firstBackground != "none" || o.background != "NONE" {
		t.Errorf("background %q, first-background %q, want none", o.background, o.firstBackground)
	}
	t.Setenv("SETDOWN_CONFIG", "")
	t.Setenv("SETDOWN_OPTIONS", "")
	o, _ = parseArgs([]string{"in.md"})
	if err := o.applyDefaults(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o, options{input: "in.md"}) {
		t.Errorf("without defaults: %+v", o)
	}
	if args := typstArgs(&o, "ROOT", "FONTS", "PACKAGES"); strings.Contains(strings.Join(args, " "), "defaults=") {
		t.Errorf("without defaults: %q", args)
	}
}

func TestDefaultsErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, tt := range []struct{ config, options, err string }{
		{"lang: [de, en]\n", "", "line 1: lang takes one value, not a list"},
		{"paper: a4\nfrobnicate: 1\n", "", "line 2: unknown key frobnicate; the config file takes font, mono-font,"},
		{"title: My Notes\n", "", "line 1: title belongs into a document's front matter"},
		{"lang:\n", "", "line 1: lang needs a value"},
		{"font-path: [a, b]\nbullet: []\n", "", "line 2: bullet needs a value"},
		{"font-path: [a, ~]\n", "", "line 1: font-path: each item of the list needs a value"},
		{"- lang\n", "", "expected keys with a value each, such as lang: de"},
		{"lang: [de\n", "", "yaml:"},
		{"scale: big\n", "", ": scale must be a factor such as 0.9"},
		{"highlight: maybe\n", "", ": highlight must be true or false, got: maybe"},
		{"toc: 9\n", "", ": toc must be true, false or the number of heading levels"},
		{"font-path: " + missing + "\n", "", ": font-path: no such directory: "},
		{"background: " + missing + "\n", "", ": background: no such file: "},
		{"first-margin-top: 1em\n", "", ": first-margin-top must be a length such as 60mm"},
		{"", "--first-background " + missing, "SETDOWN_OPTIONS: --first-background: no such file: "},
		{"", "--scale big", "SETDOWN_OPTIONS: --scale must be a factor"},
		{"", "--sans notes.md", "SETDOWN_OPTIONS: holds options only, not files: notes.md"},
		{"", "--version", "SETDOWN_OPTIONS: --version, --licenses, --clean-cache and --invoice are no defaults"},
		{"", "--invoice x.xml", "are no defaults"},
		{"", "-h", "SETDOWN_OPTIONS: --help is no default"},
		{"", "--frobnicate", "SETDOWN_OPTIONS: unknown option: --frobnicate"},
		{"", `--font "Inter`, "SETDOWN_OPTIONS: a \" quote is not closed"},
	} {
		file := writeConfig(t, tt.config)
		t.Setenv("SETDOWN_OPTIONS", tt.options)
		o := options{input: "in.md"}
		err := o.applyDefaults()
		if err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("config %q, SETDOWN_OPTIONS %q: %v, want %q", tt.config, tt.options, err, tt.err)
		} else if tt.config != "" && !strings.HasPrefix(err.Error(), file) {
			t.Errorf("config %q: %v does not name the file", tt.config, err)
		}
	}
	t.Setenv("SETDOWN_OPTIONS", "")
	t.Setenv("SETDOWN_CONFIG", missing)
	o := options{input: "in.md"}
	if err := o.applyDefaults(); err == nil || !strings.Contains(err.Error(), "SETDOWN_CONFIG names no such file") {
		t.Errorf("a missing SETDOWN_CONFIG: %v", err)
	}
	// The default config file need not exist. Each system has its own place for it.
	empty := t.TempDir()
	for _, v := range []string{"XDG_CONFIG_HOME", "APPDATA", "HOME"} {
		t.Setenv(v, empty)
	}
	os.Unsetenv("SETDOWN_CONFIG")
	if file, required := configPath(); required || !strings.HasPrefix(file, empty) ||
		!strings.HasSuffix(file, filepath.Join("setdown", "config.yaml")) {
		t.Errorf("configPath() = %s, %v", file, required)
	}
	if err := o.applyDefaults(); err != nil {
		t.Errorf("no config file: %v", err)
	}
}

// TestDefaultsConvert converts with defaults from each source: the front matter wins over them, the command line
// over everything.
func TestDefaultsConvert(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "example.md")
	example, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}
	fm := filepath.Join(work, "serif.md")
	if err := os.WriteFile(fm, bytes.Replace(example, []byte("---\n"), []byte("---\nfont: serif\n"), 1),
		0o644); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, "font: sans\npaper: a5\nscale: 90%\n")
	for _, tt := range []struct {
		options string
		args    []string
		family  string
		width   float64
		size    float64
	}{
		{"", []string{in}, "SourceSans3", a5Width, 9.9},
		{"", []string{fm}, "SourceSerif4", a5Width, 9.9},
		{"--paper a4 --scale=1", []string{in}, "SourceSans3", a4Width, 11},
		{"--paper a4", []string{"--paper", "a5", "--serif", in}, "SourceSerif4", a5Width, 9.9},
	} {
		t.Setenv("SETDOWN_OPTIONS", tt.options)
		out := filepath.Join(work, "out.pdf")
		var stdout, stderr bytes.Buffer
		if code, sig := run(append(tt.args, out), &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("run(%q): exit %d, signal %v\n%s", tt.args, code, sig, stderr.String())
		}
		p, err := readPDF(out)
		if err != nil {
			t.Fatal(err)
		}
		family := "" // of the body text, the font besides JetBrains Mono
		for _, f := range p.fonts {
			if name, _, _ := strings.Cut(f, "-"); name != jbMono {
				family = name
			}
		}
		if family != tt.family || p.width < tt.width-0.5 || p.width > tt.width+0.5 || p.bodySize < tt.size-0.01 ||
			p.bodySize > tt.size+0.01 {
			t.Errorf("SETDOWN_OPTIONS %q, %q: %s, %.2f pt wide, body %.2f pt; want %s, %.2f, %.2f", tt.options,
				tt.args, family, p.width, p.bodySize, tt.family, tt.width, tt.size)
		}
	}

	// A config file that does not parse stops a conversion, but not --version.
	writeConfig(t, "lang: [de\n")
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{in}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "setdown: ") {
		t.Errorf("a broken config file: exit %d, stderr %q", code, stderr.String())
	}
	if code, _ := run([]string{"--version"}, &stdout, &stderr); code != 0 {
		t.Errorf("--version with a broken config file: exit %d", code)
	}
}

// TestQuiet leaves out the report and the note about unpacking, which a first run on Windows and macOS makes, but
// not an error.
func TestQuiet(t *testing.T) {
	work := sandbox(t)
	in := filepath.Join(work, "example.md")
	for _, tt := range []struct {
		config string
		args   []string
		quiet  bool
	}{
		{"", []string{"-q"}, true},
		{"", []string{"--quiet", "--no-quiet"}, false},
		{"quiet: true\n", nil, true},
		{"quiet: true\n", []string{"--no-quiet"}, false},
	} {
		writeConfig(t, tt.config)
		var stdout, stderr bytes.Buffer
		if code, sig := run(append(tt.args, in), &stdout, &stderr); code != 0 || sig != nil {
			t.Fatalf("run(%q): exit %d, signal %v\n%s", tt.args, code, sig, stderr.String())
		}
		if got := stderr.String(); (got == "") != tt.quiet || stdout.Len() > 0 {
			t.Errorf("config %q, %q: stdout %q, stderr %q", tt.config, tt.args, stdout.String(), got)
		}
	}
	var stdout, stderr bytes.Buffer
	if code, _ := run([]string{"-q", "--paper", "no-such-paper", in}, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "error:") {
		t.Errorf("a failing run with -q: exit %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code, _ := run([]string{"-q", "--scale", "big", in}, &stdout, &stderr); code != 1 ||
		!strings.HasPrefix(stderr.String(), "setdown: --scale must be") {
		t.Errorf("a refused option with -q: exit %d, stderr %q", code, stderr.String())
	}
}
