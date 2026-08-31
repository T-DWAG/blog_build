#!/usr/bin/env python3
"""字体子集化:按站点实际用字裁剪,914KB → 几十 KB"""
from fontTools import subset
import os, time

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FONTS = os.path.join(ROOT, 'fonts')

chars = open('/tmp/blog-chars.txt', encoding='utf-8').read() + 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !"#$%&\'()*+,-./:;<=>?@[\\]^_`{|}~'

jobs = [
    ('fusion-pixel-12px-proportional-zh_hans.ttf.woff2', 'fusion-pixel-12px-proportional-zh_hans.subset.woff2'),
    ('fusion-pixel-12px-monospaced-zh_hans.ttf.woff2',   'fusion-pixel-12px-monospaced-zh_hans.subset.woff2'),
]

t0 = time.time()
for src, dst in jobs:
    opts = subset.Options()
    opts.text = chars
    opts.layout_features = []          # 跳过排版特性闭包,加速
    opts.hinting = False               # 位图风格字体不需要 hinting
    opts.name_IDs = ['*']
    opts.notdef_outline = True
    opts.recalc_bounds = True
    opts.recalc_timestamp = False
    font = subset.load_font(os.path.join(FONTS, src), opts)
    ss = subset.Subsetter(options=opts)
    ss.populate(text=chars)
    ss.subset(font)
    subset.save_font(font, os.path.join(FONTS, dst), opts)
    a, b = os.path.getsize(os.path.join(FONTS, src)), os.path.getsize(os.path.join(FONTS, dst))
    print(f'{dst}: {a//1024}KB -> {b//1024}KB ({time.time()-t0:.0f}s)', flush=True)
print('DONE', flush=True)
