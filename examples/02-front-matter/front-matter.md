---
title: Front Matter
author:
  - Ada Lovelace
  - Charles Babbage
date: 2000-01-01
lang: en-GB
font: sans
scale: 0.95
paper: a5
margin: 15mm
highlight: true
bullet: •
numbering: (1)
page-numbers: Page 1 of 5
---

A Markdown file can start with a block of YAML between two lines of three dashes, its front matter. setdown reads its settings from there, so a document carries them along and needs no options on the command line. This one asks for Source Sans 3 at 95 %, an A5 page with 15 mm margins, British English, highlighted code, list markers of its own and page numbers that say "Page 2 of 3".

## The keys

| Key         | Here         | Option              | Default         |
|-------------|--------------|---------------------|-----------------|
| `title`     | Front Matter |                     | none            |
| `author`    | two names    |                     | none            |
| `date`      | 2026-11-21   |                     | none            |
| `lang`      | `en-GB`      | `--lang`            | `en`            |
| `font`      | `sans`       | `--sans`, `--serif` | `serif`         |
| `scale`     | `0.95`       | `--scale`           | `1`, or 11 pt   |
| `paper`     | `a5`         | `--paper`           | `a4`            |
| `landscape` |              | `--landscape`, `--portrait` | `false` |
| `margin`    | `15mm`       | `--margin`          | `20mm`          |
| `highlight` | `true`       | `--highlight`       | `false`         |
| `bullet`    | `•`          | `--bullet`          | `–`             |
| `numbering` | `(1)`        | `--numbering`       | `1.`            |
| `heading-numbering` |      | `--heading-numbering` | none          |
| `toc`       |              | `--toc`, `--no-toc` | `false`         |
| `page-numbers` | `Page 1 of 5` | `--page-numbers` | `1 / 5`         |

An option's value follows it after a space or an equals sign, as in `--scale 95%` or `--scale=95%`. `-l`, `-s`, `-p` and `-m` are short for `--lang`, `--sans`, `--paper` and `--margin`, and `--no-highlight` turns highlighting off again.

## Lists

`bullet` sets the marker of bulleted lists, or, given a list of markers, one for each level. `numbering` sets how numbered lists count, as a Typst pattern such as `1)`, `a)` or `1.a.i.`; quote one that YAML would read as a number, as in `numbering: "1."`.

- a bullet
- and another

1. counted
2. in parentheses

## Page numbers

`page-numbers` is written by example: the first number stands for the page, a second one for the number of pages, and everything else is printed as it stands. `1 / 5` is the default; `1`, `1 of 5`, `1-5`, `– 1 –`, `Page 1 of 5` and `Seite 1 von 5` work just as well, and `none` turns the numbers off. A document of one page has none either way.

## Title, authors and date

The title is set above the text and stored in the PDF's metadata. The authors, one name or a list of them, follow below it together with the date, and they are stored in the metadata as well. A document without a title shows none, and its metadata takes the file name instead.

## The command line wins

An option given on the command line overrides the front matter. This document, set in serif and without highlighting:

```bash
setdown --serif --no-highlight front-matter.md
```

Defaults of your own, for every document, go into a config file or the environment variable `SETDOWN_OPTIONS`. They take the same keys and options, and the front matter wins over them, so a document keeps what it says about itself.

## Language and region

`lang` takes a language code and, after a hyphen, an optional region. The language decides the hyphenation and the "quotation marks"; the region refines them where a country does it differently. A code block shows the highlighting this document asks for:

```yaml
lang: en-GB   # English as written in Britain
font: sans    # or serif, the default
scale: 0.95   # or 95%
```
