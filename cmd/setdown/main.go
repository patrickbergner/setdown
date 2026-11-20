// Command setdown converts a Markdown file to a typographically clean PDF. It carries a pinned Typst release, the
// cmarker package, its fonts and its template (internal/payload), puts them where Typst can use them
// (internal/launch) and runs the embedded Typst as a child process, so it needs nothing installed and no network.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/patrickbergner/setdown/internal/launch"
	"github.com/patrickbergner/setdown/internal/payload"
	"github.com/patrickbergner/setdown/internal/pdfpack"
)

// version is set from the VERSION file at build time (see build.sh). The cache directories on Windows and macOS carry
// it in their names.
var version = "dev"

const usage = `Usage: setdown [options] <input.md> [output.pdf]

Converts Markdown (CommonMark with tables, footnotes, strikethrough) to PDF.
The output defaults to the input's path with .pdf. Images are resolved from
the input's directory and may not lie outside it.

Options:
  -s, --sans            set the text in Source Sans 3 instead of Source Serif 4
      --serif           set the text in Source Serif 4 (the default)
      --font <family>   set the text in another font family, e.g. Inter
      --mono-font <family>
                        set code in another font family (default JetBrains Mono)
      --font-path <dir> also take fonts from this directory; may be given more
                        than once
      --style <file>    a Typst file whose set and show rules apply after the
                        template's; its paths are relative to the Markdown file
      --scale <factor>  scale the text, e.g. 0.9 or 90% (default 1, i.e. 11pt)
  -l, --lang <code>     language for hyphenation and quotes, e.g. de (default en)
  -p, --paper <name>    Typst paper name, e.g. a4, a5, us-letter (default a4)
      --landscape       turn the paper sideways (--portrait: upright, the default)
  -m, --margin <length> page margin, e.g. 2cm or 20mm (default 20mm), or two to
                        four for the sides as in CSS: "30mm 20mm" for top and
                        bottom, then left and right; "30mm 20mm 25mm 15mm" for
                        top, right, bottom and left
      --first-margin-top <length>
                        the first page's top margin, e.g. 60mm to clear a
                        letterhead (default: as on the other pages)
      --background <file>
                        a PDF, SVG, PNG, JPEG, GIF or WebP file drawn behind every
                        page, at the page's width from its top; none for none
      --first-background <file>
                        the same for the first page, instead of --background
      --highlight       color code blocks by language (--no-highlight: off, the default)
      --bullet <marker> the marker of bulleted lists, e.g. • (default –)
      --numbering <pattern>
                        how numbered lists count, e.g. 1) or a) or 1.a.i. (default 1.)
      --heading-numbering <pattern>
                        number the headings, e.g. 1.1 or I.a (default none)
      --toc             a table of contents below the title, of heading levels 1
                        to 3; --toc=2 for two levels (--no-toc: none, the default)
      --page-numbers <format>
                        page numbers by example, e.g. 1, 1 of 5, "Page 1 of 5" or none
                        (default 1 / 5); the first number is the page, a second the count
      --pdf-level <level>
                        default: a small PDF 1.7, without tags for screen readers;
                        a3b: PDF/A-3b, for archiving and e-invoices (with the XML attached);
                        ua1: PDF/UA-1, accessible; both tagged and with XMP metadata
      --invoice <file.xml>
                        make a Factur-X/ZUGFeRD e-invoice: a PDF/A-3b with this
                        Cross Industry Invoice XML attached (profile BASIC, EN 16931,
                        EXTENDED or XRECHNUNG, read from the XML)
  -q, --quiet           say nothing when all goes well (--no-quiet: name the PDF written)
      --version         show the versions of setdown, Typst, the fonts and cmarker
      --licenses        show the license texts of setdown and of what it bundles
      --clean-cache     remove the files setdown unpacked, e.g. before uninstalling
  -h, --help            show this help

Options with a value also take the form --option=value. Settings can also come
from YAML front matter (title, author, date, lang, font: sans, mono-font, scale,
paper, landscape: true, margin, first-margin-top, highlight: true, bullet,
numbering, heading-numbering, toc: true, page-numbers); options on the command
line win. Defaults of your own go into a config file with the same keys (and
font-path, style, background, first-background, pdf-level, quiet: true),
%CONFIG%,
or into the environment variable SETDOWN_OPTIONS, as options such as
SETDOWN_OPTIONS='--sans --lang de'; the front matter wins over both.
A set rule in a comment changes list markers from there on, e.g.
<!--raw-typst #set enum(numbering: "a)")--> in front of a list.
`

