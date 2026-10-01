package dev.pocketagent.app.ui.component

import org.junit.Assert.assertEquals
import org.junit.Test

/**
 * 面包屑分段规则(pathBreadcrumbs)的单测 —— R2-D 落地项 1:
 * "当前路径面包屑可点级跳转,点中间一段直接回到那层"。
 *
 * 分段产物必须与 [parentDirectoryOf]/ListDirectory 的路径形态一致:
 * 父级段带尾分隔符可直接喂给 daemon;末段保持传入原样;首段恒为"此电脑"。
 */
class PathBreadcrumbTest {

    @Test
    fun `Windows 深路径 逐段可点`() {
        val crumbs = pathBreadcrumbs("G:\\a\\b")
        assertEquals(4, crumbs.size)
        assertEquals(PathBreadcrumb("此电脑", ""), crumbs[0])
        assertEquals(PathBreadcrumb("G:\\", "G:\\"), crumbs[1])
        assertEquals(PathBreadcrumb("a", "G:\\a\\"), crumbs[2])
        // 末段 = 当前层,target 保持传入原样(UI 不响应它的点击)
        assertEquals(PathBreadcrumb("b", "G:\\a\\b"), crumbs[3])
    }

    @Test
    fun `Windows 盘根 只有根段与盘符段`() {
        val crumbs = pathBreadcrumbs("G:\\")
        assertEquals(2, crumbs.size)
        assertEquals(PathBreadcrumb("此电脑", ""), crumbs[0])
        assertEquals(PathBreadcrumb("G:\\", "G:\\"), crumbs[1])
    }

    @Test
    fun `Unix 路径 根分隔符自成一段`() {
        val crumbs = pathBreadcrumbs("/home/u")
        assertEquals(4, crumbs.size)
        assertEquals(PathBreadcrumb("此电脑", ""), crumbs[0])
        assertEquals(PathBreadcrumb("/", "/"), crumbs[1])
        assertEquals(PathBreadcrumb("home", "/home/"), crumbs[2])
        assertEquals(PathBreadcrumb("u", "/home/u"), crumbs[3])
    }

    @Test
    fun `空路径 即盘符列表层 只有根段`() {
        val crumbs = pathBreadcrumbs("")
        assertEquals(listOf(PathBreadcrumb("此电脑", "")), crumbs)
    }

    @Test
    fun `裸根分隔符 当前层不被根段吞掉`() {
        val crumbs = pathBreadcrumbs("/")
        assertEquals(2, crumbs.size)
        assertEquals(PathBreadcrumb("此电脑", ""), crumbs[0])
        assertEquals(PathBreadcrumb("/", "/"), crumbs[1])
    }

    @Test
    fun `中文与空格目录名 原样保留`() {
        val crumbs = pathBreadcrumbs("G:\\试验区\\outputs\\abc-123")
        assertEquals(5, crumbs.size)
        assertEquals(PathBreadcrumb("G:\\", "G:\\"), crumbs[1])
        assertEquals(PathBreadcrumb("试验区", "G:\\试验区\\"), crumbs[2])
        assertEquals(PathBreadcrumb("outputs", "G:\\试验区\\outputs\\"), crumbs[3])
        assertEquals(PathBreadcrumb("abc-123", "G:\\试验区\\outputs\\abc-123"), crumbs[4])
    }

    @Test
    fun `任选中间段都能回退到该层 与 parentDirectoryOf 链一致`() {
        // 面包屑的 target 应当落在 parentDirectoryOf 链的节点上:
        // 从任意中间段出发,一路 parentDirectoryOf 最终收敛到盘根 → null(此电脑)
        val crumbs = pathBreadcrumbs("G:\\a\\b\\c")
        var p: String? = crumbs[2].target // 点 "a" 段
        val seen = mutableListOf<String?>()
        while (p != null) {
            seen += p
            p = parentDirectoryOf(p)
        }
        seen += null
        assertEquals(listOf<String?>("G:\\a\\", "G:\\", null), seen)
    }
}
