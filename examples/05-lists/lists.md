---
title: Lists
date: 2000-01-01
---

Lists use an en dash at every level and leave a little more room between their items than between lines. A list sits closer to the sentence that introduces it than to the text after it.

## Bullets

- Apples
- Pears
  - Williams
  - Conference
    - Even a third level uses the dash.
- Plums

The items of a list may be written with `-`, `+` or `*`; they all look the same.

## Numbers

1. Numbered lists count by themselves.
2. Their numbers are right-aligned, so that 9 and 10 line up.
3. Ten items show it:
4. Four
5. Five
6. Six
7. Seven
8. Eight
9. Nine
10. Ten

A list can also start at another number:

3. Third
4. Fourth
5. Fifth

## Groups

When the items of a list hold lists of their own, the items are separated more, so each group reads as one:

1. Before the end of 2026:
   - choose the typefaces
   - write the template
2. By spring 2027:
   - build the binaries
   - publish the first release
3. Later:
   - signed binaries for macOS and Windows

A lead-in such as "Before the end of 2026:" always stays with its nested list, so a page never ends after it.

## Lead-ins

A paragraph that ends in a colon introduces the list or table that follows, and the two are kept together:

- This item stays on the same page as the sentence above.
- So does the whole list, as long as it is short.

## Paragraphs in items

- An item can hold more than one paragraph.

  Its second paragraph is indented like the first, so the item stays one block.

- The next item continues the list.

## Tasks

- [x] Write the examples
- [x] Render them into PDFs
- [ ] Translate them into every language

Task lists get boxes that are drawn rather than taken from a font, ticked or empty.

## Markers

The Markdown does not choose the markers: `1)` numbers a list like `1.`, and `*` or `+` bullet it like `-`. The front matter's `bullet` and `numbering`, or `--bullet` and `--numbering`, choose them for the whole document. A Typst set rule in a comment changes them from where it stands, for the lists that follow:

```
<!--raw-typst #set list(marker: [•])-->
```

### Bullets

A bullet can be any character, or anything Typst can draw. Each of these lists follows a comment like the one above, with the set rule shown in its first item:

<!--raw-typst #set list(marker: [•])-->

- `#set list(marker: [•])`
- the round bullet of books and slides

<!--raw-typst #set list(marker: [→])-->

- `#set list(marker: [→])`
- for steps that lead somewhere

<!--raw-typst #set list(marker: [»])-->

- `#set list(marker: [»])`
- the guillemet some European typesetters use

<!--raw-typst #set list(marker: text(fill: rgb("#a3155f"), size: 0.75em)[■])-->

- `#set list(marker: text(fill: rgb("#a3155f"), size: 0.75em)[■])`
- a smaller square, in a color of its own

A marker can also be drawn, which needs no font at all. A comment may span several lines:

```
<!--raw-typst
#set list(marker: box(width: 0.42em, height: 0.42em,
  radius: 1pt, baseline: -0.16em, fill: rgb("#1849a8")))
-->
```

<!--raw-typst
#set list(marker: box(width: 0.42em, height: 0.42em,
  radius: 1pt, baseline: -0.16em, fill: rgb("#1849a8")))
-->

- a rounded square
- in blue

<!--raw-typst #set list(marker: ([•], [–], [∙]))-->

A list of markers gives one to each level in turn:

- `#set list(marker: ([•], [–], [∙]))`
  - the second level
    - the third
      - and the first one again

Pick characters that the body font has, so that the markers match the text. Source Serif 4 has •, ■, ◆, ∙, →, », ›, ▶, ▷, †, ‡, § and ¶, among others. A character it lacks comes from another font, and the few that no font has come out as empty boxes.

### Numbers

A numbering pattern puts the number among other characters. Its counting symbol says how to count: `1` in digits, `a` and `A` in letters, `i` and `I` in Roman numerals, `α` in Greek letters and `*` in the symbols of footnotes.

<!--raw-typst #set enum(numbering: "(a)")-->

1. `#set enum(numbering: "(a)")`
2. letters in parentheses

<!--raw-typst #set enum(numbering: "A.")-->

1. `#set enum(numbering: "A.")`
2. capital letters

<!--raw-typst #set enum(numbering: "I.")-->

1. `#set enum(numbering: "I.")`
2. Roman numerals,
3. right-aligned
4. on their dots

<!--raw-typst #set enum(numbering: "i)")-->

1. `#set enum(numbering: "i)")`
2. small Roman numerals

<!--raw-typst #set enum(numbering: "α)")-->

1. `#set enum(numbering: "α)")`
2. Greek letters

<!--raw-typst #set enum(numbering: "*")-->

1. `#set enum(numbering: "*")`
2. the dagger
3. and the double dagger

Everything else in a pattern is printed as it stands, so it can carry a word or a sign, as long as that holds no counting symbol: `"Part 1"` would not work, since its `a` counts as well.

<!--raw-typst #set enum(numbering: "Step 1:")-->

1. Download setdown.
2. Put it in a directory on the PATH.
3. Convert a Markdown file.

<!--raw-typst #set enum(numbering: "§ 1")-->

1. `#set enum(numbering: "§ 1")`
2. for rules and regulations

<!--raw-typst #set enum(numbering: "1.a.i.")-->

A pattern with more than one counting symbol numbers the levels in turn, as `"1.a.i."` does:

1. The first level
   1. The second level
      1. The third level

<!--raw-typst #set enum(numbering: "1.1.", full: true)-->

With `full: true`, as in `#set enum(numbering: "1.1.", full: true)`, nested items carry the numbers of their parents:

1. Introduction
   1. Scope
   2. Terms
2. Installation
   1. Download

A function can set the number in any way Typst can:

```
<!--raw-typst
#set enum(full: false, numbering: n => text(
  weight: "bold", fill: rgb("#1849a8"))[#n.])
-->
```

<!--raw-typst
#set enum(full: false, numbering: n => text(
  weight: "bold", fill: rgb("#1849a8"))[#n.])
-->

1. bold numbers
2. in blue

```
<!--raw-typst
#set enum(numbering: n => move(dy: -0.25em, box(
  width: 1.35em, height: 1.35em, radius: 50%, fill: luma(225),
  align(center + horizon, text(size: 0.75em)[#n]))))
-->
```

<!--raw-typst
#set enum(numbering: n => move(dy: -0.25em, box(
  width: 1.35em, height: 1.35em, radius: 50%, fill: luma(225),
  align(center + horizon, text(size: 0.75em)[#n]))))
-->

1. numbers in circles,
2. drawn by Typst,
3. for any number of items

### Within a list item

<!--raw-typst #set list(marker: [–])-->
<!--raw-typst #set enum(numbering: "1.")-->

A set rule within a list item applies to that item only:

- An item
  <!--raw-typst #set list(marker: [→])-->
  - with a marker of its own
- and one without
  - that keeps the dash

The two comments in front of this example set the markers back to setdown's defaults, for the rest of this document.

## Across pages

A list never starts or ends a page with a single item: Typst keeps the first item with the second, and the second to last with the last. Only items of up to about three lines are kept with their neighbor, since a longer one moved to the next page as a whole would leave a large gap.

> Lists in a block quote keep Typst's own layout:
>
> - one item
> - and another