// options holds the command line. A value of "" means not given; switches hold "true" or "false" once given, so that
// one can turn off what a user default turned on.
type options struct {
	font, monoFont, scale, lang, paper, landscape, margin, firstMarginTop string
	highlight, bullet, numbering, headingNumbering, toc, pageNumbers      string
	pdfLevel                                                              string
	fontPaths                                                             []string
	style                                                                 string
	background, firstBackground                                           string // files, or none
	quiet                                                                 string
	input, output                                                         string
	showVersion, licenses, cleanCache                                     bool
	styleCode                                                             []byte      // the --style file's content, read by check
	firstBg, restBg                                                       *background // the backgrounds, read by check
	eInvoice                                                              string
	invoice                                                               *eInvoice      // the --invoice file, read by check
	defaults                                                              map[string]any // see applyDefaults
}

// A setting is an option that front matter can give as well, under its key.
type setting struct {
	key   string
	value *string
}

// settings lists the options that front matter can give as well, in the order typstArgs passes them to Typst.
func (o *options) settings() []setting {
	return []setting{
		{"font", &o.font}, {"mono-font", &o.monoFont}, {"scale", &o.scale}, {"lang", &o.lang}, {"paper", &o.paper},
		{"landscape", &o.landscape}, {"margin", &o.margin}, {"first-margin-top", &o.firstMarginTop},
		{"highlight", &o.highlight}, {"bullet", &o.bullet},
		{"numbering", &o.numbering}, {"heading-numbering", &o.headingNumbering}, {"toc", &o.toc},
		{"page-numbers", &o.pageNumbers},
	}
}

var errHelp = errors.New("help requested")

func main() {
	code, sig := run(os.Args[1:], os.Stdout, os.Stderr)
	if sig != nil {
		exitBySignal(sig)
	}
	os.Exit(code)
}

// run is setdown without the process around it. It returns the exit code and, if a signal ended Typst or interrupted
// setdown, that signal, which setdown should then end by.
func run(args []string, stdout, stderr io.Writer) (int, os.Signal) {
	o, err := parseArgs(args)
	if errors.Is(err, errHelp) {
		fmt.Fprint(stdout, usageText())
		return 0, nil
	}
	if err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	if o.showVersion {
		return printVersion(stdout, stderr), nil
	}
	if o.licenses {
		return printLicenses(stdout, stderr), nil
	}
	if o.cleanCache {
		if err := launch.CleanCache(func(format string, args ...any) { notef(stderr, format, args...) }); err != nil {
			notef(stderr, "%v", err)
			return 1, nil
		}
		if o.input == "" {
			return 0, nil
		}
	}
	if o.input == "" {
		fmt.Fprint(stderr, usageText())
		return 1, nil
	}
	// The user's defaults matter only for a conversion, so a broken config file does not stand in the way of --help,
	// --version or --clean-cache.
	if err := o.applyDefaults(); err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	if err := o.check(); err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	return convert(&o, stdout, stderr)
}

