---
title: Text, Links and Notes
date: 2000-01-01
---

This example goes through the Markdown that works within and between paragraphs.

## Emphasis

| Markdown                      | Result                    |
|-------------------------------|---------------------------|
| `*italic*` or `_italic_`      | *italic*                  |
| `**bold**` or `__bold__`      | **bold**                  |
| `***both***`                  | ***both***                |
| `~~struck~~`                  | ~~struck~~                |
| `H<sub>2</sub>O`              | H<sub>2</sub>O            |
| `mc<sup>2</sup>`              | mc<sup>2</sup>            |
| `<mark>highlighted</mark>`    | <mark>highlighted</mark>  |

A backslash keeps a character from meaning something: `\*not italic\*` gives \*not italic\*, and `5 \* 3` gives 5 \* 3.

## Punctuation

Straight quotes become typographic ones: "double" and 'single', also when they are "nested 'like this'". Two hyphens make an en dash for ranges, as in 9--5 or pages 10--12, three make an em dash---like this---and three dots make an ellipsis... Quotation marks typed directly, “like these”, are left as they are, or adjusted to the language.

Dollar signs stay what they are, so prices such as $5 and $10 print as written; Markdown math is not interpreted.

## Line breaks

A single line break in the Markdown is a space, so a paragraph can be written one sentence per line.
It still comes out as one paragraph.

For a line break within a paragraph, end the line with a backslash or two spaces, or write `<br>`:

Roses are red,\
violets are blue,\
Markdown goes in,\
a PDF comes through.

## Footnotes

A footnote is marked in the text with a caret and a name in brackets,[^short] and its text can stand anywhere in the file. Notes are numbered in the order they appear, and each goes to the bottom of the page that refers to it.[^long]

[^short]: Like this one.

[^long]: A footnote can be longer than a line. It is set in a smaller size, justified like the text, and Typst keeps it on the same page as its mark.

## Links

A link like [Typst's documentation](https://typst.app/docs/) is underlined and opens in a browser. A reference-style link puts the address elsewhere, as for [cmarker][] and [the fonts][fonts]. An address in angle brackets is a link as well: <https://github.com/patrickbergner/setdown>.

Links can also point into the document: [back to Emphasis](#emphasis) jumps to that section. Each heading gets a label from its text, lowercase and with hyphens for spaces.

[cmarker]: https://typst.app/universe/package/cmarker
[fonts]: https://github.com/adobe-fonts/source-serif

## Block quotes

> A block quote is set off by a rule on its left.
>
> > It can hold another quote,
>
> as well as **formatting**, `code` and lists:
>
> - one
> - two

## Rules

Three dashes on a line of their own draw a rule across the text:

---

<!--typst-begin-exclude-->
This sentence is only shown by other Markdown programs. setdown leaves out anything between `typst-begin-exclude` and `typst-end-exclude` comments.
<!--typst-end-exclude-->

Comments themselves are never shown: <!-- like this one --> there is nothing between these two words.
