---
title: Code
date: 2000-01-01
highlight: true
---

Code is set in JetBrains Mono, scaled so that its x-height matches that of the body font. This document turns highlighting on in its front matter, which `--highlight` does on the command line.

## Inline code

Inline code such as `git status` sits in the line without disturbing it. Spaces in it keep their width, also in a justified line: `a  b`, with two spaces, stays two spaces wide. Long strings may break after `=` and `&`, as in `https://example.com/search?query=typography&lang=en&page=2&sort=date`.

Programming ligatures are off, so code prints the characters to type: `->`, `=>`, `!=`, `===`, `<=`, `>=`, `|>`, `::`, `www`.

## Code blocks

A code block without a language stays black on light gray:

```
setdown report.md
setdown --sans --scale 90% report.md out/report.pdf
```

With highlighting, a language after the opening backticks colors the block. The theme is setdown's own; every one of its colors reaches a contrast of at least 4.5:1 against the background.

```python
from pathlib import Path


def convert_all(directory: Path) -> int:
    """Convert the Markdown files below directory, return how many."""
    count = 0
    for markdown in sorted(directory.rglob("*.md")):
        print(f"converting {markdown}")  # setdown does the work
        count += 1
    return count
```

```go
// pageCount reads the page count from a PDF that Typst wrote.
func pageCount(pdf []byte) (int, error) {
	m := regexp.MustCompile(`/Type */Pages[^>]*/Count ([0-9]+)`).FindSubmatch(pdf)
	if m == nil {
		return 0, errors.New("no page tree")
	}
	return strconv.Atoi(string(m[1]))
}
```

```rust
fn main() {
    let words = ["justified", "hyphenated", "typeset"];
    for (i, word) in words.iter().enumerate() {
        println!("{}: {word}", i + 1);
    }
}
```

```typescript
interface Options {
  sans?: boolean;
  scale?: number;
  paper: "a4" | "a5" | "us-letter";
}

const defaults: Options = { paper: "a4" };
export const merge = (o: Partial<Options>): Options => ({ ...defaults, ...o });
```

```json
{
  "title": "Release Notes",
  "authors": ["Jane Doe"],
  "pages": 3,
  "draft": false
}
```

```yaml
title: Release Notes
author: Jane Doe
scale: 0.95   # 95 % of 11 pt
highlight: true
```

```sql
SELECT title, COUNT(*) AS pages
FROM documents
WHERE created >= '2000-01-01'
GROUP BY title
ORDER BY pages DESC;
```

```html
<p class="note">Rendered with <a href="https://typst.app/">Typst</a>.</p>
```

```diff
- setdown --update report.md
+ setdown report.md
```

```bash
#!/usr/bin/env bash
set -euo pipefail
for f in *.md; do
    setdown --highlight "$f" || echo "failed: $f" >&2
done
```
