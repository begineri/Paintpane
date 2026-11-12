# Fonts

Paintpane draws all of its own text (settings window, Tune Image) in **Noto Sans SC**, Regular and
Bold, so Chinese and Latin share one design and Chinese has a real bold. The app bundles a subset, not the full fonts.

| | |
| :--- | :--- |
| Font | Noto Sans SC (Noto Sans CJK, region subset SC), Version 2.004 |
| Source | https://github.com/notofonts/noto-cjk, commit `165c01b46ea533872e002e0785ff17e44f6d97d8`, files `Sans/SubsetOTF/SC/NotoSansSC-Regular.otf` and `NotoSansSC-Bold.otf` |
| SHA-256 | Regular `faa6c9df652116dde789d351359f3d7e5d2285a2b2a1f04a2d7244df706d5ea9`<br>Bold `c6cb5a93abaa9edc8ee7463b7ebb7f42d618d40e6ed2f7a5371c97b0b64767c0` |
| Copyright | © 2014-2021 Adobe (http://www.adobe.com/). Noto is a trademark of Google Inc. |
| Licence | SIL Open Font License 1.1, no Reserved Font Name; full text in [`ui/fonts/OFL.txt`](../../ui/fonts/OFL.txt) |

The source fonts (8.3 MB and 8.5 MB) are not in git (`.gitignore`); `subset.py` downloads them
into this folder and checks their SHA-256.

## Rebuild the subset

```bash
pip install fonttools
python assets-src/fonts/subset.py
```

It writes `ui/fonts/NotoSansSC-Regular.subset.otf` and `NotoSansSC-Bold.subset.otf`, which
`ui/theme.go` embeds. The subset keeps every character in the translations and curated collection
files, GB2312 (6,763 Chinese characters), Big5 level 1 (5,401), Latin, Greek, Cyrillic, kana, and
CJK and full-width punctuation: 11,126 characters, about 2.3 MB per weight. Run it again when a
translation adds characters outside these sets. Characters that are still missing fall back to
Fyne's built-in font, then to a Windows font.
