// The letterhead of the example, as a PDF the width of an A4 page: a letterhead usually comes as a PDF from whoever
// designed it. Rendered with the fonts setdown bundles:
//   SOURCE_DATE_EPOCH=946728000 typst compile --ignore-system-fonts --font-path ../../internal/payload/assets/files/fonts letterhead.typ
#set page(width: 210mm, height: 45mm, margin: 0pt, fill: none)
#set text(font: "Source Sans 3", fill: rgb("#1a2b4c"))

#place(top + left, dx: 20mm, dy: 14mm, {
    text(size: 18pt, weight: "semibold")[Northwind Type Foundry]
    linebreak()
    text(size: 9pt, fill: rgb("#4a5568"))[42 Harbor Lane · Portland, ME 04101 · hello\@northwind.example]
})
#place(top + right, dx: -20mm, dy: 14mm, rect(width: 11mm, height: 11mm, radius: 1.5mm, fill: rgb("#a3155f")))
#place(bottom + left, dx: 20mm, dy: -4mm, line(length: 170mm, stroke: 0.6pt + rgb("#a3155f")))
