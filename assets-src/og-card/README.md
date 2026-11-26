# Share cards (og:image)

The pictures that link previews show when someone shares the intro page: `site/assets/img/en/og-card.jpg`
(English page) and `site/assets/img/zh/og-card.jpg` (Chinese page), 1200 × 630.

Each card shows only the painting and the app's name, on the wall colour of the S1 desktop shot (`#31322b`):
Renoir's *Two Sisters (On the Terrace)* (Art Institute of Chicago, open access), cut whole out of
`site/assets/img/s1-desktop.jpg`, and "Paintpane" or "观画壁纸 / Paintpane" in the system fonts.

To make them again (from this folder, in Git Bash):

```bash
EDGE="/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"
for L in en zh; do
  "$EDGE" --headless=new --disable-gpu --hide-scrollbars --user-data-dir="$TEMP/og-card-edge" \
    --window-size=1200,630 --screenshot="$PWD/card-$L.png" "file:///$(cygpath -m "$PWD")/card-$L.html"
  go run tojpg.go card-$L.png ../../site/assets/img/$L/og-card.jpg && rm card-$L.png
done
```

`go test ./internal/sitecheck/` checks that both pages point at a 1200 × 630 card.
