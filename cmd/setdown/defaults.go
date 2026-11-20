package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v4"
)

// User defaults come from a config file and from the environment variable SETDOWN_OPTIONS, which wins over it. Both
// rank below the front matter, so a document that names its language keeps it; the command line wins over all. The
// settings that front matter can hold reach the template in one input, defaults, that setting() reads after the
// front matter; the options that exist only on the command line (font-path, style, background, first-background,
// pdf-level, quiet) are merged here.

// configPath returns the config file and whether it has to exist: SETDOWN_CONFIG names another one, or none when it
// is set but empty; otherwise it is setdown/config.yaml in the user's config directory ($XDG_CONFIG_HOME or
// ~/.config, ~/Library/Application Support, %APPDATA%), which may be missing.
func configPath() (string, bool) {
	if v, ok := os.LookupEnv("SETDOWN_CONFIG"); ok {
		return v, v != ""
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", false // no home directory, so no config file either
	}
	return filepath.Join(dir, "setdown", "config.yaml"), false
}

// usageText is the usage with this system's config file in it.
func usageText() string {
	where := "none, since SETDOWN_CONFIG is empty"
	if file, _ := configPath(); file != "" {
		where = "here " + file
	}
	return strings.Replace(usage, "%CONFIG%", where, 1)
}

// A layer is one source of user defaults.
type layer struct {
	source  string   // the config file or SETDOWN_OPTIONS, for messages
	config  bool     // whether it is the config file, whose keys have no leading --
	o       options  // its values, as the command line would give them
	bullets []string // a list of bullets, one per level, which only the config file can give
}

// applyDefaults reads the user's defaults and adds them to the options: the settings into o.defaults, unless the
// command line gives them, and the options of the command line only where it does not give them (font-path adds the
// directories in front of its own). Each source is checked on its own, so that a message names the one at fault.
func (o *options) applyDefaults() error {
	var layers []layer
	if file, required := configPath(); file != "" {
		l, found, err := readConfig(file, required)
		if err != nil {
			return err
		}
		if found {
			layers = append(layers, l)
		}
	}
	l, found, err := readEnvOptions()
	if err != nil {
		return err
	}
	if found {
		layers = append(layers, l)
	}

	defaults := map[string]any{}
	var fontPaths []string
	var style, background, firstBackground, pdfLevel, quiet string
	for _, l := range layers {
		if err := l.o.checkValues(); err != nil {
			msg := err.Error()
			if l.config {
				msg = strings.TrimPrefix(msg, "--")
			}
			return fmt.Errorf("%s: %s", l.source, msg)
		}
		for _, s := range l.o.settings() {
			if *s.value != "" {
				defaults[s.key] = *s.value
			}
		}
		if l.bullets != nil {
			defaults["bullet"] = l.bullets
		}
		fontPaths = append(fontPaths, l.o.fontPaths...)
		if l.o.style != "" {
			style = l.o.style
		}
		if l.o.background != "" {
			background = l.o.background
		}
		if l.o.firstBackground != "" {
			firstBackground = l.o.firstBackground
		}
		if l.o.pdfLevel != "" {
			pdfLevel = l.o.pdfLevel
		}
		if l.o.quiet != "" {
			quiet = l.o.quiet
		}
	}
	for _, s := range o.settings() {
		if *s.value != "" {
			delete(defaults, s.key)
		}
	}
	if len(defaults) > 0 {
		o.defaults = defaults
	}
	if len(fontPaths) > 0 {
		o.fontPaths = append(fontPaths, o.fontPaths...)
	}
	if o.style == "" {
		o.style = style
	}
	if o.background == "" {
		o.background = background
	}
	if o.firstBackground == "" {
		o.firstBackground = firstBackground
	}
	// An e-invoice is a PDF/A-3b, whatever level the defaults name.
	if o.pdfLevel == "" && o.eInvoice == "" {
		o.pdfLevel = pdfLevel
	}
	if o.quiet == "" {
		o.quiet = quiet
	}
	return nil
}

// optionOnlyKeys are the keys of a config file that front matter cannot hold.
var optionOnlyKeys = []string{"font-path", "style", "background", "first-background", "pdf-level", "quiet"}

// readConfig reads a config file: YAML keys with a value each, as in front matter, or a list for font-path and
// bullet. found is false if the file is missing and need not exist.
func readConfig(file string, required bool) (l layer, found bool, err error) {
	l = layer{source: file, config: true}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		if !required {
			return l, false, nil
		}
		return l, false, fmt.Errorf("SETDOWN_CONFIG names no such file: %s", file)
	}
	if err != nil {
		return l, false, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return l, false, fmt.Errorf("%s: %v", file, err)
	}
	if len(doc.Content) == 0 {
		return l, true, nil // empty, or nothing but comments
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return l, false, fmt.Errorf("%s: expected keys with a value each, such as lang: de", file)
	}
	dir := filepath.Dir(file)
	for i := 0; i+1 < len(root.Content); i += 2 {
		if err := l.set(root.Content[i].Value, root.Content[i+1], dir); err != nil {
			return l, false, fmt.Errorf("%s, line %d: %v", file, root.Content[i].Line, err)
		}
	}
	return l, true, nil
}

