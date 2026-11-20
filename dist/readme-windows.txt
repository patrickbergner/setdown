setdown for Windows

Converts Markdown to a typographically high-quality PDF. Advanced use cases
like letterheads, e-invoices, archival PDF/A and accessible PDF/UA are just
one option away.

INSTALL
  Copy setdown.exe into a folder on your PATH or start it from any folder.

  setdown is not signed. If SmartScreen warns at the first start (after a
  download with a browser), choose More info > Run anyway, file properties
  in Explorer and Unblock, or in PowerShell: Unblock-File setdown.exe

USE
  setdown report.md                     writes report.pdf next to it
  setdown report.md out/report.pdf
  setdown --sans --lang de report.md
  setdown --help                        all options
  setdown --version                     the bundled Typst, fonts and cmarker

SETTINGS
  option                   front matter               default
  -s, --sans / --serif     font: sans                 serif
  --font Inter             font: Inter                serif
  --mono-font Iosevka      mono-font: Iosevka         JetBrains Mono
  --scale 90%              scale: 0.9                 1 (11 pt)
  -l, --lang de            lang: de-CH                en
  -p, --paper a5           paper: us-letter           a4
  --landscape              landscape: true            portrait
  -m, --margin 15mm        margin: 1in                20mm
  --first-margin-top 55mm  first-margin-top: 55mm     as the others
  --highlight              highlight: true            off
  --bullet •               bullet: [•, ◦]             –
  --numbering "a)"         numbering: "1."            1.
  --heading-numbering 1.1  heading-numbering: 1.1     none
  --toc, --toc=2           toc: true, toc: 2          none
  --page-numbers none      page-numbers: Page 1 of 5  1 / 5
                           title, author, date        none

  Front matter goes at the top of the Markdown file, between two --- lines.
  Options on the command line win over it. Two to four margins set the
  sides as in CSS: "30mm 20mm" is top and bottom, then left and right;
  "30mm 20mm 25mm 15mm" is top, right, bottom and left. first-margin-top
  is the first page's, e.g. to clear a letterhead. --toc lists the
  heading levels 1 to 3, --toc=2 two. --portrait, --no-toc and
  --no-highlight turn off what front matter or a default turned on.

  --font-path fonts        also take fonts from this directory, for --font
                           and --mono-font; can be given more than once
  --style corp.typ         Typst set and show rules applied after the
                           template's; its paths are relative to the
                           Markdown file
  --background page.svg    a PDF, SVG, PNG, JPEG, GIF or WebP file drawn
                           behind every page, at the page's width from its
                           top; none for none
  --first-background letterhead.pdf
                           the same for the first page, instead of
                           --background
  --pdf-level a3b          PDF/A-3b, for archiving and e-invoices; ua1:
                           PDF/UA-1, accessible; default: the smallest
                           file, without tags for screen readers
  --invoice factur-x.xml   a Factur-X/ZUGFeRD e-invoice: PDF/A-3b with
                           this XML attached (BASIC, EN 16931, EXTENDED
                           or XRECHNUNG)
  -q, --quiet              no "wrote report.pdf" line when all goes well;
                           errors still show

DEFAULTS
  Your own defaults go into a config file (see FILES) with the front
  matter's keys and font-path, style, background, first-background,
  pdf-level and quiet, such as
    lang: de
    font: sans
  or into the environment variable SETDOWN_OPTIONS as options, such as
  --sans --lang de. The front matter wins over both, the command line over
  everything. SETDOWN_CONFIG names another config file, empty none.

FILES
  The first run unpacks Typst and the fonts (about 60 MB) into
  %LOCALAPPDATA%\setdown; set SETDOWN_CACHE to another folder to move them,
  for example when AppLocker or WDAC blocks programs in user folders.
  Defender may scan the unpacked typst.exe at its first start, once.
  Your defaults, if you make the file: %APPDATA%\setdown\config.yaml

UNINSTALL
  setdown --clean-cache then delete setdown.exe.

LICENSES
  setdown: MIT, see LICENSE. Bundled: Typst (Apache-2.0), Source Serif,
  Source Sans, JetBrains Mono and Noto Emoji (OFL-1.1), cmarker (MIT);
  built with Go and Go modules (BSD-3-Clause, Apache-2.0, MIT). Their
  texts: setdown --licenses

https://github.com/patrickbergner/setdown