// parseArgs reads the command line by hand: options anywhere, -- ends them, and a value follows its option (or, for
// long options, comes after =). Go's flag package would stop at the first file name and has no short and long forms
// of one option.
func parseArgs(args []string) (options, error) {
	var o options
	positional := func(a string) error {
		switch {
		case o.input == "":
			o.input = a
		case o.output == "":
			o.output = a
		default:
			return errors.New("too many arguments")
		}
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			for _, rest := range args[i+1:] {
				if err := positional(rest); err != nil {
					return o, err
				}
			}
			break
		}
		if !strings.HasPrefix(a, "-") {
			if err := positional(a); err != nil {
				return o, err
			}
			continue
		}
		name, value, inline := a, "", false
		if strings.HasPrefix(a, "--") {
			name, value, inline = strings.Cut(a, "=")
		}
		if name == "--toc" {
			// The one option whose value is optional, so it comes only after an equals sign: --toc=2.
			o.toc = "true"
			if inline {
				if value == "" {
					return o, errors.New("--toc= needs a value, such as --toc=2")
				}
				o.toc = value
			}
			continue
		}
		if set := o.valueOf(name); set != nil {
			if !inline && i+1 < len(args) {
				i++
				value = args[i]
			}
			if value == "" {
				return o, fmt.Errorf("%s needs a value", name)
			}
			set(value)
			continue
		}
		help := false
		switch name {
		case "-s", "--sans":
			o.font = "sans"
		case "--serif":
			o.font = "serif"
		case "--highlight":
			o.highlight = "true"
		case "--no-highlight":
			o.highlight = "false"
		case "--landscape":
			o.landscape = "true"
		case "--portrait":
			o.landscape = "false"
		case "--no-toc":
			o.toc = "false"
		case "-q", "--quiet":
			o.quiet = "true"
		case "--no-quiet":
			o.quiet = "false"
		case "--version":
			o.showVersion = true
		case "--licenses":
			o.licenses = true
		case "--clean-cache":
			o.cleanCache = true
		case "-h", "--help":
			help = true
		default:
			return o, fmt.Errorf("unknown option: %s (see --help)", a)
		}
		if inline {
			return o, fmt.Errorf("%s takes no value", name)
		}
		if help {
			return o, errHelp
		}
	}
	return o, nil
}

// valueOf returns what takes the value of an option, or nil if the option takes none. A repeated option replaces
// the value before it, except --font-path, which adds a directory each time.
func (o *options) valueOf(name string) func(string) {
	var dest *string
	switch name {
	case "--font":
		dest = &o.font
	case "--mono-font":
		dest = &o.monoFont
	case "--font-path":
		return func(v string) { o.fontPaths = append(o.fontPaths, v) }
	case "--style":
		dest = &o.style
	case "--scale":
		dest = &o.scale
	case "-l", "--lang":
		dest = &o.lang
	case "-p", "--paper":
		dest = &o.paper
	case "-m", "--margin":
		dest = &o.margin
	case "--first-margin-top":
		dest = &o.firstMarginTop
	case "--background":
		dest = &o.background
	case "--first-background":
		dest = &o.firstBackground
	case "--bullet":
		dest = &o.bullet
	case "--numbering":
		dest = &o.numbering
	case "--heading-numbering":
		dest = &o.headingNumbering
	case "--page-numbers":
		dest = &o.pageNumbers
	case "--pdf-level":
		dest = &o.pdfLevel
	case "--invoice":
		dest = &o.eInvoice
	default:
		return nil
	}
	return func(v string) { *dest = v }
}

// pdfLevels are the values of --pdf-level and the Typst arguments for each. The default leaves out the tags, which
// screen readers need and which make up a quarter of a typical PDF; pdfpack leaves out the XMP metadata as well. The
// two standards need both, and Typst checks the document against them. All three end up in object streams, which
// PDF/A-1 and PDF 1.4 would not allow.
var pdfLevels = map[string][]string{
	"default": {"--no-pdf-tags"},
	"a3b":     {"--pdf-standard", "a-3b"},
	"ua1":     {"--pdf-standard", "ua-1"},
}

// level returns the --pdf-level, "default" when none is given.
func (o *options) level() string {
	if o.pdfLevel == "" {
		return "default"
	}
	return o.pdfLevel
}

var (
	scalePattern  = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?%?$`)
	lengthPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?(mm|cm|in|pt)$`)
	numberPattern = regexp.MustCompile(`[0-9]+`)
	tocPattern    = regexp.MustCompile(`^[1-6]$`)
)

