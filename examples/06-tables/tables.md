---
title: Tables
date: 2000-01-01
landscape: true
---

Tables have a rule below their header row and thinner, gray rules between the other rows. The header is set in semibold. Cells are left-aligned unless their column says otherwise, and their text is not justified. This document is set on landscape paper, which gives wide tables room: `landscape: true` in its front matter, or `--landscape` on the command line.

## Alignment

The colons in the line below the header align a column: on the left, on both sides for centered, on the right.

| Left       | Centered     | Right    |
|:-----------|:------------:|---------:|
| Apples     | 3 kg         |   4.50 € |
| Pears      | 12 pieces    |  11.20 € |
| Plums      | a small box  |   2.99 € |

## Formatting in cells

| Markdown                | Result                       |
|-------------------------|------------------------------|
| `**bold**`              | **bold**                     |
| `*italic*`              | *italic*                     |
| `~~struck~~`            | ~~struck~~                   |
| `` `code` ``            | `code`                       |
| `[link](https://typst.app/)` | [link](https://typst.app/) |

A pipe inside a cell is written as `\|`.

## Lead-ins

A paragraph that ends in a colon stays on the same page as the table it introduces, as this one does:

| Quarter | Orders | Returns |
|---------|-------:|--------:|
| Q1      |  1,204 |      31 |
| Q2      |  1,388 |      27 |

## Wide tables

Landscape paper turns the page sideways and keeps the margins, so a table of many columns fits without cramped cells:

| Region        |    Q1 |    Q2 |    Q3 |    Q4 |  Total | Change | Target | Note                           |
|---------------|------:|------:|------:|------:|-------:|-------:|-------:|--------------------------------|
| North         | 1,204 | 1,388 | 1,412 | 1,630 |  5,634 |   +8 % |  5,500 | above target since Q2          |
| South         |   980 | 1,015 |   960 | 1,104 |  4,059 |   +3 % |  4,200 | new store opened in Q4         |
| East          | 1,502 | 1,470 | 1,544 | 1,601 |  6,117 |   +5 % |  6,000 |                                |
| West          |   745 |   812 |   890 |   934 |  3,381 |  +12 % |  3,200 | the fastest growth             |

## Longer text

| Option          | What it does                                                                                       |
|-----------------|----------------------------------------------------------------------------------------------------|
| `--sans`        | Sets the text in Source Sans 3 instead of Source Serif 4, headings and tables included.           |
| `--scale 90%`   | Scales the text, but not the margins, so more of it fits on a page.                               |
| `--highlight`   | Colors code blocks by the language named after their opening backticks.                           |

Long cells wrap within their column, and the columns share the width of the page.

