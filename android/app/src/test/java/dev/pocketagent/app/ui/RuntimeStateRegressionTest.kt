package dev.pocketagent.app.ui

import org.junit.Assert.*
import org.junit.Test
import pocketagent.v1.CodexEvent
import pocketagent.v1.ModelInfo
import pocketagent.v1.TextDelta
import pocketagent.v1.TurnCompleted
import pocketagent.v1.TurnServerMessage
import pocketagent.v1.TurnStarted

class RuntimeStateRegressionTest {
    @Test fun repeatedReplayFramesDoNotDuplicateText() {
        val state = TurnSession.State()
        val delta = TurnServerMessage.newBuilder().setSequence(10)
            .setTextDelta(TextDelta.newBuilder().setDelta("原生结果")).build()
        TurnSession.applyServerFrame("test", state, delta, replay = true)
        TurnSession.applyServerFrame("test", state, delta, replay = true)
        assertEquals("原生结果", state.streamText)
    }

    @Test fun runtimeConfigurationIsNotAnotherTurn() {
        val state = TurnSession.State()
        state.turnTerminal = true
        TurnSession.applyServerFrame("test", state, TurnServerMessage.newBuilder().setStarted(
            TurnStarted.newBuilder().setModelId("pi/private/new-model").setEffort("adaptive")
                .setConfigurationConfirmed(true)).build(), replay = true)
        assertTrue(state.turnTerminal)
        assertFalse(state.streaming)
        assertEquals("pi/private/new-model", state.runningModel)
        assertEquals("adaptive", state.runningEffort)
    }

    @Test fun upstreamRetryIsNotAConnectionFailure() {
        val state = TurnSession.State()
        TurnSession.applyServerFrame("test", state, TurnServerMessage.newBuilder().setCodexEvent(
            CodexEvent.newBuilder().setKind(CodexEvent.Kind.ERROR).setWillRetry(true)
                .setMessage("上游重试")).build(), replay = true)
        assertNull(state.errorMessage)
        TurnSession.applyServerFrame("test", state, TurnServerMessage.newBuilder().setCompleted(
            TurnCompleted.newBuilder().setTurnId("native")).build(), replay = true)
        assertNull(state.errorMessage)
        assertTrue(state.turnTerminal)
    }

    @Test fun unfamiliarModelAndStrengthArePresentedWithoutFallback() {
        val model = ModelInfo.newBuilder().setId("pi/private/custom-v999").setProvider("pi")
            .addAllAvailableEfforts(listOf("adaptive", "ultra-new")).build()
        assertEquals(listOf("", "adaptive", "ultra-new"),
            effortsFor(listOf(model), model.id, "pi").map { it.id })
        assertEquals("ultra-new", effortLabelOf("ultra-new"))
        assertEquals(listOf(""), effortsFor(listOf(model), "pi/unknown", "pi").map { it.id })
    }
}