// check validates the input and the values Typst would otherwise reject with a less helpful message, and derives the
// output's name.
func (o *options) check() error {
	in, err := os.Stat(o.input)
	if err != nil || !in.Mode().IsRegular() {
		return fmt.Errorf("no such file: %s", o.input)
	}
	if o.output == "" {
		o.output = strings.TrimSuffix(o.input, filepath.Ext(o.input)) + ".pdf"
	}
	if out, err := os.Stat(o.output); err == nil && os.SameFile(in, out) {
		return fmt.Errorf("the output would overwrite the input: %s", o.output)
	}
	if err := o.checkValues(); err != nil {
		return err
	}
	if o.eInvoice != "" {
		if o.pdfLevel != "" && o.pdfLevel != "a3b" {
			return fmt.Errorf("--invoice makes a PDF/A-3b and takes no --pdf-level %s", o.pdfLevel)
		}
		inv, err := readEInvoice(o.eInvoice)
		if err != nil {
			return fmt.Errorf("--invoice: %v", err)
		}
		o.invoice, o.pdfLevel = inv, "a3b"
	}
	if o.style != "" {
		var err error
		if o.styleCode, err = os.ReadFile(o.style); err != nil {
			return fmt.Errorf("--style: %v", err)
		}
		if !utf8.Valid(o.styleCode) {
			return fmt.Errorf("--style: not UTF-8 text, which Typst needs: %s", o.style)
		}
		o.styleCode = bytes.TrimPrefix(o.styleCode, []byte("\uFEFF"))
		if n := len(o.styleCode); n > 0 && o.styleCode[n-1] != '\n' {
			o.styleCode = append(o.styleCode, '\n')
		}
	}
	return o.readBackgrounds()
}

// checkValues checks the values of options, which may also be user defaults, without reading any file.
func (o *options) checkValues() error {
	if o.scale != "" && !scalePattern.MatchString(o.scale) {
		return fmt.Errorf("--scale must be a factor such as 0.9 or a percentage such as 90%%, got: %s", o.scale)
	}
	if o.margin != "" && !validMargin(o.margin) {
		return fmt.Errorf("--margin must be a length such as 2cm or 20mm, or two to four for the sides as in CSS, "+
			"such as \"30mm 20mm\", got: %s", o.margin)
	}
	if o.firstMarginTop != "" && !lengthPattern.MatchString(o.firstMarginTop) {
		return fmt.Errorf("--first-margin-top must be a length such as 60mm, got: %s", o.firstMarginTop)
	}
	if o.pageNumbers != "" && !pageNumbersOff(o.pageNumbers) {
		if n := len(numberPattern.FindAllString(o.pageNumbers, -1)); n < 1 || n > 2 {
			return fmt.Errorf("--page-numbers must be none or show a number for the page, and maybe one for the "+
				"number of pages, as in \"1 / 5\" or \"Page 1 of 5\", got: %s", o.pageNumbers)
		}
	}
	if o.pdfLevel != "" {
		if _, ok := pdfLevels[o.pdfLevel]; !ok {
			return fmt.Errorf("--pdf-level must be default, a3b or ua1, got: %s", o.pdfLevel)
		}
	}
	if o.toc != "" && !isBool(o.toc) && !tocPattern.MatchString(o.toc) {
		return fmt.Errorf("--toc must be true, false or the number of heading levels from 1 to 6, got: %s", o.toc)
	}
	for _, s := range []setting{{"--highlight", &o.highlight}, {"--landscape", &o.landscape}, {"--quiet", &o.quiet}} {
		if *s.value != "" && !isBool(*s.value) {
			return fmt.Errorf("%s must be true or false, got: %s", s.key, *s.value)
		}
	}
	for _, dir := range o.fontPaths {
		// Typst would skip a missing directory without a word and fall back to another font.
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			return fmt.Errorf("--font-path: no such directory: %s", dir)
		}
	}
	if o.style != "" {
		if fi, err := os.Stat(o.style); err != nil || !fi.Mode().IsRegular() {
			return fmt.Errorf("--style: no such file: %s", o.style)
		}
	}
	for _, s := range []setting{{"--background", &o.background}, {"--first-background", &o.firstBackground}} {
		if *s.value != "" && !noBackground(*s.value) {
			if fi, err := os.Stat(*s.value); err != nil || !fi.Mode().IsRegular() {
				return fmt.Errorf("%s: no such file: %s", s.key, *s.value)
			}
		}
	}
	return nil
}

