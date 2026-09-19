package dev.pocketagent.app.ui
import org.junit.Assert.assertEquals
import org.junit.Test
class CodexWriterRegressionTest {
    @Test fun copyRetainsCommitAndOnlyChangesRequestedField() {
        var calls = 0
        var actual = ""
        val writer = CodexWriter()
        writer.onCommit = { approval, _, _, effort, _, _, _, _ ->
            calls++
            actual = "$approval:$effort"
        }
        writer.copy(approval="on-request").copy(effort="low").commit()
        assertEquals(1, calls)
        assertEquals("on-request:low", actual)
    }
}
