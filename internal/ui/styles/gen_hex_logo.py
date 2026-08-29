#!/usr/bin/env python3
"""Render official DERO logo PNG to truecolor half-block ANSI. Keeps alpha."""

import os
import re
import shutil
import subprocess
import sys

COLS, ROWS = 68, 37
PW, PH = COLS, ROWS * 2
SMALL_COLS, SMALL_ROWS = 48, 26
SMALL_PW, SMALL_PH = SMALL_COLS, SMALL_ROWS * 2
SRC = os.path.expanduser("~/Pictures/dero/dero_logo.png")
ALPHA_CUT = 32


def load_rgba(path, w, h):
    convert = shutil.which("magick") or shutil.which("convert")
    if not convert:
        sys.exit("need ImageMagick convert/magick")
    raw = subprocess.check_output(
        [
            convert,
            path,
            "-filter",
            "Lanczos",
            "-resize",
            "{}x{}!".format(w, h),
            "rgba:-",
        ]
    )
    need = w * h * 4
    if len(raw) < need:
        sys.exit("rgba too short: %d" % len(raw))
    pix = []
    idx = 0
    for _y in range(h):
        row = []
        for _x in range(w):
            row.append((raw[idx], raw[idx + 1], raw[idx + 2], raw[idx + 3]))
            idx += 4
        pix.append(row)
    return pix


def cell(top, bot):
    ta, ba = top[3] > ALPHA_CUT, bot[3] > ALPHA_CUT
    if not ta and not ba:
        return " "
    if ta and ba:
        return "\x1b[38;2;{};{};{}m\x1b[48;2;{};{};{}m▀\x1b[0m".format(
            top[0], top[1], top[2], bot[0], bot[1], bot[2]
        )
    if ta:
        return "\x1b[38;2;{};{};{}m▀\x1b[0m".format(top[0], top[1], top[2])
    return "\x1b[38;2;{};{};{}m▄\x1b[0m".format(bot[0], bot[1], bot[2])


def ansi_from_pixels(pix, cols, rows):
    lines = []
    for row in range(rows):
        parts = []
        y0, y1 = row * 2, row * 2 + 1
        for x in range(cols):
            parts.append(cell(pix[y0][x], pix[y1][x]))
        lines.append("".join(parts))
    return "\n".join(lines)


_ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")


def _visible(s):
    return _ANSI_RE.sub("", s)


def write_ansi(src, w, h, cols, rows, ansi_path):
    pix = load_rgba(src, w, h)
    ansi = ansi_from_pixels(pix, cols, rows)
    lines = ansi.split("\n")
    while lines and _visible(lines[0]).strip() == "":
        lines.pop(0)
    while lines and _visible(lines[-1]).strip() == "":
        lines.pop()
    ansi = "\n".join(lines)
    with open(ansi_path, "w") as f:
        f.write(ansi + "\n")
    print("wrote", ansi_path, "from", src, "(%d rows)" % len(lines))


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else SRC
    if not os.path.isfile(src):
        sys.exit("missing " + src)
    out_dir = os.path.dirname(os.path.abspath(__file__))
    write_ansi(src, PW, PH, COLS, ROWS, os.path.join(out_dir, "dero_hex.ansi"))
    write_ansi(
        src,
        SMALL_PW,
        SMALL_PH,
        SMALL_COLS,
        SMALL_ROWS,
        os.path.join(out_dir, "dero_hex_small.ansi"),
    )


if __name__ == "__main__":
    main()