// isBool reports whether a value is one of the words the template takes for true and false.
func isBool(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off":
		return true
	}
	return false
}

// isTrue reports whether a value checked by isBool means true.
func isTrue(v string) bool {
	switch strings.ToLower(v) {
	case "true", "yes", "on":
		return true
	}
	return false
}

// styleMarker is the line of the template that a --style file replaces.
var styleMarker = []byte("// setdown: --style\n")

// templateWith returns the template with the style code (lines ending in a newline) in place of its marker, and the
// line of the result at which the style code starts, 0 without style code.
func templateWith(style []byte) ([]byte, int) {
	if style == nil {
		return payload.Template, 0
	}
	before, after, ok := bytes.Cut(payload.Template, styleMarker)
	if !ok {
		panic("the template lacks the line " + strings.TrimSpace(string(styleMarker)))
	}
	src := make([]byte, 0, len(before)+len(style)+len(after))
	src = append(append(append(src, before...), style...), after...)
	return src, bytes.Count(before, []byte("\n")) + 1
}

// validMargin reports whether a --margin value is one to four lengths, as the template reads it.
func validMargin(v string) bool {
	parts := strings.Fields(v)
	for _, p := range parts {
		if !lengthPattern.MatchString(p) {
			return false
		}
	}
	return len(parts) >= 1 && len(parts) <= 4
}

// pageNumbersOff reports whether a --page-numbers value turns the numbers off, as the template reads it.
func pageNumbersOff(v string) bool {
	switch strings.ToLower(v) {
	case "none", "false", "off", "no":
		return true
	}
	return false
}

// convert runs Typst on the input with the template on stdin and reports what it wrote.
func convert(o *options, stdout, stderr io.Writer) (int, os.Signal) {
	p, err := payload.Embedded()
	if err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	sigs := catchSignals()
	defer sigs.stop()
	env, err := launch.Prepare(launch.Run{
		Version: version,
		// Only says what setdown is doing (unpacking the cache), which --quiet leaves out.
		Notef: func(format string, args ...any) {
			if !isTrue(o.quiet) {
				notef(stderr, format, args...)
			}
		},
		Stop: sigs.stopRequested,
	}, p)
	if err != nil {
		if sig := sigs.interrupted(); sig != nil {
			return exitCode(sig), sig
		}
		notef(stderr, "%v", err)
		return 1, nil
	}
	defer env.Cleanup()

	root, err := filepath.Abs(filepath.Dir(o.input))
	if err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	if o.level() != "default" {
		// PDF/A and PDF/UA cannot embed a PDF in Typst, so a PDF background becomes an SVG of its first page first.
		for _, b := range o.backgrounds() {
			if b.format != "pdf" {
				continue
			}
			args, err := svgArgs(b.file)
			if err != nil {
				notef(stderr, "%v", err)
				return 1, nil
			}
			var svg bytes.Buffer
			code, sig, err := runTypst(env, sigs, args, []byte(pdfToSVG), &svg, stderr)
			switch {
			case err != nil:
				notef(stderr, "%v", err)
				return 1, nil
			case sig != nil:
				return exitCode(sig), sig
			case code != 0 || svg.Len() == 0:
				notef(stderr, "could not convert %s (%s) to SVG, which %s needs", b.file, o.optionOf(b),
					standardNames[o.level()])
				return max(code, 1), nil
			}
			notef(stderr, "warning: Typst cannot embed a PDF image in a %s, so %s %s is drawn from an SVG of its "+
				"first page, in which text becomes outlines; give an SVG instead to avoid the conversion",
				standardNames[o.level()], o.optionOf(b), b.file)
			b.data, b.format = svg.Bytes(), "svg"
		}
	}

	args := typstArgs(o, root, env.Fonts, env.Packages)
	template, styleLine := templateWith(o.styleCode)
	// One line in place of its marker, before the style code, which so keeps its line numbers.
	template = withBackgrounds(template, o.firstBg, o.restBg)
	if o.invoice != nil {
		// At the end, so that the line numbers of a style file stay as setdown reports them.
		template = append(append([]byte{}, template...), o.invoice.typstLine()...)
	}
	// The template comes in on stdin, so the input's directory can be the root that Markdown paths resolve against.
	code, sig, err := runTypst(env, sigs, args, template, stdout, stderr)
	if err != nil {
		notef(stderr, "%v", err)
		return 1, nil
	}
	if sig != nil {
		return exitCode(sig), sig
	}
	if code != 0 {
		if n := bytes.Count(o.styleCode, []byte("\n")); n > 0 {
			// Typst's messages name the template with the style code <stdin>. They reach the terminal unchanged,
			// so that their colors stay, which leaves rewriting their line numbers out.
			notef(stderr, "in Typst's messages, lines %d to %d of <stdin> are lines 1 to %d of %s",
				styleLine, styleLine+n-1, n, o.style)
		}
		if bgs := o.backgrounds(); len(bgs) > 0 {
			var files []string
			for _, b := range bgs {
				files = append(files, fmt.Sprintf("%s (%s)", b.file, o.optionOf(b)))
			}
			notef(stderr, "in Typst's messages, line %d of <stdin> draws the background %s",
				backgroundCallLine(template), strings.Join(files, " or "))
		}
		return code, nil
	}
	opts := pdfpack.Options{DropMetadata: o.level() == "default"}
	if o.invoice != nil {
		opts.FacturX = o.invoice.facturX()
	}
	pages, err := shrink(o.output, opts)
	if err != nil {
		// The PDF is fine as Typst wrote it, only larger.
		notef(stderr, "could not make %s smaller, it stays as Typst wrote it: %v", o.output, err)
	}
	if !isTrue(o.quiet) {
		report(stderr, o.output, pages)
	}
	// Typst had ended, so a signal since then waited for the PDF to be complete; setdown ends by it now.
	if sig := sigs.interrupted(); sig != nil {
		return exitCode(sig), sig
	}
	return 0, nil
}

