package dev.openreview

import dev.openreview.services.CliRunner
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals

class CliRunnerTest {
    @Test fun argumentsAreSeparateAndPartialOutputIsKept() {
        val result = CliRunner("/bin/sh").run(File("."), listOf("-c", "printf '%s' \"\$1\"; exit 2", "sh", "a b"))
        assertEquals(2, result.exitCode)
        assertEquals("a b\n", result.output)
    }
}
