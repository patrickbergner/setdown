// Template of setdown, which passes it on stdin with the Markdown file's directory as the root.
// Inputs (all strings): md (file name below the root), name (fallback title), and the optional font, mono-font, scale, lang, paper, landscape, margin, first-margin-top, highlight, bullet, numbering, page-numbers, toc and heading-numbering, which win over the front matter.
// defaults, a JSON object, holds the user's defaults for the same keys (config file, SETDOWN_OPTIONS), which the front matter wins over.
// setdown puts a --style file in place of the line that marks its place, after the template's rules.
// It puts the --background and --first-background files in place of another such line, before the page's set rule.

#import "@preview/cmarker:0.1.10"

#let base-size = 11pt
#let fonts = (serif: "Source Serif 4", sans: "Source Sans 3", mono: "JetBrains Mono", emoji: "Noto Emoji")

// Emoji come from Noto Emoji, in black and white like the text around them. covers keeps it to the characters that
// are emoji by default and to the parts of emoji sequences (skin tones, flags, joiners, the emoji variation selector,
// keycaps, tags), so that a symbol such as ★, ✓, © or ™ stays in the text fonts; a character no other font has still
// comes from Noto Emoji through Typst's fallback.
#let emoji-font = (name: fonts.emoji, covers: regex(
    "[\p{Emoji_Presentation}\p{Emoji_Modifier}\p{Regional_Indicator}\u{200D}\u{FE0F}\u{20E3}\u{E0020}-\u{E007F}]",
))

// A checkbox for task list items (- [ ] and - [x]), drawn so it needs no glyph. It spans about the cap height.
#let task-box(checked) = {
    let s = 0.72em
    box(width: s, height: s, baseline: 0.02em, stroke: 0.045em, radius: 0.1em, if checked {
        // place() anchors the tick at the box's corner; inline, it would sit on the baseline.
        place(top + left, curve(
            stroke: (thickness: 0.08em, cap: "round", join: "round"),
            curve.move((0.2 * s, 0.5 * s)),
            curve.line((0.41 * s, 0.71 * s)),
            curve.line((0.8 * s, 0.27 * s)),
        ))
    })
}