// standardNames name the standards of the PDF levels, for messages.
var standardNames = map[string]string{"a3b": "PDF/A-3b", "ua1": "PDF/UA-1"}

// runTypst runs Typst with the arguments and stdin and returns its exit code, or the signal that ended it. Signals
// that reach setdown while it runs go to Typst as well (see signals); one that came before stops setdown instead.
func runTypst(env *launch.Setup, sigs *signals, args []string, stdin []byte, stdout, stderr io.Writer) (int,
	os.Signal, error) {
	if sig := sigs.interrupted(); sig != nil {
		return 0, sig, nil
	}
	cmd, err := env.Start(func(path string) *exec.Cmd {
		return &exec.Cmd{
			Path: path,
			// Typst's messages look as they do when it is run directly.
			Args:   append([]string{"typst"}, args...),
			Stdin:  bytes.NewReader(stdin),
			Stdout: stdout,
			Stderr: stderr,
		}
	})
	if err != nil {
		return 0, nil, err
	}
	sigs.started(cmd.Process)
	err = cmd.Wait()
	sigs.exited()
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return 0, nil, fmt.Errorf("running Typst: %v", err)
		}
		if ws, ok := exit.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 0, ws.Signal(), nil
		}
		return exit.ExitCode(), nil, nil
	}
	return 0, nil, nil
}

// shrink rewrites the PDF Typst wrote with pdfpack and returns its page count. The new file replaces the old one by
// a rename, so the output is never half written, and keeps its permissions.
func shrink(output string, opts pdfpack.Options) (int, error) {
	data, err := os.ReadFile(output)
	if err != nil {
		return 0, err
	}
	packed, res, err := pdfpack.Pack(data, opts)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(output)
	if err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".setdown-*.pdf")
	if err != nil {
		return 0, err
	}
	defer os.Remove(tmp.Name()) // fails once renamed
	_, err = tmp.Write(packed)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), fi.Mode().Perm())
	}
	if err == nil {
		err = os.Rename(tmp.Name(), output)
	}
	if err != nil {
		return 0, err
	}
	return res.Pages, nil
}

