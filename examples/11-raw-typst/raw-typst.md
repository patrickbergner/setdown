---
title: Raw Typst
date: 2000-01-01
---

Typst code in an HTML comment that starts with `raw-typst` goes straight into the document. Other Markdown programs ignore such comments, so the file stays readable everywhere. Each section below shows one. A table of contents needs none: `toc: true` in the front matter puts one below the title, as in the [headings example](../04-headings/headings.md).

## Page breaks

`<!--raw-typst #pagebreak()-->` starts a new page, as after this paragraph.

<!--raw-typst #pagebreak()-->

## Alignment and spacing

<!--raw-typst #align(center)[This line is centered.]-->

<!--raw-typst #align(right)[And this one is flush right.]-->

<!--raw-typst #v(1.5em)-->

This paragraph is preceded by extra space, from `#v(1.5em)`.

## Color and boxes

Some <!--raw-typst #text(fill: rgb("#1849a8"))[blue text]--> in the middle of a sentence, and a box around a note:

<!--raw-typst
#block(fill: luma(246), inset: 0.8em, radius: 3pt, width: 100%)[
  *Note:* everything in a raw Typst comment is Typst markup, not Markdown. Typst writes bold with single asterisks
  and italic with underscores.
]
-->

## Columns

<!--raw-typst
#columns(2, gutter: 1.5em)[
  Typst can set text in columns. Both columns are justified and hyphenated like the rest of the document, since
  the template's settings apply to raw Typst as well.

  Typst does not balance columns: text fills the first one down to the end of the page before it goes on in the
  second, unless a column break says otherwise.

  #colbreak()

  This column starts after `#colbreak()`. #lorem(30)
]
-->

## Multiple lines

A comment may span several lines, as the box and the columns above do. It ends at the first `-->`.

## Packages

A document could import a Typst package in such a comment. Typst would then download it into its own cache, which is the only case in which setdown's output depends on the network; these examples do without.
