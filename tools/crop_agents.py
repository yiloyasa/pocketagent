# -*- coding: utf-8 -*-
# 从原作截图裁剪 7 个 agent 的真实图标 → res/drawable-nodpi/agent_*.png
# 再生成 Pa 启动图标(白底、黑 P、蓝 a)
from PIL import Image
import os

CACHE = r'C:\Users\yanzi\.zcode\cli\image-cache\sess_c1f177c3-9a2f-4e50-9d3f-0f1f7081261c'
OUT = r'C:\dev\pocketagent\android\app\src\main\res\drawable-nodpi'
os.makedirs(OUT, exist_ok=True)

# 主页网格图:第一行 Codex/OpenCode/ClaudeCode/Pi,第二行 Hermes/DSH/ZCode
src = Image.open(os.path.join(CACHE, 'image-187bfca5c2a53a9e46c3b8c789880baa.jpg'))
W, H = src.size
print('src size:', W, H)
# 我在 864 宽视图里读到的 tile 中心坐标(比例换算)
scale = W / 864.0
tiles = {
    'codex':      (148, 618),
    'opencode':   (344, 618),
    'claudecode': (540, 618),
    'pi':         (736, 618),
    'hermes':     (148, 858),
    'dsh':        (344, 858),
    'zcode':      (540, 858),
}
half = int(52 * scale)  # tile 半宽(图标块约 104px @864)
for name, (cx, cy) in tiles.items():
    x1, y1 = int(cx*scale - half), int(cy*scale - half)
    x2, y2 = int(cx*scale + half), int(cy*scale + half)
    crop = src.crop((x1, y1, x2, y2))
    # 放大到 256x256 存盘(圆角在 Compose 里裁)
    crop = crop.resize((256, 256), Image.LANCZOS)
    crop.save(os.path.join(OUT, f'agent_{name}.png'))
    print('saved', name, crop.size)