// set takes one key of a config file. Paths are relative to the file's directory, and ~ stands for the home
// directory, as it would in a shell; a background of none stays none.
func (l *layer) set(key string, v *yaml.Node, dir string) error {
	var values []string
	list := false
	switch {
	case v.Kind == yaml.ScalarNode && v.Tag != "!!null" && v.Value != "":
		// The value as written, so that 1.1 or 0.95 stays as it is rather than becoming a number first.
		values = []string{v.Value}
	case v.Kind == yaml.SequenceNode && (key == "font-path" || key == "bullet"):
		list = true
		for _, item := range v.Content {
			if item.Kind != yaml.ScalarNode || item.Tag == "!!null" || item.Value == "" {
				return fmt.Errorf("%s: each item of the list needs a value", key)
			}
			values = append(values, item.Value)
		}
		if len(values) == 0 {
			return fmt.Errorf("%s needs a value", key)
		}
	case v.Kind == yaml.SequenceNode:
		return fmt.Errorf("%s takes one value, not a list", key)
	default:
		return fmt.Errorf("%s needs a value", key)
	}
	path := func(p string) string {
		if p = expandHome(p); !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return p
	}
	switch key {
	case "font-path":
		for _, p := range values {
			l.o.fontPaths = append(l.o.fontPaths, path(p))
		}
		return nil
	case "style":
		l.o.style = path(values[0])
		return nil
	case "background", "first-background":
		v := values[0]
		if !noBackground(v) {
			v = path(v)
		}
		if key == "background" {
			l.o.background = v
		} else {
			l.o.firstBackground = v
		}
		return nil
	case "pdf-level":
		l.o.pdfLevel = values[0]
		return nil
	case "quiet":
		l.o.quiet = values[0]
		return nil
	case "bullet":
		if list {
			l.bullets = values
			return nil
		}
	case "title", "author", "date":
		return fmt.Errorf("%s belongs into a document's front matter, not into the config file", key)
	}
	keys := []string{}
	for _, s := range l.o.settings() {
		if s.key == key {
			*s.value = values[0]
			return nil
		}
		keys = append(keys, s.key)
	}
	return fmt.Errorf("unknown key %s; the config file takes %s", key, strings.Join(append(keys, optionOnlyKeys...), ", "))
}

// readEnvOptions reads SETDOWN_OPTIONS: options as on the command line, split at spaces, where quotes keep a value
// with spaces together (--margin "30mm 20mm"). found is false if it is unset or empty.
func readEnvOptions() (l layer, found bool, err error) {
	l = layer{source: "SETDOWN_OPTIONS"}
	args, err := splitOptions(os.Getenv("SETDOWN_OPTIONS"))
	if err != nil {
		return l, false, fmt.Errorf("SETDOWN_OPTIONS: %v", err)
	}
	if len(args) == 0 {
		return l, false, nil
	}
	o, err := parseArgs(args)
	if errors.Is(err, errHelp) {
		err = errors.New("--help is no default")
	}
	if err == nil {
		switch {
		case o.input != "":
			err = fmt.Errorf("holds options only, not files: %s (a value with spaces goes in quotes)", o.input)
		case o.showVersion, o.licenses, o.cleanCache, o.eInvoice != "":
			err = errors.New("--version, --licenses, --clean-cache and --invoice are no defaults")
		}
	}
	if err != nil {
		return l, false, fmt.Errorf("SETDOWN_OPTIONS: %v", err)
	}
	// No shell expands a ~ in the variable's value, so setdown does.
	for i, p := range o.fontPaths {
		o.fontPaths[i] = expandHome(p)
	}
	o.style = expandHome(o.style)
	o.background, o.firstBackground = expandHome(o.background), expandHome(o.firstBackground)
	l.o = o
	return l, true, nil
}

// splitOptions splits a line of options at white space. Single or double quotes keep what is between them together,
// as in a shell, but without escapes.
func splitOptions(s string) ([]string, error) {
	var args []string
	var arg strings.Builder
	inArg := false
	var quote rune
	for _, r := range s {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				arg.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case unicode.IsSpace(r):
			if inArg {
				args = append(args, arg.String())
				arg.Reset()
				inArg = false
			}
		default:
			arg.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("a %c quote is not closed", quote)
	}
	if inArg {
		args = append(args, arg.String())
	}
	return args, nil
}

// expandHome replaces a leading ~ with the home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}
