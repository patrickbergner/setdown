// The style of the fonts-and-style example: Typst rules that setdown
// applies after its template's.
#let accent = rgb("#a3155f")

// Headings in the bundled sans and the accent color; the body font comes
// from the front matter.
#show heading: set text(font: "Source Sans 3", fill: accent)
#show link: set text(fill: accent)

// A header with a logo and a front matter key of the document's own.
// The path is relative to the Markdown file.
#set page(header: grid(
    columns: (1fr, auto),
    align: horizon,
    image("logo.svg", height: 6mm),
    text(size: 0.85em, fill: luma(90), meta.at("client", default: "")),
))
