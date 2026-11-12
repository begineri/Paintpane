"""Builds the subset of Noto Sans SC that Paintpane bundles (ui/fonts/).

Run from the repository root, with fonttools installed (pip install fonttools):

    python assets-src/fonts/subset.py

It downloads the two source fonts from the official notofonts repository if
they are missing (pinned commit, checked by SHA-256), then keeps:

- every character in the app's translations and curated collection files,
- the 6,763 Chinese characters of GB2312 and the 5,401 of Big5 level 1, so
  most other Chinese text (folder names, titles) also renders in this font,
- Latin, Greek, Cyrillic, kana, CJK and full-width punctuation, and symbols.

Anything else falls back to Fyne's built-in font or a Windows font.
See README.md in this folder for the source, version and licence.
"""

import glob
import hashlib
import json
import os
import sys
import urllib.request

from fontTools import subset

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
OUT = os.path.join(ROOT, "ui", "fonts")

COMMIT = "165c01b46ea533872e002e0785ff17e44f6d97d8"  # notofonts/noto-cjk, Noto Sans CJK 2.004
SOURCES = {
    "NotoSansSC-Regular.otf": "faa6c9df652116dde789d351359f3d7e5d2285a2b2a1f04a2d7244df706d5ea9",
    "NotoSansSC-Bold.otf": "c6cb5a93abaa9edc8ee7463b7ebb7f42d618d40e6ed2f7a5371c97b0b64767c0",
}
URL = "https://raw.githubusercontent.com/notofonts/noto-cjk/" + COMMIT + "/Sans/SubsetOTF/SC/"

RANGES = [
    (0x0020, 0x007E),  # ASCII
    (0x00A0, 0x017F),  # Latin-1, Latin Extended-A
    (0x0370, 0x03FF),  # Greek
    (0x0400, 0x04FF),  # Cyrillic
    (0x2000, 0x206F),  # General punctuation
    (0x20A0, 0x20CF),  # Currency
    (0x2100, 0x214F),  # Letterlike symbols
    (0x2190, 0x21FF),  # Arrows
    (0x2200, 0x22FF),  # Mathematical operators
    (0x2460, 0x24FF),  # Enclosed alphanumerics
    (0x25A0, 0x25FF),  # Geometric shapes
    (0x2600, 0x26FF),  # Miscellaneous symbols (★)
    (0x3000, 0x303F),  # CJK symbols and punctuation
    (0x3040, 0x30FF),  # Hiragana, Katakana
    (0xFF00, 0xFFEF),  # Full-width forms
]

# Files whose text the app shows: translations and curated collections.
TEXT_FILES = [
    "pkg/i18n/translations/*.json",
    "docs/collections/*.json",
    "asset/text/*.json",
]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def fetch_sources():
    for name, digest in SOURCES.items():
        path = os.path.join(HERE, name)
        if not os.path.exists(path):
            print("downloading", URL + name)
            urllib.request.urlretrieve(URL + name, path)
        if sha256(path) != digest:
            sys.exit(name + ": SHA-256 does not match; delete it and run again")


def legacy_hanzi(codec, lead, trail):
    chars = set()
    for hi in range(lead[0], lead[1] + 1):
        for lo in range(trail[0], trail[1] + 1):
            try:
                chars.add(bytes([hi, lo]).decode(codec))
            except UnicodeDecodeError:
                pass
    return chars


def text_chars():
    chars = set()

    def walk(v):
        if isinstance(v, str):
            chars.update(v)
        elif isinstance(v, dict):
            for k, x in v.items():
                walk(k)
                walk(x)
        elif isinstance(v, list):
            for x in v:
                walk(x)

    for pattern in TEXT_FILES:
        for path in sorted(glob.glob(os.path.join(ROOT, pattern))):
            with open(path, encoding="utf-8") as f:
                walk(json.load(f))
    return chars


def main():
    fetch_sources()
    chars = text_chars()
    chars |= legacy_hanzi("gb2312", (0xB0, 0xF7), (0xA1, 0xFE))  # GB2312 level 1 and 2
    for hi in range(0xA4, 0xC7):  # Big5 level 1: A440-C67E
        for lo in list(range(0x40, 0x7F)) + list(range(0xA1, 0xFF)):
            if (hi, lo) > (0xC6, 0x7E):
                continue
            try:
                chars.add(bytes([hi, lo]).decode("big5"))
            except UnicodeDecodeError:
                pass
    for lo, hi in RANGES:
        chars.update(chr(c) for c in range(lo, hi + 1))
    unicodes = sorted(ord(c) for c in chars if c.isprintable() or c == "　")

    os.makedirs(OUT, exist_ok=True)
    for name in SOURCES:
        options = subset.Options()
        options.layout_features = ["*"]
        options.name_IDs = ["*"]  # keep the copyright and licence entries
        options.name_languages = ["*"]
        options.notdef_outline = True
        options.hinting = False
        font = subset.load_font(os.path.join(HERE, name), options)
        sub = subset.Subsetter(options)
        sub.populate(unicodes=unicodes)
        sub.subset(font)
        out = os.path.join(OUT, name.replace(".otf", ".subset.otf"))
        subset.save_font(font, out, options)
        print(f"{os.path.relpath(out, ROOT)}: {len(font.getGlyphOrder())} glyphs, {os.path.getsize(out):,} bytes")
    print(f"{len(unicodes)} characters")


if __name__ == "__main__":
    main()
