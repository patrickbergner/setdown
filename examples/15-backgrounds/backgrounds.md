---
title: Backgrounds and a Letterhead
author: Northwind Type Foundry
date: 2026-11-21
font: sans
margin: 26mm 20mm 20mm
first-margin-top: 52mm
---

This document has a letterhead on its first page and a narrow band at the top of every other page. Neither is part of the Markdown: both come from files that setdown draws behind the pages, given in this example's `options.txt`:

```bash
setdown --first-background letterhead.pdf --background page.svg backgrounds.md
```

## How a background fits the page

A background is scaled to the width of the page and placed at its top. Its height follows from the file: a file as tall as an A4 page covers the whole page, and one 45 mm tall, as `letterhead.pdf` is, covers only a strip at the top. So the same file works as a full page with a frame or a watermark, and as a header across the top.

- `--background` is drawn behind every page.
- `--first-background` is drawn behind the first page instead. Without `--background`, the other pages stay plain.
- `none` turns either off, for example when a config file sets a letterhead that one document should not have.

The text does not move out of the way on its own. The margins give it room: this document's front matter sets a top margin of 26 mm for the band on the following pages, and `first-margin-top: 52mm` for the letterhead on the first page.

## Formats

A background can be a PDF, SVG, PNG, JPEG, GIF or WebP file. setdown reads the format from the file itself and passes the file to Typst whole, so it can lie anywhere, for instance next to the config file.

| Format | Drawn as |
|:-------|:---------|
| PDF | its first page, with its text as text (PDF/A and PDF/UA: converted to SVG first) |
| SVG | shapes and text, at any size |
| PNG, JPEG, GIF, WebP | pixels, stretched to the page width |

A PDF from a designer is usually the best choice for a letterhead. A raster image should be wide enough for print: 2480 pixels across for 300 dpi on A4.

## Archiving and accessibility

Typst cannot embed a PDF in a PDF/A-3b or PDF/UA-1 document. At those levels, and for an e-invoice, setdown first turns a PDF background into an SVG of its first page and says so with a warning. The page looks the same, but the background's text becomes outlines, which cannot be searched or copied. To avoid the conversion, give an SVG instead.

In a tagged PDF, a background is an artifact: screen readers skip it, and it needs no alternative text.

## A second page

The rest of this document only fills the page, so that the next one shows the band from `page.svg` instead of the letterhead.

A letterhead tells the reader who writes before they read a word: a name, an address, a mark. On the pages after the first, a quieter sign is enough to show that the pages belong together, a thin band, a small logo, or nothing at all.

Paper costs nothing in a PDF, but the conventions of paper still help. A reader who prints the document finds the same margins, the same position of the address and the same page numbers as in a printed letter.

Kind regards,

Northwind Type Foundry
