# -*- coding: utf-8 -*-
"""Agent 官方/官方风格图标合成:
- 官方品牌资源(claude/deepseek/opencode 官方 SVG,zcode 官方 PNG)
- Codex = 官方图标风格(蓝渐变 + 终端提示符)
- Pi = 官方像素 P;Hermes = 信使之翼
统一 512px 圆角瓦片 → 192px 输出(drawable-nodpi)
"""
from PIL import Image, ImageDraw, ImageFont
from svglib.svglib import svg2rlg
from reportlab.graphics import renderPM
import io, os

BRAND = r'C:\dev\pocketagent\tools\brand'
OUT = r'C:\dev\pocketagent\android\app\src\main\res\drawable-nodpi'
FONT = r'C:\dev\pocketagent\tools\fonts\Poppins-Bold.ttf'
S = 512
R = int(S * 0.24)
TILE = (23, 23, 27, 255)  # #17171B


def tile():
    im = Image.new('RGBA', (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle([0, 0, S, S], radius=R, fill=TILE)
    return im, d


def glyph_png(png_bytes, size, color=None):
    """SVG 渲染字节流 → 透明 PNG → 重着色 → 缩放"""
    g = Image.open(io.BytesIO(png_bytes)).convert('RGBA')
    g = g.resize((size, size), Image.LANCZOS)
    if color:
        px = g.load()
        for y in range(g.height):
            for x in range(g.width):
                r, gg, b, a = px[x, y]
                if a > 0:
                    px[x, y] = (color[0], color[1], color[2], a)
    return g


def svg_glyph(svg_path, size, color):
    """SVG → 透明 PNG。支持填充型(simple-icons)与描边型(lucide),
    曲线/圆弧按参数细分采样,孔洞按子路径(非零环绕)正确保留。"""
    import re
    import numpy as np
    import matplotlib
    matplotlib.use('Agg')
    from matplotlib.path import Path as MPath
    import matplotlib.figure as mfigure
    from matplotlib.patches import PathPatch
    from svgpathtools import parse_path
    from svgpathtools.path import CubicBezier, QuadraticBezier, Arc

    s = open(svg_path, encoding='utf-8').read()
    is_stroke = 'fill="none"' in s or ('stroke=' in s and 'stroke="none"' not in s)
    ds = re.findall(r'<path[^>]*\bd="([^"]+)"', s)
    sp = parse_path(' '.join(ds))
    vertices, codes = [], []
    for sub in sp.continuous_subpaths():
        verts = [complex(sub.start.real, sub.start.imag)]
        for seg in sub:
            if isinstance(seg, (CubicBezier, QuadraticBezier, Arc)):
                n = 24 if isinstance(seg, Arc) else 14
                for t in np.linspace(0, 1, n)[1:]:
                    p = seg.point(t)
                    verts.append(complex(p.real, p.imag))
            else:
                p = seg.end
                verts.append(complex(p.real, p.imag))
        vertices.extend([(p.real, p.imag) for p in verts])
        codes.extend([MPath.MOVETO] + [MPath.LINETO] * (len(verts) - 1))
    mp = MPath(vertices, codes)

    fig = mfigure.Figure(figsize=(size / 100, size / 100), dpi=100)
    ax = fig.add_axes([0, 0, 1, 1])
    ax.set_xlim(0, 24)
    ax.set_ylim(24, 0)  # SVG y 向下
    ax.axis('off')
    if is_stroke:
        # lucide: 视图 24 单位、stroke-width 2 → 按渲染尺寸换算线宽
        ax.add_patch(PathPatch(
            mp, fill=False, edgecolor=tuple(c / 255 for c in color),
            linewidth=2 / 24 * size * (100 / size), capstyle='round', joinstyle='round'))
    else:
        ax.add_patch(PathPatch(mp, facecolor=tuple(c / 255 for c in color), edgecolor='none'))
    buf = io.BytesIO()
    fig.savefig(buf, format='png', transparent=True)
    buf.seek(0)
    g = Image.open(buf).convert('RGBA').resize((size, size), Image.LANCZOS)
    # 精确重着色(保 alpha)
    px = g.load()
    for y in range(g.height):
        for x in range(g.width):
            r, gg, b, a = px[x, y]
            if a > 0:
                px[x, y] = (color[0], color[1], color[2], a)
    return g


def center_paste(im, g, dy=0):
    im.alpha_composite(g, ((S - g.width) // 2, (S - g.height) // 2 + dy))
    return im


def save(im, name):
    im.resize((192, 192), Image.LANCZOS).save(os.path.join(OUT, f'agent_{name}.png'))
    im.save(os.path.join(BRAND, f'tile_{name}.png'))
    print('✓', name)


# ---- Claude:官方星芒 #D97757 ----
im, _ = tile()
center_paste(im, svg_glyph(os.path.join(BRAND, 'claude.svg'), 300, (217, 119, 87)))
save(im, 'claudecode')

# ---- DSH(DeepSeek 官方鲸鱼 #5786FE)----
im, _ = tile()
center_paste(im, svg_glyph(os.path.join(BRAND, 'deepseek.svg'), 300, (87, 134, 254)))
save(im, 'dsh')

# ---- OpenCode:官方标(反转白色适配深色瓦片)----
im, _ = tile()
center_paste(im, svg_glyph(os.path.join(BRAND, 'opencode.svg'), 290, (236, 236, 240)))
save(im, 'opencode')

# ---- ZCode:官方应用图标整体贴入 ----
z = Image.open(os.path.join(BRAND, 'zcode-official.png')).convert('RGBA').resize((S, S), Image.LANCZOS)
mask = Image.new('L', (S, S), 0)
ImageDraw.Draw(mask).rounded_rectangle([0, 0, S, S], radius=R, fill=255)
im = Image.new('RGBA', (S, S), (0, 0, 0, 0))
im.paste(z, (0, 0), mask)
save(im, 'zcode')

# ---- Codex:官方风格 —— 蓝渐变 + 终端提示符 ">_" ----
im = Image.new('RGBA', (S, S), (0, 0, 0, 0))
d = ImageDraw.Draw(im)
grad = Image.new('RGBA', (S, S))
top, bot = (96, 165, 250), (37, 99, 235)
gd = ImageDraw.Draw(grad)
for y in range(S):
    t = y / S
    gd.line([(0, y), (S, y)], fill=(
        int(top[0] + (bot[0] - top[0]) * t),
        int(top[1] + (bot[1] - top[1]) * t),
        int(top[2] + (bot[2] - top[2]) * t), 255))
mask = Image.new('L', (S, S), 0)
ImageDraw.Draw(mask).rounded_rectangle([0, 0, S, S], radius=R, fill=255)
im.paste(grad, (0, 0), mask)
f = ImageFont.truetype(FONT, 190)
bb = d.textbbox((0, 0), '>_', font=f)
d.text(((S - bb[2] - bb[0]) // 2, (S - bb[3] - bb[1]) // 2), '>_', font=f, fill=(255, 255, 255, 255))
save(im, 'codex')

# ---- Pi:官方像素 P ----
im, d = tile()
blk = 42
grid = [
    'XXXXX.',
    'X....X',
    'X....X',
    'X....X',
    'XXXXX.',
    'X.....',
    'X.....',
]
gw, gh = len(grid[0]) * blk, len(grid) * blk
ox, oy = (S - gw) // 2, (S - gh) // 2
for r_i, row in enumerate(grid):
    for c_i, ch in enumerate(row):
        if ch == 'X':
            x0, y0 = ox + c_i * blk, oy + r_i * blk
            d.rounded_rectangle([x0 + 2, y0 + 2, x0 + blk - 2, y0 + blk - 2], radius=9, fill=(236, 236, 240, 255))
save(im, 'pi')

# ---- Hermes:信使之翼(lucide feather,金) ----
im, _ = tile()
fg = svg_glyph(r'G:\Zcode 文件\143\icons\lucide\icons\feather.svg', 290, (245, 197, 66))
center_paste(im, fg)
save(im, 'hermes')

print('all agent tiles done')
