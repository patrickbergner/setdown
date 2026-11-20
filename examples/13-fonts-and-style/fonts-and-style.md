---
title: Fonts and Style
author: Jane Doe
date: 2000-01-01
client: Typesetting Supplies, Springfield
font: Libertinus Serif
mono-font: DejaVu Sans Mono
margin: 32mm 20mm 20mm
---

This document is set in Libertinus Serif, its code in DejaVu Sans Mono, two families built into Typst that the front matter names. Its look comes from `style.typ` next to it, a file of Typst rules that setdown applies after those of its template: the colored headings in Source Sans 3, the colored links and the header with the logo and the client.

```bash
setdown --style style.typ fonts-and-style.md
```

## Fonts by name

`font` and `mono-font` in the front matter, or `--font` and `--mono-font` on the command line, take the name of a font family; `font: sans` and `font: serif` stand for the bundled Source Sans 3 and Source Serif 4. setdown takes no fonts from the system, so a document comes out the same on every machine. Fonts of your own come from a directory of font files:

```bash
setdown --font-path ~/fonts --font "Inter" --mono-font "Iosevka" report.md
```

| Families                                                | Where they come from |
|---------------------------------------------------------|----------------------|
| Source Serif 4, Source Sans 3, JetBrains Mono           | bundled with setdown |
| Libertinus Serif, New Computer Modern, DejaVu Sans Mono | built into Typst     |
| any other                                               | `--font-path`        |

- The name is the family's own, as a font viewer shows it, not the file's.
- The template sets text in regular, *italic*, **bold** and semibold, as in the table's header; the directory needs all four.
- Typst warns about a family it does not find and sets the text in the bundled fonts instead.
- Inline code such as `--font "Inter"` is scaled to the body font's x-height whatever the fonts, and its spaces keep the width of the code font's.

## The style file

`--style` adds a file of Typst code after the template's rules and before the title, so its set and show rules win over the template's. This document's `style.typ`:

```typ
// The style of the fonts-and-style example: Typst rules that setdown
// applies after its template's.
#let accent = rgb("#a3155f")

// Headings in the bundled sans and the accent color; the body font comes
// from the front matter.
#show heading: set text(font: "Source Sans 3", fill: accent)
#show link: set text(fill: accent)

// A header with a logo and a front matter key of the document's own.
// The path is relative to the Markdown file.
#set page(header: grid(
    columns: (1fr, auto),
    align: horizon,
    image("logo.svg", height: 6mm),
    text(size: 0.85em, fill: luma(90), meta.at("client", default: "")),
))
```

- Paths in the style file are relative to the Markdown file and lie in its directory or below, as the document's own do: `logo.svg` lies next to this document, not necessarily next to the style.
- `meta` is the front matter as a dictionary, so `client`, a key setdown does not know, can end up in the header. `setting(key, default)` gives an option, else the front matter, else the default.
- The header sits in the top margin, so the front matter makes that one larger: `margin: 32mm 20mm 20mm` is 32 mm at the top, 20 mm left and right and 20 mm at the bottom, read as in CSS.
- `#set page(footer: …)` would replace the page numbers; the size of code stays matched to the body font.
- When Typst fails on the style, setdown says which lines of Typst's `<stdin>` are the style file's.

`--style` and `--font-path` exist only on the command line, since setdown hands them to Typst before Typst reads the front matter. For a single document, a `raw-typst` comment at its top can hold the same rules. The [setdown README](https://github.com/patrickbergner/setdown#fonts-and-style) has the details.
