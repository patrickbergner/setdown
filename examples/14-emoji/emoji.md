---
title: Emoji
date: 2000-01-01
---

READMEs, notes and chat logs are full of emoji 👍. setdown sets them in Noto Emoji, which it bundles: a font in black and white, so emoji print like the letters around them, in the text's size and color, and look the same in every PDF viewer and on every printer.

## In text, headings and lists 🚀

Emoji sit in the line like letters, in **bold 💪** and *italic 🍕* text, in headings such as the one above, in `inline code ✨` and in links: [the README 📖](https://github.com/patrickbergner/setdown).

- 🐛 Fixed: the login form forgot the user name.
- ✨ New: a dark mode.
- 🔥 Removed: the old export.

## Status tables

| Check   | Linux | macOS | Windows |
|---------|:-----:|:-----:|:-------:|
| Build   |  ✅   |  ✅   |   ✅    |
| Tests   |  ✅   |  ⚠️   |   ❌    |
| Release |  🚧   |  🚧   |   🚧    |

## Sequences

Many emoji are made of several characters, which the font joins into one sign:

| Made of                                  | Emoji             |
|------------------------------------------|-------------------|
| emoji joined by zero-width joiners       | 👩‍💻 👨‍👩‍👧 🏳️‍🌈     |
| an emoji and a skin tone                 | 👋 👋🏻 👋🏽 👋🏿       |
| two regional letters, for a flag         | 🇩🇪 🇫🇷 🇯🇵 🇺🇸       |
| a digit, # or * and the keycap           | 1️⃣ 2️⃣ #️⃣          |
| a symbol and the emoji variation selector | ❤️ ⚠️ ☀️ ✔️        |

The last row's symbols have a plain form as well, without the selector, which emoji pickers and phone keyboards add.

## Code

Code blocks and inline code take emoji too, though an emoji is wider than the code font's characters:

```bash
git commit -m "🐛 Fix the login form"
git tag v1.2.0 -m "🚀 Release 1.2.0"
```

## Symbols that are no emoji

A character that is a symbol of the text rather than an emoji stays in the text fonts, as it does without Noto Emoji: ★ ✓ ✗ → ↔ © ® ™ § ¶ †. Only characters that are emoji by default, and those asked to look like one, come from Noto Emoji.

## Black and white

Emoji take the color of their text, <!--raw-typst #text(fill: rgb("#a3155f"))[like these 🌸 🎈 🍓]-->, and a style file that colors the headings colors their emoji as well. Color emoji fonts hold a picture of every emoji, which would add megabytes to the program and to every PDF.

Shortcodes such as `:smile:` stay as they are written, since Markdown knows no emoji of its own; type the emoji itself or paste it from an emoji picker (Windows key and period on Windows, Control, Command and Space on macOS).