// typstArgs returns the arguments of typst compile. The fonts and cmarker come from setdown's own directories, the
// user's fonts from --font-path after them. Typst looks for a package in the --package-path directory before its
// cache, so nothing is downloaded, and it takes no system fonts, so a document comes out the same everywhere.
func typstArgs(o *options, root, fonts, packages string) []string {
	name := filepath.Base(o.input)
	args := []string{"compile", "--root", root, "--font-path", fonts}
	for _, dir := range o.fontPaths {
		args = append(args, "--font-path", dir)
	}
	args = append(args, "--ignore-system-fonts", "--package-path", packages,
		"--input", "md="+name, "--input", "name="+strings.TrimSuffix(name, filepath.Ext(name)))
	for _, s := range o.settings() {
		if *s.value != "" {
			args = append(args, "--input", s.key+"="+*s.value)
		}
	}
	if len(o.defaults) > 0 {
		// The user's defaults come in one input, since the front matter wins over them, but not over the others.
		data, err := json.Marshal(o.defaults)
		if err != nil {
			panic(err) // strings and lists of strings
		}
		args = append(args, "--input", "defaults="+string(data))
	}
	args = append(args, pdfLevels[o.level()]...)
	output := o.output
	if strings.HasPrefix(output, "-") {
		output = "." + string(filepath.Separator) + output // not an option, nor stdout
	}
	return append(args, "-", output)
}

// pageTree finds the page count when pdfpack did not. Typst writes one flat, uncompressed page tree, so the count can
// be read as text.
var pageTree = regexp.MustCompile(`/Type */Pages[^>\n]*/Count ([0-9]+)`)

// report says what setdown wrote; pages is the page count, 0 if unknown.
func report(stderr io.Writer, output string, pages int) {
	data, err := os.ReadFile(output)
	if err != nil {
		notef(stderr, "wrote %s", output)
		return
	}
	size := formatSize(int64(len(data)))
	if pages == 0 {
		if m := pageTree.FindSubmatch(data); m != nil {
			pages, _ = strconv.Atoi(string(m[1]))
		}
	}
	switch pages {
	case 0:
		notef(stderr, "wrote %s (%s)", output, size)
	case 1:
		notef(stderr, "wrote %s (1 page, %s)", output, size)
	default:
		notef(stderr, "wrote %s (%d pages, %s)", output, pages, size)
	}
}

func formatSize(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1<<20:
		return fmt.Sprintf("%.0f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	}
}

func printVersion(stdout, stderr io.Writer) int {
	m, err := payload.ReadManifest()
	if err != nil {
		notef(stderr, "%v", err)
		return 1
	}
	fmt.Fprintf(stdout, "setdown %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(stdout, "Typst %s\n", m.Typst)
	for _, f := range m.Fonts {
		fmt.Fprintf(stdout, "%s %s\n", f.Name, f.Tag)
	}
	fmt.Fprintf(stdout, "cmarker %s\n", m.Cmarker)
	return 0
}

// printLicenses shows the license texts that have to travel with the binary: setdown's own (MIT) and those of the
// bundled parts (the OFL requires it for the fonts).
func printLicenses(stdout, stderr io.Writer) int {
	m, err := payload.ReadManifest()
	if err != nil {
		notef(stderr, "%v", err)
		return 1
	}
	fmt.Fprintln(stdout, "setdown is under the MIT License and bundles Typst, four font families and the cmarker "+
		"package, and Go and the Go modules it is built with. The license texts follow.")
	for _, l := range m.Licenses {
		text, err := payload.License(l.File)
		if err != nil {
			notef(stderr, "%v", err)
			return 1
		}
		fmt.Fprintf(stdout, "\n==> %s <==\n\n", l.Title)
		stdout.Write(bytes.TrimRight(text, "\r\n"))
		fmt.Fprintln(stdout)
	}
	return 0
}

func notef(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, "setdown: "+format+"\n", args...)
}

// exitCode is the exit code a shell reports for a process ended by a signal.
func exitCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok && s > 0 {
		return 128 + int(s)
	}
	return 1
}
