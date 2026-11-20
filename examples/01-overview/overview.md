---
title: An Overview of setdown
author: setdown
date: 2000-01-01
lang: en
---

This document shows how setdown sets Markdown: justified text with English hyphenation, typographic "quotation marks" made from 'straight' ones, dashes made from `--` -- like these -- and an ellipsis made from three dots...

## Paragraphs and line breaks

Typst breaks a paragraph as a whole rather than line by line. That keeps the spaces between words even in justified text, also around long words such as incomprehensibility, electroencephalography or counterrevolutionaries. Single lines at the top or bottom of a page (widows and orphans) are avoided, and a heading always stays with the paragraph that follows it.

A second paragraph shows the space between paragraphs. It is a little larger than the space between lines, so the structure stays visible without the text falling apart. The line spacing suits the line length of an A4 page with 20 mm margins.

## Code

Inline code like `setdown --scale 0.9` is set in JetBrains Mono, at the size where its x-height matches that of the body text. Code blocks get a light background; with `--highlight`, they are colored by their language:

```bash
# A letter in German, the text scaled to 90 %
setdown --lang de --scale 90% letter.md
for f in *.md; do setdown --sans --highlight "$f"; done
```

## Lists

- A bullet with **bold** and *italic* text
- A bullet with ~~strikethrough~~ and a [link](https://typst.app/)
- A bullet with a footnote[^1]

1. First step
2. Second step
   - nested
   - and another one

- [x] A task that is done
- [ ] A task that is still open

[^1]: Footnotes appear at the bottom of the page they belong to.

## Tables

| Typeface       | Used for                  | Styles |
|:---------------|:-------------------------:|-------:|
| Source Serif 4 | body text (the default)   |     12 |
| Source Sans 3  | body text with `--sans`   |     14 |
| JetBrains Mono | code                      |     16 |

## Quotes

> A block quote is set off by a rule on its left. It is justified as well, and its "quotation marks" follow the language of the document.

---

## Page breaks

A manual page break is a Typst command in an HTML comment, which other Markdown programs ignore:

<!--raw-typst #pagebreak()-->

## After the break

This heading starts on a new page. From two pages on, the document gets page numbers in its footer.

The other examples next to this one go into each of these features in detail.
