# setdown

Converts Markdown to a typographically high-quality PDF. Advanced use cases like letterheads, e-invoices,
archival PDF/A and accessible PDF/UA are just one option away.

setdown is a single executable with everything it needs inside: a pinned release of the [Typst](https://typst.app/)
typesetting system, the [cmarker](https://typst.app/universe/package/cmarker) Markdown reader package, the Source
Serif 4, Source Sans 3, JetBrains Mono and Noto Emoji fonts and its Typst template.

Nothing but setdown needs to be installed and it works offline.

```bash
setdown report.md    # → report.pdf
```

## Install

Each [release](https://github.com/patrickbergner/setdown/releases) has a zip file per platform.

Unpack it and put the program in a directory on your PATH, such as `/usr/local/bin` or `~/.local/bin`:

A package is about 30 MB, the program in it about 60 MB, most of that Typst and the fonts.
The Linux programs are static and run on any distribution.

**Windows:** setdown is not signed, so SmartScreen warns about it when it was downloaded with a browser. Choose More
info › Run anyway, Unblock in the file properties in Explorer or run `Unblock-File setdown.exe` in PowerShell.

**macOS:** setdown is not notarized. Downloaded with a browser, macOS refuses to open it the first time;
allow it once under System Settings › Privacy & Security (Open Anyway), or remove the quarantine attribute:
`xattr -d com.apple.quarantine setdown`.

To uninstall, run `setdown --clean-cache` (see [Files on disk](#files-on-disk)) and delete the program, and the
[config file](#user-defaults) if you made one.

## Usage

Example calls:

```bash
setdown report.md    # → report.pdf
setdown --lang de report.md out/report.pdf
setdown --sans --scale 90% --margin 15mm report.md
```

| Option | Default | |
|---|---|---|
| `--sans`, `-s` / `--serif` | serif | Source Sans 3 instead of Source Serif 4 |
| `--font <family>` | `serif` | the text in another font family (see [Fonts and style](#fonts-and-style)); `serif` and `sans` name the bundled ones |
| `--mono-font <family>` | JetBrains Mono | code in another font family |
| `--font-path <dir>` | | also take fonts from this directory; can be given more than once |
| `--style <file>` | | a Typst file whose set and show rules apply after the template's (see [Fonts and style](#fonts-and-style)) |
| `--lang <code>`, `-l` | `en` | hyphenation and quotation marks, e.g. `de`, `de-CH` |
| `--paper <name>`, `-p` | `a4` | any [Typst paper name](https://typst.app/docs/reference/layout/page/#parameters-paper) |
| `--landscape` / `--portrait` | portrait | the paper turned sideways; the margins stay as they are |
| `--margin <length>`, `-m` | `20mm` | in `mm`, `cm`, `in` or `pt`; two to four lengths set the sides as in CSS: `30mm 20mm` is top and bottom, then left and right, `30mm 20mm 25mm` top, left and right, bottom, and `30mm 20mm 25mm 15mm` top, right, bottom, left |
| `--first-margin-top <length>` | as the other pages | the first page's top margin, e.g. `60mm` to clear a letterhead; at least the other pages' (see [Backgrounds](#backgrounds)) |
| `--background <file>` | | a PDF, SVG, PNG, JPEG, GIF or WebP file drawn behind every page, at the page's width from its top; `none` for none; see [Backgrounds](#backgrounds); only as an option |
| `--first-background <file>` | | the same for the first page, instead of `--background`; only as an option |
| `--scale <factor>` | `1` (11 pt) | text size, as a factor (`0.9`) or percentage (`90%`); margins stay |
| `--highlight` / `--no-highlight` | off | color code blocks by their language |
| `--bullet <marker>` | `–` | the marker of bulleted lists, e.g. `•` or `▸` |
| `--page-numbers <format>` | `1 / 5` | the page numbers in the footer, by example: the first number is the page, an optional second one the number of pages, the rest stays as written: `1`, `1 of 5`, `1-5`, `Page 1 of 5`, or `none` |
| `--numbering <pattern>` | `1.` | how numbered lists count, as a [Typst numbering pattern](https://typst.app/docs/reference/model/numbering/): `1)`, `a)`, `(i)`, or `1.a.i.` for one style per level |
| `--heading-numbering <pattern>` | none | numbers the headings by a Typst numbering pattern: `1.`, `1.1`, `1.a`, `I.1`; counted from the document's top heading level, so sections that start at `##` are 1, 2, 3 |
| `--toc` / `--no-toc` | off | a table of contents below the title, of the heading levels 1 to 3; `--toc=2` for two levels, up to 6. Its title follows `lang` |
| `--pdf-level <level>` | `default` | `default` for a small PDF, `a3b` for PDF/A-3b (archiving, e-invoices), `ua1` for PDF/UA-1 (accessibility); see [PDF levels](#pdf-levels); only as an option |
| `--invoice <file.xml>` | | a Factur-X/ZUGFeRD e-invoice: PDF/A-3b with this invoice XML attached; see [E-invoices](#e-invoices); only as an option |
| `--quiet`, `-q` / `--no-quiet` | off | no `wrote out.pdf` line when all goes well, for scripts and Makefiles; errors and Typst's warnings still show |
| `--version` | | the versions of setdown, Typst, the fonts and cmarker |
| `--licenses` | | the license texts of setdown and the bundled Typst, fonts and cmarker |
| `--clean-cache` | | remove the files setdown unpacked, e.g. before uninstalling |
| `--help`, `-h` | | |

Options can come before or after the file names, values follow their option (`--lang de`) or come after an equals
sign (`--lang=de`), and `--` ends the options. The output defaults to the input's path with `.pdf`.

The same settings can go into YAML front matter; options on the command line win:

```yaml
---
title: Release Notes     # rendered as the title and stored as PDF metadata
author: Jane Doe         # one name or a list; shown below the title with the date
date: 2026-11-21
lang: en
font: sans               # or serif, or a family name such as Inter
mono-font: JetBrains Mono
scale: 0.95
paper: a4
landscape: true
margin: 20mm             # or per side, as in CSS: 30mm 20mm 25mm
first-margin-top: 60mm   # the first page's top margin, e.g. below a letterhead
highlight: true
bullet: •                # or one per level: [•, ◦]
numbering: 1)            # quote it if YAML would read a number: "1."
heading-numbering: 1.1   # unquoted is fine: 1 and 1. stand for "1.", 1.1 for "1.1"
toc: true                # or the number of levels, 1 to 6; true lists 3
page-numbers: Page 1 of 5   # or 1, 1 of 5, 1-5, none; the default is 1 / 5
---
```

Without front matter, the PDF metadata title is the file name and no title is rendered. In the tagged PDF levels
(`a3b`, `ua1`), the title is the document's level 1 heading for screen readers, so the document's own headings fit
below it from `##` on. It does not appear in a table of contents or the PDF's bookmarks.

Messages from setdown start with `setdown:`; errors in the document come from Typst as it reports them. setdown exits
with Typst's exit code. With `--quiet`, a run that goes well prints nothing.

## User defaults

Settings you want for every document go into a config file or the environment variable `SETDOWN_OPTIONS`, instead of
onto every command line or into every front matter. Both are defaults: the front matter wins over them, so a document
keeps what it says about itself, and the command line wins over everything. `SETDOWN_OPTIONS` wins over the config
file.

```bash
export SETDOWN_OPTIONS='--sans --lang de --margin "25mm 20mm"'
```

The config file is YAML with the front matter's keys, all but `title`, `author` and `date`, and six of its own:

```yaml
font: sans
lang: de
bullet: [•, ◦]
heading-numbering: 1.1
font-path: [fonts, ~/more-fonts]   # one directory or a list; relative to this file
style: corp.typ                    # relative to this file
background: ~/letters/page.svg     # behind every page; relative to this file
first-background: letterhead.pdf   # behind the first page; none for none
pdf-level: ua1                     # an --invoice still makes a PDF/A-3b
quiet: true
```

| | Config file |
|---|---|
| Linux | `$XDG_CONFIG_HOME/setdown/config.yaml`, else `~/.config/setdown/config.yaml` |
| macOS | `~/Library/Application Support/setdown/config.yaml` |
| Windows | `%APPDATA%\setdown\config.yaml` |

`setdown --help` names the file on the system at hand. `SETDOWN_CONFIG` names another file, and set but empty
(`SETDOWN_CONFIG=`) skips using the config file at all. A key the file does not know, or a value setdown refuses,
stops a conversion with a message that names the file and line. `--font-path` directories from all three places
add up; `--style`, `--background`, `--first-background` and `--pdf-level` given as options replace the defaults'
(`--background none` drops a default background). `--no-highlight`, `--portrait`, `--no-toc`
and `--no-quiet` turn off what a default turned on.

## Examples

[examples/](examples/) holds a document per feature, each Markdown file next to the PDF.

| Example | Shows |
|---|---|
| [01-overview](examples/01-overview/overview.md) ([PDF](examples/01-overview/overview.pdf)) | a tour of everything: paragraphs, code, lists, tasks, a table, a quote, a page break, page numbers |
| [02-front-matter](examples/02-front-matter/front-matter.md) ([PDF](examples/02-front-matter/front-matter.pdf)) | every front matter key (title, authors, date, language and region, sans, scale, A5 paper, margins, highlighting, list markers, page numbers) and the options that override them |
| [03-without-front-matter](examples/03-without-front-matter/without-front-matter.md) ([PDF](examples/03-without-front-matter/without-front-matter.pdf)) | the defaults; the file name as the PDF's title; no page number on a single page |
| [04-headings](examples/04-headings/headings.md) ([PDF](examples/04-headings/headings.pdf)) | six heading levels, bold lines used as headings, long headings, numbered headings (`heading-numbering`) and a table of contents (`toc`) |
| [05-lists](examples/05-lists/lists.md) ([PDF](examples/05-lists/lists.pdf)) | bullets, numbers, nesting, groups, lead-ins that stay with their list, paragraphs in items, task lists, other markers from a comment in front of a list |
| [06-tables](examples/06-tables/tables.md) ([PDF](examples/06-tables/tables.pdf)) | landscape paper (`landscape`), column alignment, formatting in cells, lead-ins, a wide table, long cells |
| [07-code](examples/07-code/code.md) ([PDF](examples/07-code/code.pdf)) | inline code, code blocks, highlighting for Python, Go, Rust, TypeScript, JSON, YAML, SQL, HTML, diffs and Bash |
| [08-images](examples/08-images/images.md) ([PDF](examples/08-images/images.pdf)) | SVG and PNG images, alternative text, a numbered figure with a caption, image width, inline SVG |
| [09-text-and-links](examples/09-text-and-links/text-and-links.md) ([PDF](examples/09-text-and-links/text-and-links.pdf)) | emphasis, sub- and superscript, smart punctuation, line breaks, footnotes, links within and out of the document, block quotes, rules, text only other programs show |
| [10-german](examples/10-german/german.md) ([PDF](examples/10-german/german.pdf)) | German hyphenation and quotation marks, § and its abbreviations, the Swiss variant |
| [11-raw-typst](examples/11-raw-typst/raw-typst.md) ([PDF](examples/11-raw-typst/raw-typst.pdf)) | Typst code in comments: page breaks, alignment, color, a box, columns |
| [12-letter](examples/12-letter/letter.md) ([PDF](examples/12-letter/letter.pdf)) | a letter on US Letter paper with 1 inch margins: address lines, an invoice table |
| [13-fonts-and-style](examples/13-fonts-and-style/fonts-and-style.md) ([PDF](examples/13-fonts-and-style/fonts-and-style.pdf)) | font families by name from the front matter, `--font-path`, and a style file ([style.typ](examples/13-fonts-and-style/style.typ)) with colored headings and a header with a logo and a front matter key of its own |
| [14-emoji](examples/14-emoji/emoji.md) ([PDF](examples/14-emoji/emoji.pdf)) | emoji in text, headings, lists, tables and code, joined emoji, skin tones, flags and keycaps, symbols that stay text, emoji in colored text |
| [15-backgrounds](examples/15-backgrounds/backgrounds.md) ([PDF](examples/15-backgrounds/backgrounds.pdf)) | a letterhead on the first page ([letterhead.pdf](examples/15-backgrounds/letterhead.pdf), made from [letterhead.typ](examples/15-backgrounds/letterhead.typ)) and a band on the others ([page.svg](examples/15-backgrounds/page.svg)), `first-margin-top` below the letterhead |
| [16-pdf-a3b](examples/16-pdf-a3b/pdf-a3b.md) ([PDF](examples/16-pdf-a3b/pdf-a3b.pdf)) | a Factur-X/ZUGFeRD e-invoice (`--invoice`): PDF/A-3b with the invoice attached as XML |
| [17-pdf-ua1](examples/17-pdf-ua1/pdf-ua1.md) ([PDF](examples/17-pdf-ua1/pdf-ua1.pdf)) | a PDF/UA-1 document (`--pdf-level ua1`): what the tags hold, headings in order, alternative text, table headers |

## Markdown

CommonMark with tables, footnotes, strikethrough and task lists, as
[cmarker](https://typst.app/universe/package/cmarker) supports it. Notes:

- **Page break:** `<!--raw-typst #pagebreak()-->`. Any Typst code works in such a comment, and other Markdown tools
  ignore it.
- **Images** resolve relative to the Markdown file and must lie in its directory or below; remote URLs do not work.
- **Math** (`$…$`) stays plain text, so prices with dollar signs are safe.
- **List markers** come from `bullet` and `numbering`, not from the Markdown: `1)` numbers a list like `1.`, and `*`
  or `+` bullet it like `-`. A Typst set rule in a comment changes them from where it stands, inside a list item for
  that item only: `<!--raw-typst #set list(marker: [•])-->` or `<!--raw-typst #set enum(numbering: "a)")-->` in front
  of a list. The [lists example](examples/05-lists/lists.pdf) shows each.

## PDF levels

`--pdf-level` chooses what goes into the PDF besides its pages. Every level keeps the text searchable and copyable,
the bookmarks (one per heading), the links and the document's title, authors and dates.

| Level | | What it adds |
|---|---|---|
| `default` | PDF 1.7 | nothing: the smallest file, for reading, printing and sending |
| `a3b` | PDF/A-3b | tags for screen readers, XMP metadata and an sRGB output intent; allows attached files, as e-invoices need |
| `ua1` | PDF/UA-1 | tags for screen readers and XMP metadata; requires alternative text for every image |

With `a3b` and `ua1`, Typst checks the document against the standard; a document that breaks one of its rules fails
with Typst's message saying which.

**PDF/UA-1** requires a level 1 heading first and no skipped levels after it (`##`, then `####`). The front matter's
`title` is that heading, so a document with a title starts its own at `##`; one without a title starts at `#`.

Typst cannot embed a PDF in a PDF/A-3b or PDF/UA-1, so at these levels a PDF given as a
[background](#backgrounds) becomes an SVG first, with a warning.

## Typography

- Body text is justified with hyphenation for the document's language. Typst breaks whole paragraphs at once
  (Knuth–Plass), avoids widows and orphans, and keeps headings with the block that follows.
- Headings get more space above than below, so they bind to their section; size and spacing per level are in
  `heading-styles` in the template. A paragraph that is only bold text (`**Goal**`) is treated like a level 4 heading.
- Typst itself keeps only headings with what follows, so the template adds keep-with-next before layout: for a bold
  line used as a heading, for a paragraph ending in a colon before a list or table, and for the text of a list item
  before its nested list. A lead-in like "2. By the end of 2026:" never ends a page without its items.
- Lists use an en dash at every level (unless `bullet` says otherwise) and leave a little more room between items
  than between lines. A list sits closer to the sentence that introduces it than to the text after it; nested lists
  keep the item spacing. A list whose items hold nested lists separates its items more (`group-spacing`), so each
  group reads as one.
- A list never starts or ends a page with a single item: the first item stays with the second and the second to last
  with the last. Typst has no stretchable vertical spacing and its list items cannot be kept together, so the
  template lays out lists itself, one block per item. Only items of up to about three lines are kept with their
  neighbor, since Typst moves such a block to the next page as a whole and a long one would leave a large gap. Task
  lists keep Typst's own layout.
- A section sign and the abbreviations Abs., Nr. and Art. stay on the line of the number that follows.
- Code is set in JetBrains Mono (or `mono-font`), scaled so its x-height matches the body font's. The factor is measured at render time,
  so it also holds for the sans font and inside headings. Spaces in inline code keep their width in justified lines.
  Its programming ligatures are off, so `->` or `!=` print as typed.
- Code blocks are black on light gray. With `--highlight`, they are colored by the language named after the opening
  backticks, in a theme of its own (in the template) whose colors all reach a contrast of at least 4.5:1.
- Straight quotes become the language's quotation marks („…“ for `de`, “…” for `en`). Quotes typed directly are adjusted
  as well.
- Page numbers appear at the bottom right, only when there is more than one page, as `1 / 3` unless `page-numbers`
  says otherwise.
- With `toc`, a table of contents follows the title, with dot leaders to the page numbers and the first level in
  semibold. Its title is a level 1 heading, in the document's language, that stays out of the bookmarks.
  `heading-numbering` numbers the headings and their entries; the title and bold lines used as headings stay
  unnumbered.
- Emoji come from Noto Emoji, in black and white like the text, in its size and color, also in code. It sets the
  characters that are emoji by default, joined emoji (👩‍💻), skin tones, flags, keycaps and symbols written with the
  emoji variation selector (❤️), which emoji pickers add. A symbol without it, such as ★, ✓, © or ™, stays in the text
  fonts. Shortcodes such as `:smile:` stay as written.
- A character the body font lacks comes from the other bundled fonts or from those built into Typst (Libertinus
  Serif, New Computer Modern, DejaVu Sans Mono); the few that no font has come out as empty boxes.

The template is [template.typ](internal/payload/template.typ). It is built into setdown and reaches Typst on stdin,
so the Markdown file's directory can be the project root.

## Fonts and style

setdown takes no fonts from the system, so a document comes out the same on every machine. Besides its own four
families, Typst's built-in Libertinus Serif, New Computer Modern and DejaVu Sans Mono can be named. For others,
`--font-path` names a directory of font files (`.ttf`, `.otf`, `.ttc`, also in its subdirectories), and `--font` and
`--mono-font`, or `font` and `mono-font` in the front matter, pick a family by name:

```bash
setdown --font-path ~/fonts --font "Inter" --mono-font "Iosevka" report.md
```

- The name is the family's own, as a font viewer shows it, not the file name. Typst warns about a family it does not
  find and sets the text in the bundled fonts instead.
- The template uses regular, italic, bold and semibold; put all four of the family into the directory.
- Code is scaled to the body font's x-height whatever the fonts, and its spaces keep the code font's width.

`--style <file>` adds Typst code of its own after the template's rules, so its set and show rules win over the
template's and reach the title as well:

```typ
// corp.typ
#set text(fill: rgb("#1a2b4c"))
#show heading: set text(fill: rgb("#a3155f"))
#set page(header: align(right, text(size: 0.85em, meta.at("client", default: ""))))
```

```bash
setdown --style ~/styles/corp.typ --font-path ~/fonts --font "Inter" offer.md
```

- The style becomes part of the template, which reaches Typst on stdin, so its paths (an `image`, an `#import`) are
  relative to the Markdown file and lie in its directory or below, like the document's own. A logo goes next to the
  document.
- It can use `meta`, the front matter as a dictionary (so keys of your own such as `client:` work), and
  `setting(key, default)`, which gives the option, else the front matter, else the default. The template's other
  names may change between releases.
- The front matter's title is a level 1 heading, so a `show heading` rule reaches it as well; the label
  `<setdown-title>` names the title alone: `#show <setdown-title>: set text(fill: black)`.
- A header or footer sits in the page's margin; give it room with a larger one, such as `margin: 32mm 20mm 20mm`
  for the top.
- `#set page(footer: …)` replaces the page numbers. The size of code stays matched to the body font.
- Typst's messages call the template with the style `<stdin>`; when Typst fails, setdown says which of its lines are
  the style file's.
- `--style` and `--font-path` are options and [user defaults](#user-defaults), not front matter: setdown hands them
  to Typst before Typst reads the front matter, as `--pdf-level` and `--invoice`.
  For a single document, a `raw-typst` comment at its top can hold the same rules.

## Backgrounds

`--background` draws a file behind every page, `--first-background` one behind the first page instead, for a
letterhead, a band at the top of each page, a frame or a watermark:

```bash
setdown --first-background letterhead.pdf --background page.svg --first-margin-top 60mm letter.md
```

- The file is scaled to the page's width and placed at its top; its height follows from the file. A file with the
  page's proportions covers the page, a 45 mm strip as wide as the page only its top.
- PDF (its first page), SVG, PNG, JPEG, GIF and WebP work; setdown reads the format from the file, not its name.
  For print, a raster image wants about 2500 pixels across an A4 page (300 dpi). A PDF keeps its text as text.
- With `--first-background` alone, the pages after the first stay plain; with `--background` alone, the first page
  has it too. `none` turns either off where a [user default](#user-defaults) sets one: `--first-background none`
  leaves the first page plain and the others as they are.
- The text does not make way for a background: the margins give it room. `margin` sets every page's,
  `first-margin-top` the first page's top margin, which can be larger, for a letterhead. Typst cannot change the margin
  for one page, so the template moves the first page's text down by the difference; it cannot be smaller than the
  others'.
- The file can lie anywhere: setdown hands it to Typst as part of the template, as the XML of an e-invoice. So the two
  options work as options and as user defaults, like `--style`, but not in front matter.
- Typst cannot embed a PDF in a PDF/A-3b or PDF/UA-1 (`--pdf-level a3b` or `ua1`, and an e-invoice). There, setdown
  first turns a PDF background into an SVG of its first page with the bundled Typst and warns that it did: the page
  looks the same, but the background's text becomes outlines, not text that can be searched or copied. An SVG of
  your own avoids the conversion.
- In a tagged PDF, a background is an artifact: screen readers skip it, and PDF/UA asks no alternative text for it.
- `--style` code can still set a background of its own with `#set page(background: …)`, which wins.

## E-invoices

`--invoice invoice.xml` makes a Factur-X and ZUGFeRD 2.x e-invoice: a PDF/A-3b of the Markdown with the invoice's
XML attached, for people and accounting software to read the same invoice.

```bash
setdown --invoice factur-x.xml invoice.md
```

- The XML is a UN/CEFACT Cross Industry Invoice, and its profile comes from it: BASIC, EN 16931, EXTENDED or
  XRECHNUNG. MINIMUM and BASIC WL carry no full invoice; setdown refuses them.
- setdown attaches it as `factur-x.xml` (`xrechnung.xml` for XRECHNUNG) with the relationship `Alternative` and writes
  the metadata the formats require into the PDF: document type, file name, version and profile. The file can lie
  anywhere.
- The XML comes from accounting software or an e-invoicing library; setdown does not write or check it. The pages and
  the XML must agree.

## Files on disk

Typst reads fonts and packages only from files, so setdown has to put them on disk.

**Linux:** nothing persists. Typst runs straight from memory. The fonts and cmarker go into a private
temporary directory, in `$XDG_RUNTIME_DIR` if it is set, otherwise in `$TMPDIR` or `/tmp`, which is removed when the
run ends, also after Ctrl+C. A run killed with `kill -9` leaves it behind; a later run removes it after a day, and
`setdown --clean-cache` right away. Where a system does not allow running programs from memory, Typst runs from the
temporary directory instead. If that directory is mounted `noexec` as well, setdown says so; point `TMPDIR` to a
directory that allows running programs.

**Windows and macOS:** neither can run a program from memory, so the first run unpacks Typst, the fonts and cmarker
(about 60 MB) into a cache, which takes a second. Later runs start right away.

| | Cache |
|---|---|
| Windows | `%LOCALAPPDATA%\setdown\` |
| macOS | `~/Library/Caches/setdown/` |

Set `SETDOWN_CACHE` to use another directory. The cache keeps one version: after an upgrade, the new version removes
the old one's files once the old one has not run for an hour. If files go missing from the cache (an antivirus
quarantine, a cleanup tool, macOS freeing disk space), setdown unpacks them again. `setdown --clean-cache` removes the
cache.

On Windows, Defender scans the unpacked `typst.exe` when it first starts, which can take a second or two once. A
policy that blocks programs in user folders (AppLocker, WDAC) blocks it as well; then set `SETDOWN_CACHE` to a folder
from which programs may run.

**Packages:** a document that imports a Typst package itself, in a `raw-typst` comment, makes Typst download that
package into its own cache (`~/.cache/typst/packages`, `%LOCALAPPDATA%\typst\packages`,
`~/Library/Caches/typst/packages`). That is the only case in which anything goes online.

## Versions

`setdown --version` shows what is bundled.

Each setdown release pins its versions of Typst, the fonts and cmarker.

## Building

With [Go](https://go.dev/) 1.27 or later and Bash (Git Bash on Windows):

```bash
./build.sh                          # all five binaries, setdown-<platform>, into this directory
./build.sh linux/arm64              # only some
./run-tests.sh                      # gofmt, go vet and the tests, for the platform at hand
./dist/package.sh                   # the release zips and SHA256SUMS.txt into dist/, from the binaries
./update-deps.sh                    # move Typst, the fonts, cmarker and the Go modules to their latest releases
```

Both first run `go run ./tools/fetch`, which downloads the Typst releases, fonts and cmarker pinned in
[tools/versions.json](tools/versions.json) into `tools/download-cache/`, checks each against its SHA-256 and unpacks
what the binaries embed into `internal/payload/assets/`, removing the hinting from TrueType fonts (JetBrains Mono).
Noto Emoji has no releases; it is pinned as a file of the [google/fonts](https://github.com/google/fonts) repository
at a commit. The license texts of Go and the Go modules compiled in come from Go's module cache, so the modules must
be downloaded (`go mod download`). The version comes from [VERSION](VERSION).

## Licenses

setdown is under the [MIT License](LICENSE). It bundles [Typst](https://github.com/typst/typst) (Apache-2.0),
[Source Serif](https://github.com/adobe-fonts/source-serif), [Source Sans](https://github.com/adobe-fonts/source-sans),
[JetBrains Mono](https://github.com/JetBrains/JetBrainsMono) and [Noto Emoji](https://github.com/googlefonts/noto-emoji)
(each under the SIL Open Font License 1.1; setdown removes JetBrains Mono's hinting) and
[cmarker](https://github.com/SabrinaJewson/cmarker.typ) (MIT). It is built with [Go](https://go.dev/) (BSD-3-Clause)
and the Go modules [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) (BSD-3-Clause) and
[go.yaml.in/yaml/v4](https://github.com/yaml/go-yaml) (Apache-2.0, partly MIT). `setdown --licenses` shows all these
license texts, setdown's own included.
