package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Test

class ConnectionDiagnosticsTest {
    @Test
    fun classifies_auth_failure() {
        assertEquals(
            ConnectionFailureKind.AUTH,
            classifyConnectionFailure("UNAUTHENTICATED: token 无效,请先配对"),
        )
    }

    @Test
    fun classifies_timeout_failure() {
        assertEquals(
            ConnectionFailureKind.TIMEOUT,
            classifyConnectionFailure("deadline exceeded"),
        )
    }

    @Test
    fun classifies_unavailable_failure() {
        assertEquals(
            ConnectionFailureKind.UNAVAILABLE,
            classifyConnectionFailure("UNAVAILABLE: failed to connect to host"),
        )
    }
}