// A light theme for --highlight, with every color at a contrast of at least 4.5:1 on the code block's background.
#let highlight-theme = bytes(```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>name</key><string>setdown</string>
<key>settings</key><array>
<dict><key>settings</key><dict><key>foreground</key><string>#1a1a1a</string></dict></dict>
<dict><key>scope</key><string>comment</string><key>settings</key><dict><key>foreground</key><string>#646464</string><key>fontStyle</key><string>italic</string></dict></dict>
<dict><key>scope</key><string>keyword, storage, keyword.operator.word, keyword.operator.logical, keyword.operator.new</string><key>settings</key><dict><key>foreground</key><string>#a3155f</string></dict></dict>
<dict><key>scope</key><string>keyword.operator, punctuation</string><key>settings</key><dict><key>foreground</key><string>#1a1a1a</string></dict></dict>
<dict><key>scope</key><string>string, markup.raw</string><key>settings</key><dict><key>foreground</key><string>#1d6b1d</string></dict></dict>
<dict><key>scope</key><string>constant, support.constant, variable.other.constant</string><key>settings</key><dict><key>foreground</key><string>#9a3c00</string></dict></dict>
<dict><key>scope</key><string>entity.name.function, support.function, meta.function-call entity.name, variable.function</string><key>settings</key><dict><key>foreground</key><string>#1849a8</string></dict></dict>
<dict><key>scope</key><string>entity.name.type, entity.name.class, entity.other.inherited-class, support.type, support.class, storage.type</string><key>settings</key><dict><key>foreground</key><string>#6b3fa0</string></dict></dict>
<dict><key>scope</key><string>entity.name.tag</string><key>settings</key><dict><key>foreground</key><string>#1849a8</string></dict></dict>
<dict><key>scope</key><string>entity.other.attribute-name, support.type.property-name</string><key>settings</key><dict><key>foreground</key><string>#9a3c00</string></dict></dict>
<dict><key>scope</key><string>variable.parameter, variable.other.readwrite, variable.language</string><key>settings</key><dict><key>foreground</key><string>#7a4a00</string></dict></dict>
<dict><key>scope</key><string>markup.heading, markup.bold</string><key>settings</key><dict><key>fontStyle</key><string>bold</string></dict></dict>
<dict><key>scope</key><string>markup.italic</string><key>settings</key><dict><key>fontStyle</key><string>italic</string></dict></dict>
<dict><key>scope</key><string>markup.inserted</string><key>settings</key><dict><key>foreground</key><string>#1d6b1d</string></dict></dict>
<dict><key>scope</key><string>markup.deleted, invalid</string><key>settings</key><dict><key>foreground</key><string>#b00020</string></dict></dict>
</array>
</dict></plist>
```.text)

// Markdown paths are relative to the Markdown file, which is the root. cmarker evaluates the code of raw-typst
// comments in its own package, where a path, even one starting with a slash, would name a file of the package. So
// image and read come from here; other functions that take a path take read's bytes as well, such as pdf.attach.
#let root-path(source) = if type(source) == str and not source.starts-with("/") { "/" + source } else { source }

#let (meta, body) = cmarker.render-with-metadata(
    read("/" + sys.inputs.md),
    metadata-block: "frontmatter-yaml",
    task-list-marker: task-box,
    scope: (
        image: (source, ..args) => image(root-path(source), ..args),
        read: (path, ..args) => read(root-path(path), ..args),
    ),
)
#let meta = if type(meta) == dictionary { meta } else { (:) }
// An option wins over the front matter, which wins over the user's defaults (strings, or a list of strings for
// bullet), which win over the template's own.
#let user-defaults = json(bytes(sys.inputs.at("defaults", default: "{}")))
#let setting(key, default) = sys.inputs.at(key, default: meta.at(key, default: user-defaults.at(key, default: default)))

#let parse-scale(value) = {
    let factor = if type(value) == str {
        let v = value.trim()
        if v.ends-with("%") { float(v.slice(0, -1)) / 100 } else { float(v) }
    } else { float(value) }
    assert(factor > 0.2 and factor < 5, message: "scale must be between 0.2 and 5, got " + repr(value))
    factor
}

// A margin as in CSS: one length for every side; two for top and bottom, then left and right; three for top, left
// and right, then bottom; four for top, right, bottom and left.
#let parse-margin(value) = {
    let message = "margin must be a length such as 2cm or 20mm, or two to four for the sides as in CSS, such as "
    let parts = if type(value) == length { (value,) } else { str(value).split() }
    assert(parts.len() in (1, 2, 3, 4), message: message + "30mm 20mm, got " + repr(value))
    let lengths = parts.map(v => if type(v) == length { v } else {
        assert(v.match(regex("^\d+(\.\d+)?(mm|cm|in|pt)$")) != none, message: message + "30mm 20mm, got " + repr(value))
        eval(v)
    })
    let (top, right, bottom, left) = if lengths.len() == 1 { lengths * 4 } else if lengths.len() == 2 {
        lengths * 2
    } else if lengths.len() == 3 { (..lengths, lengths.at(1)) } else { lengths }
    (top: top, right: right, bottom: bottom, left: left)
}

#let parse-bool(key, value) = {
    if type(value) == bool { return value }
    let v = lower(str(value))
    assert(v in ("true", "false", "yes", "no", "on", "off"), message: key + " must be true or false, got " + repr(value))
    v in ("true", "yes", "on")
}

// serif and sans name the bundled families; anything else is a family name, from --font-path or built into Typst.
#let font-name(value) = {
    let v = str(value).trim()
    assert(v != "", message: "a font must be serif, sans or the name of a family, got " + repr(value))
    v
}
#let body-font = {
    let v = font-name(setting("font", "serif"))
    if lower(v) in ("serif", "sans") { fonts.at(lower(v)) } else { v }
}
#let mono-font = font-name(setting("mono-font", fonts.mono))
#let (lang, region) = {
    let parts = str(setting("lang", "en")).split("-")
    (lower(parts.at(0)), parts.at(1, default: none))
}
#let title = meta.at("title", default: none)
#let author = meta.at("author", default: none)
#let date = meta.at("date", default: none)
#let authors = if author == none { () } else if type(author) == array { author.map(str) } else { (str(author),) }

#let page-margin = parse-margin(setting("margin", "20mm"))
// The first page's top margin, which Typst cannot set for one page alone: the body begins with space to make up the
// difference to the other pages' top margin.
#let first-margin-top = {
    let value = setting("first-margin-top", none)
    if value == none { none } else {
        let message = "first-margin-top must be a length such as 60mm, got " + repr(value)
        let v = if type(value) == length { value } else {
            assert(str(value).trim().match(regex("^\d+(\.\d+)?(mm|cm|in|pt)$")) != none, message: message)
            eval(str(value).trim())
        }
        assert(v >= page-margin.top, message: "first-margin-top must be at least the other pages' top margin, "
            + str(calc.round(page-margin.top.mm(), digits: 2)) + "mm, got " + str(value))
        v
    }
}

// The files of --background and --first-background, as dictionaries of their data and format, or none. setdown puts
// them in place of the marker line, as one line, so the line numbers of the template stay.
#let backgrounds = (first: none, rest: none)
// setdown: --background

// Page numbers by example: the first number in the setting stands for the page, a second one for the number of
// pages, and the rest is printed as it stands, so "1 / 5", "1 of 5" and "Page 1 of 5" all work. Typst's own
// numbering patterns would count the a in "Page" as well. none or false turns the numbers off.
#let page-format = {
    let value = setting("page-numbers", "1 / 5")
    if value == false or value == none or lower(str(value)) in ("none", "false", "off", "no") { none } else {
        let format = if value == true { "1 / 5" } else { str(value) }
        let numbers = format.matches(regex("\d+"))
        assert(numbers.len() in (1, 2), message: "page-numbers must be none or show a number for the page, and maybe "
            + "one for the number of pages, as in \"1 / 5\" or \"Page 1 of 5\", got " + repr(format))
        (format: format, numbers: numbers)
    }
}
#let page-label(page, pages) = {
    let (format, numbers) = page-format
    let first = numbers.first()
    let out = format.slice(0, first.start) + str(page)
    if numbers.len() == 1 { return out + format.slice(first.end) }
    let second = numbers.last()
    out + format.slice(first.end, second.start) + str(pages) + format.slice(second.end)
}

#set document(title: if title != none { str(title) } else { sys.inputs.name }, author: authors)
#set page(
    paper: str(setting("paper", "a4")),
    flipped: parse-bool("landscape", setting("landscape", false)),
    margin: page-margin,
    // At the page's width from its top, as tall as the file makes it, so it can cover the page or a strip at the top.
    // here().page() counts the pages as they are, whatever a style does to the page counter. Without a background
    // none, as an empty one would still leave an empty artifact on each page of a tagged PDF.
    background: if backgrounds.first != none or backgrounds.rest != none {
        context {
            let bg = if here().page() == 1 { backgrounds.first } else { backgrounds.rest }
            if bg != none { place(top + left, image(bg.data, format: bg.format, width: 100%)) }
        }
    },
    // Only from two pages on.
    footer: context {
        let pages = counter(page).final().first()
        if page-format != none and pages > 1 {
            align(right, text(size: 0.85em, page-label(counter(page).get().first(), pages)))
        }
    },
)
#let body-size = base-size * parse-scale(setting("scale", 1))
#set text(font: (body-font, emoji-font), size: body-size, lang: lang, region: region)
// Sequences that the covers above would split between fonts come from Noto Emoji as a whole: a keycap (1️⃣), whose
// digit is no emoji of its own; a symbol that is text by default, asked by the emoji variation selector to look like
// an emoji (❤️, ✔️); and emoji joined into one (🧑‍⚕️, 🏳️‍🌈), whose parts may be such symbols. A symbol without the
// selector (©, ✔) stays text.
#show regex({
    let part = "\p{Extended_Pictographic}[\u{FE0F}\p{Emoji_Modifier}\u{E0020}-\u{E007F}]*"
    let joined = "(?:\u{200D}" + part + ")"
    (
        "[0-9#*]\u{FE0F}?\u{20E3}",
        "\p{Extended_Pictographic}\p{Emoji_Modifier}?\u{FE0F}" + joined + "*",
        part + joined + "+",
    ).join("|")
}): set text(font: fonts.emoji)
#set par(justify: true, leading: 0.75em, spacing: 1.25em)

// cmarker's smart punctuation produces English quotes; Typst's smart quotes follow the language.
// German low quotes typed in the source are included, so Typst knows which quote they open.
#show regex("[“”„]"): smartquote(double: true)
#show regex("[‘’‚]"): smartquote(double: false)

// A section sign or a legal abbreviation stays on the line of the number that follows (§ 13 Abs. 3 Nr. 1).
#show regex("(§§?|Abs\.|Nr\.|Art\.) \d"): it => it.text.replace(" ", sym.space.nobreak)

// Spacing in Typst: between two blocks the larger of below and above applies; after a block, a paragraph keeps the
// block's below, and before one it gets its above. Values in em refer to the body size unless set inside a heading.

// Size, space above and space below per heading level, in multiples of the body size; level 4 also serves 5 and 6
// and a paragraph that is only bold text, which Markdown documents often use as a heading.
#let heading-styles = (
    (size: 1.75, above: 2.8, below: 1.6),
    (size: 1.4, above: 2.6, below: 1.45),
    (size: 1.15, above: 2.2, below: 1.3),
    (size: 1.0, above: 1.8, below: 1.05),
)
// The front matter's title is a level 1 heading, for PDF/UA and screen readers, but keeps a look of its own. Typst
// sets a heading's text larger and bold by itself, so the body size comes back first: the em of the size and of the
// paragraph's spacing then mean what they did before the title was a heading.
#let title-text(body) = {
    set text(size: body-size, weight: "regular")
    par(justify: false, text(size: 1.8em, weight: "semibold", hyphenate: false, body))
}
#show heading: it => {
    if it.at("label", default: none) == <setdown-title> { return title-text(it.body) }
    let style = heading-styles.at(calc.min(it.level, heading-styles.len()) - 1)
    set text(size: style.size * body-size, weight: "semibold", hyphenate: false)
    set par(justify: false)
    set block(above: style.above * body-size, below: style.below * body-size)
    it
}

// Lists: an en dash at every level and a little more room between items than between lines. A list sits closer to
// the sentence that introduces it than to what follows; inside a list, nested lists keep the item spacing. A list
// whose items hold nested lists separates its items more, so each group reads as one.
// The markers are Typst's own list.marker and enum.numbering, set from the bullet and numbering settings, so a set
// rule in a raw-typst comment changes them for the lists after it: <!--raw-typst #set enum(numbering: "a)")-->.
#let bullet = setting("bullet", none)
#let enum-numbering = setting("numbering", "1.")
#assert(type(enum-numbering) == str, message: "numbering must be a pattern in quotes, such as \"1.\" or \"a)\", got "
    + repr(enum-numbering))
#let item-spacing = 0.95em
#let group-spacing = 1.4em
#let list-spacing = (above: 1em, below: 1.5em)
#let nested-list-spacing = (above: item-spacing, below: item-spacing)
#set list(
    // A list of bullets serves the nesting levels in turn, as in Typst.
    marker: if bullet == none { [–] } else if type(bullet) == array { bullet.map(b => [#b]) } else { [#bullet] },
    body-indent: 0.75em,
    spacing: item-spacing,
)
#set enum(numbering: enum-numbering, body-indent: 0.6em, spacing: item-spacing)
// The spacing serves lists the template does not lay out itself (in quotes or tables).
#show selector.or(list, enum): set block(above: list-spacing.above, below: list-spacing.below)
#show selector.or(list, enum): it => {
    show selector.or(list, enum): set block(above: nested-list-spacing.above, below: nested-list-spacing.below)
    it
}

// Keep-with-next for the Markdown body, applied before layout, since Typst keeps only headings with what follows:
// - a paragraph that is only bold text becomes a block styled as a level 4 heading;
// - a paragraph ending in a colon stays with the list or table it introduces;
// - in a list item, the text before a nested list stays with that list;
// - a list never starts or ends a page with a single item (see layout-list).
// Such a block directly below a heading takes the heading's space below.
// A set rule in a raw-typst comment wraps everything after it, which is laid out the same way under its styles.
#let is-styled(c) = repr(c.func()) == "styled"
#let plain(c) = if c.has("text") { c.text } else if c.has("body") { plain(c.body) } else if c.has("child") {
    plain(c.child)
} else if c.has("children") { c.children.map(plain).join() } else { "" }
#let is-space(c) = c.func() == [ ].func()
#let is-block(c) = c.func() in (parbreak, heading, list, enum, table, quote, figure, block, divider, pagebreak) or (
    c.func() == raw and c.block
) or is-styled(c)
#let children-of(c) = if c.func() == [].func() { c.children } else { (c,) }
#let has-list(c) = c.func() in (list, enum) or (is-styled(c) and has-list(c.child)) or (
    c.has("children") and c.children.any(has-list)
)
#let has-nested(item) = has-list(item.body)

// The first block among children from index i on, looking past spaces and paragraph breaks and into styled content.
#let next-block(children, i) = {
    for c in children.slice(i) {
        if is-space(c) or c.func() == parbreak { continue }
        if is-styled(c) { return next-block(children-of(c.child), 0) }
        return c
    }
    none
}

// The label of item number n at enum nesting depth k (0 at the top) under a numbering pattern, as Typst's enum makes
// it: the pattern's first prefix, its k-th counting symbol (the last one at deeper levels) and its suffix, so that
// "1.a.i." numbers the levels 1., a. and i. The counting symbols are Typst 0.15's.
#let counting-symbols = "1aAiIαΑ一壹あいアイא가ㄱ*١۱१১ক①⓵"
#let nth-label(pattern, k, n) = {
    let pieces = ()
    let pending = ""
    for c in pattern.codepoints() {
        if c in counting-symbols {
            pieces.push((pending, c))
            pending = ""
        } else {
            pending += c
        }
    }
    pieces.first().at(0) + numbering(pieces.at(calc.min(k, pieces.len() - 1)).at(1), n) + pending
}
#let enum-label(pattern, full, parents, n) = if type(pattern) == function {
    if full { pattern(..parents, n) } else { pattern(n) }
} else if full { numbering(pattern, ..parents, n) } else { nth-label(pattern, parents.len(), n) }
#let list-label(marker, depth) = if type(marker) == array { marker.at(calc.rem(depth, marker.len())) } else if type(
    marker,
) == function { marker(depth) } else { marker }

// Lays out a list as one block per item with the marker set beside it. Typst's own list items cannot stick to each
// other, these blocks can: the first item stays with the second and the second to last with the last. Only short
// items stick (about three lines, estimated from the text length), because Typst moves a sticky block that does not
// fit as a whole and would leave a large gap. The markers come from list.marker and enum.numbering where the list
// is, so set rules before it apply; depth holds the list nesting level and the numbers of the enclosing enum items.
// process(body, marker, depth) prepares an item's body and puts the marker into its first line or lead-in block, so
// the marker moves with them when they go to the next page. A list with its own marker (a task list, whose
// checkboxes cmarker draws as markers) keeps Typst's layout.
#let layout-list(it, in-item, depth, process) = {
    let numbered = it.func() == enum
    let start = it.at("start", default: 1)
    let numbers = it.children.enumerate().map(((i, item)) => {
        if item.at("number", default: none) != none { item.number } else { start + i }
    })
    let inner(i) = if numbered { (..depth, enum: (..depth.enum, numbers.at(i))) } else {
        (..depth, list: depth.list + 1)
    }
    if it.has("marker") {
        let fields = it.fields()
        let items = fields.remove("children").enumerate().map(((i, item)) => item.func()(process(item.body, none, inner(i))))
        return it.func()(..fields, ..items)
    }
    context {
        let labels = if numbered {
            let pattern = it.at("numbering", default: enum.numbering)
            numbers.map(n => enum-label(pattern, it.at("full", default: enum.full), depth.enum, n))
        } else {
            it.children.map(_ => list-label(list.marker, depth.list))
        }
        let column = calc.max(..labels.map(l => measure(l).width))
        let gap = if numbered { 0.6em } else { 0.75em }
        let spacing = if it.children.any(has-nested) { group-spacing } else { item-spacing }
        let line-chars = (page.width - page-margin.left - page-margin.right) / (0.5 * text.size)
        let short(i) = not has-nested(it.children.at(i)) and plain(it.children.at(i).body).len() <= 3 * line-chars
        let n = it.children.len()
        let items = it.children.enumerate().map(((i, item)) => {
            let sticky = ((i == 0 and n >= 3) or (i == n - 2)) and short(i)
            let marker = place(top + left, dx: -column - gap, box(width: column, align(
                if numbered { right } else { left },
                labels.at(i),
            )))
            block(above: spacing, below: spacing, sticky: sticky, inset: (left: column + gap), width: 100%, process(
                item.body,
                marker,
                inner(i),
            ))
        })
        let outer = if in-item { nested-list-spacing } else { list-spacing }
        block(above: outer.above, below: outer.below, width: 100%, items.join())
    }
}

// marker: a list item's marker, put into the first paragraph or block of the body. depth: see layout-list.
#let keep-with-next(body, in-item: false, marker: none, depth: (list: 0, enum: ()), after-heading: false) = {
    let children = children-of(body)
    let out = ()
    let run = ()
    let lead = marker
    // after-heading: whether the next block follows a heading (or a bold line used as one) directly.
    for (i, c) in (..children, none).enumerate() {
        if c != none and not is-block(c) {
            run.push(c)
            continue
        }
        let words = run.filter(r => not is-space(r))
        let following = if c == none { none } else { next-block(children, i) }
        if words.len() == 1 and words.first().func() == strong and not in-item and c != none {
            let style = heading-styles.last()
            let above = if after-heading { 0pt } else { style.above * body-size }
            out.push(block(sticky: true, above: above, below: style.below * body-size, par(justify: false, run.join())))
        } else if words.len() > 0 and following != none and following.func() in (list, enum, table) and (
            in-item or plain(run.join()).trim().ends-with(":")
        ) {
            let above = if in-item or after-heading { 0pt } else { par.spacing }
            out.push(block(sticky: true, above: above, below: 0pt, { lead; run.join() }))
            lead = none
        } else if words.len() > 0 {
            out += (lead, ..run)
            lead = none
        } else {
            out += run
        }
        if words.len() > 0 { after-heading = words.len() == 1 and words.first().func() == strong and not in-item }
        run = ()
        if c == none { break }
        if c.func() == parbreak {
            out.push(c)
            continue
        }
        if is-styled(c) {
            out.push((c.func())(keep-with-next(c.child, in-item: in-item, marker: lead, depth: depth, after-heading:
                after-heading), c.styles))
            lead = none
            continue
        }
        // A body that starts with a block gets the marker just before it.
        out.push(lead)
        lead = none
        if c.func() in (list, enum) {
            out.push(layout-list(c, in-item, depth, (b, m, d) => keep-with-next(b, in-item: true, marker: m, depth: d)))
        } else {
            out.push(c)
        }
        after-heading = c.func() == heading
    }
    out.push(lead)
    out.join()
}

#set raw(theme: if parse-bool("highlight", setting("highlight", false)) { highlight-theme } else { none })
#show raw.where(block: true): set block(fill: luma(246), inset: (x: 0.8em, y: 0.7em), radius: 2pt, width: 100%)
#show raw.where(block: true): set par(justify: false)

// Spaces in inline code keep their width in justified lines: that of an x, which is every glyph's in a monospaced
// font, measured where the space stands so it holds for any mono-font. The zero-width space still allows a break.
// After = and &, long parameter strings may break as well.
#show raw.where(block: false): it => {
    show " ": context box(width: measure("x").width) + sym.zws
    show regex("[=&]"): it => it + sym.zws
    it
}

// The bar reaches half a line (cap height plus leading) above and below the text; outset keeps the text in place.
// The spacing is explicit, since the quote's own block would add a large gap above.
#show quote.where(block: true): it => block(above: 1.6em, below: 1.6em, stroke: (left: 1.5pt + luma(190)), inset: (left: 1em), outset: (y: 0.7em), it.body)

#set table(inset: (x: 0.6em, y: 0.7em), stroke: (_, y) => if y == 0 { (bottom: 0.6pt) } else { (bottom: 0.3pt + luma(190)) })
#show table: set par(justify: false)
#show table: set block(above: 1em, below: 1.5em)
#show table.cell.where(y: 0): set text(weight: "semibold")

#show link: it => if type(it.dest) == str { underline(stroke: 0.4pt, offset: 0.18em, it) } else { it }

// No programming ligatures (-> as an arrow, != as ≠): printed code should show the characters to type.
#show raw: set text(font: (mono-font, emoji-font), features: (calt: 0))

// Heading numbers: a Typst numbering pattern such as "1.1" or "I.a", or none. YAML reads an unquoted 1.1 as a number,
// which stands for the same pattern, and 1 or 1. as a whole number, which stands for "1." (the pattern "1" would
// number a subsection 11).
#let heading-pattern = {
    let value = setting("heading-numbering", none)
    if type(value) == int or (type(value) == float and calc.fract(value) == 0) {
        value = str(int(value)) + "."
    } else if type(value) == float { value = str(value) }
    if value == none or value == false or (type(value) == str and lower(value.trim()) in ("none", "false", "off", "no", "")) {
        none
    } else {
        let message = "heading-numbering must be none or a numbering pattern such as \"1.1\" or \"I.a\", got "
        assert(type(value) == str and value.codepoints().any(c => c in counting-symbols), message: message + repr(value))
        value.trim()
    }
}
// The numbers start at the level of the document's top headings: below a front matter title, a document often starts
// its sections at ##, which would otherwise be numbered 0.1, 0.2 and so on.
// Before layout, a heading knows its depth (cmarker sets it) and offset, not yet its level.
#let heading-levels(c) = if c.func() == heading {
    (if c.has("level") { c.level } else { c.at("depth", default: 1) + c.at("offset", default: 0) },)
} else if is-styled(c) { heading-levels(c.child) } else if (
    c.func() == [].func()
) { c.children.map(heading-levels).flatten() } else { () }
#let top-level = calc.min(6, ..heading-levels(body))
#set heading(numbering: if heading-pattern != none {
    (..numbers) => numbering(heading-pattern, ..numbers.pos().slice(calc.min(top-level, numbers.pos().len()) - 1))
})

// The table of contents, below the title: true for the headings of levels 1 to 3, a number for that many levels.
#let toc-depth = {
    let value = setting("toc", false)
    let message = "toc must be true, false or the number of heading levels from 1 to 6, got " + repr(value)
    if type(value) == str {
        let v = lower(value.trim())
        value = if v.match(regex("^\d+$")) != none { int(v) } else {
            assert(v in ("true", "false", "yes", "no", "on", "off"), message: message)
            v in ("true", "yes", "on")
        }
    }
    if value == none or value == false { none } else if value == true { 3 } else {
        assert(type(value) == int and value >= 1 and value <= 6, message: message)
        value
    }
}
// Entries of the first level stand apart a little and in the headings' weight.
#show outline.entry.where(level: 1): set block(above: 1.1em)
#show outline.entry.where(level: 1): set text(weight: "semibold")

// A --style file comes here, so its set and show rules win over the template's and apply to the title as well.
// It can use what the template defines, such as meta (the front matter) and setting(key, default).
// setdown: --style

// Weak spacing would vanish at the top of the page.
#if first-margin-top != none and first-margin-top > page-margin.top {
    v(first-margin-top - page-margin.top, weak: false)
}

#if title != none {
    block(below: 1.8em, {
        // Outside the outline, the bookmarks and the heading numbers; the template's heading rule draws it.
        [#heading(level: 1, outlined: false, bookmarked: false, numbering: none, str(title))<setdown-title>]
        let byline = (..authors, ..if date != none { (str(date),) } else { () })
        if byline.len() > 0 { v(-0.4em); text(size: 1.05em, byline.join(" · ")) }
    })
}

// Its title is a level 1 heading in the language of the document, outside the outline and the bookmarks.
#if toc-depth != none {
    block(below: 2em, outline(depth: toc-depth))
}

// The code font at the size where its x-height matches the body font's. The factor is measured on raw itself,
// because Typst scales raw text by a built-in factor (0.8em) on top of any size set here. The x-height is that of the
// outline of an x, not the font's metric, which can be wrong: for DejaVu Sans Mono, built into Typst, it is the cap
// height (0.76em, the outline gives 0.55em), which set code a third too small. For the bundled fonts both agree.
#context {
    let x-height(it) = measure(text(top-edge: "bounds", bottom-edge: "baseline", it)).height
    show raw: set text(size: x-height[x] / x-height(raw("x")) * 1em)
    keep-with-next(body)
}
