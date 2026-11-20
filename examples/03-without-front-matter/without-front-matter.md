This document has no front matter, so setdown uses its defaults: Source Serif 4 at 11 pt, an A4 page with 20 mm margins, English hyphenation and quotation marks, and code blocks in black on light gray. Nothing is rendered as a title, and the PDF's metadata takes the file name, "without-front-matter", as its title.

The document fits on one page, so it has no page number either. Page numbers appear only from two pages on.

## Settings without front matter

The same settings can come from the command line instead:

```bash
setdown --sans --lang de --paper us-letter --margin 1in without-front-matter.md
```

A heading in the text is just a heading, also when it comes first. To get a title, set `title` in the front matter, as the example before this one shows.
