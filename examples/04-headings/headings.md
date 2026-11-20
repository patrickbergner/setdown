---
title: Headings
date: 2000-01-01
toc: true
heading-numbering: 1.1
---

Headings get more space above them than below, so each one binds to the section it opens. They are set in the semibold of the body font, neither justified nor hyphenated, and a heading never ends a page on its own: Typst moves it to the next page together with the start of its section. This document numbers its headings and lists them in a table of contents, as the [last section](#numbers-and-a-table-of-contents) explains.

# A level 1 heading

The front matter's title is not a heading; it stands above everything else. A level 1 heading, written with one `#`, is the largest within the text.

## A level 2 heading

Written with `##`. Most documents use this level for their sections.

### A level 3 heading

Written with `###`, for subsections.

#### A level 4 heading

Written with `####`. It is set in the size of the body text, and it serves levels 5 and 6 as well.

##### A level 5 heading

Looks like level 4.

###### A level 6 heading

Looks like level 4 too.

## Bold lines as headings

Many Markdown documents use a paragraph of nothing but bold text as a small heading. setdown sets such a line like a level 4 heading and keeps it with what follows:

**Goal**

Make the conversion from Markdown to PDF a single command that needs nothing installed.

**Approach**

Bundle Typst, the fonts and the template into one executable for each platform.

A bold word inside a sentence, such as **this one**, stays part of its paragraph. The rule only applies to a paragraph that is bold from its start to its end.

## A long heading that runs over more than one line, to show that headings are neither justified nor hyphenated

The line breaks in a heading fall between words, and its lines are left-aligned.

## Numbers and a table of contents

Two keys of the front matter ask for them:

- `toc: true` puts a table of contents below the title, with the headings of levels 1 to 3 and their pages. A number instead of `true` sets how many levels it lists: `toc: 2` for two, `toc: 6` for all. Its title, "Contents", follows the document's language.
- `heading-numbering: 1.1` numbers the headings by a Typst numbering pattern, such as `1.`, `1.1`, `1.a` or `I.1`. The numbers count from the document's top level, so a document whose sections start at `##`, below a front matter title, counts them 1, 2, 3.

A bold line used as a heading gets neither a number nor an entry. On the command line, `--toc`, `--toc=2`, `--no-toc` and `--heading-numbering 1.1` do the same.
