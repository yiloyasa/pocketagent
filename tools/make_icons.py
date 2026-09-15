# -*- coding: utf-8 -*-
# Pa 启动图标:白底圆角 + Poppins Bold 大号 "Pa"(黑 P / 蓝 a),基线对齐、负字距
from PIL import Image, ImageDraw, ImageFont
import os

RES = r'C:\dev\pocketagent\android\app\src\main\res'
FONT = r'C:\dev\pocketagent\tools\fonts\Poppins-Bold.ttf'
BLACK = (13, 13, 15)
BLUE = (74, 157, 255)


def draw_pa(size):
    """画布上画 Pa,内容占 ~74%(自适应图标安全区内),光学居中"""
    im = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    # 字号:P 占画布 62%,a 约 0.68 倍 —— 大而饱满
    p_size = int(size * 0.62)
    a_size = int(p_size * 0.68)
    fp = ImageFont.truetype(FONT, p_size)
    fa = ImageFont.truetype(FONT, a_size)
    pb = d.textbbox((0, 0), 'P', font=fp)
    ab = d.textbbox((0, 0), 'a', font=fa)
    pw, ph = pb[2] - pb[0], pb[3] - pb[1]
    aw, ah = ab[2] - ab[0], ab[3] - ab[1]
    # 负字距:大 P 和小 a 咬合更紧(≈P 字号的 2%)
    gap = -int(p_size * 0.02)
    total_w = pw + gap + aw
    x0 = (size - total_w) // 2
    # 基线:下移一点点做光学居中(大写 P 视觉重心偏上)
    base_y = (size + ph) // 2 + int(size * 0.012)
    d.text((x0 - pb[0], base_y - ph - pb[1]), 'P', font=fp, fill=BLACK)
    ax = x0 + pw + gap
    d.text((ax - ab[0], base_y - ah - ab[1]), 'a', font=fa, fill=BLUE)
    return im


# 自适应图标前景 432x432,内容居中 ~70%(≈300px)
fg = Image.new('RGBA', (432, 432), (255, 255, 255, 255))
inner = draw_pa(302)
fg.paste(inner, ((432 - 302) // 2, (432 - 302) // 2), inner)
os.makedirs(os.path.join(RES, 'drawable-nodpi'), exist_ok=True)
fg.save(os.path.join(RES, 'drawable-nodpi', 'ic_launcher_fg.png'))

# 传统 PNG:高清绘制 1024 再缩到各 dpi(抗锯齿)
HI = 1024
hi = draw_pa(HI)
RADIUS = int(HI * 0.225)
mask = Image.new('L', (HI, HI), 0)
ImageDraw.Draw(mask).rounded_rectangle([0, 0, HI, HI], radius=RADIUS, fill=255)
white = Image.new('RGBA', (HI, HI), (255, 255, 255, 255))
white.paste(hi, (0, 0), hi)
launcher = Image.new('RGBA', (HI, HI), (0, 0, 0, 0))
launcher.paste(white, (0, 0), mask)

for dpi, size in [('mdpi', 48), ('hdpi', 72), ('xhdpi', 96), ('xxhdpi', 144), ('xxxhdpi', 192)]:
    d = os.path.join(RES, f'mipmap-{dpi}')
    os.makedirs(d, exist_ok=True)
    launcher.resize((size, size), Image.LANCZOS).save(os.path.join(d, 'ic_launcher.png'))
    # 圆形版(部分桌面用)
    mask2 = Image.new('L', (HI, HI), 0)
    ImageDraw.Draw(mask2).ellipse([0, 0, HI, HI], fill=255)
    round_im = Image.new('RGBA', (HI, HI), (0, 0, 0, 0))
    round_im.paste(white, (0, 0), mask2)
    round_im.resize((size, size), Image.LANCZOS).save(os.path.join(d, 'ic_launcher_round.png'))

print('launcher icons regenerated (Poppins-Bold, oversized Pa)')
