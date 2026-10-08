import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/anim/PaMotion.kt'
s = io.open(p, encoding='utf-8').read()
old = '''    /** 减弱动效/瞬时到位：状态直达，仅保留"变了"这个事实。 */
    fun <T> instant(): AnimationSpec<T> = snap<T>()'''
new = '''    /** 减弱动效/瞬时到位：状态直达，仅保留"变了"这个事实。
     *  返回类型收窄为 FiniteAnimationSpec：snap 本就是有限动画，fadeIn/fadeOut
     *  等转场函数只收 Finite，宽声明会逼调用点写死类型参数。 */
    fun <T> instant(): FiniteAnimationSpec<T> = snap<T>()'''
assert s.count(old) == 1, s.count(old)
s = s.replace(old, new)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('PaMotion.instant narrowed')
