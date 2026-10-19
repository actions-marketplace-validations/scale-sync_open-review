package dev.openreview.services

import java.io.File
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread

class CliRunner(private val executable: String = "open-review") {
    data class Result(val output: String, val exitCode: Int)

    fun run(root: File, args: List<String>, cancelled: () -> Boolean = { false }, input: String? = null): Result {
        val process = ProcessBuilder(listOf(executable) + args).directory(root).start()
		process.outputStream.use { if (input != null) it.write(input.toByteArray(Charsets.UTF_8)) }
        val stdout = StringBuffer()
        val stderr = StringBuffer()
        val outThread = thread { process.inputStream.bufferedReader().use { it.forEachLine { line -> if (stdout.length < 16 * 1024 * 1024) stdout.appendLine(line) } } }
        val errThread = thread { process.errorStream.bufferedReader().use { it.forEachLine { line -> if (stderr.length < 16 * 1024 * 1024) stderr.appendLine(line) } } }
        while (!process.waitFor(100, TimeUnit.MILLISECONDS)) {
            if (cancelled() || stdout.length >= 16 * 1024 * 1024 || stderr.length >= 16 * 1024 * 1024) {
                process.destroyForcibly()
                outThread.join()
                errThread.join()
                error("Review cancelled or CLI output exceeded 16 MB")
            }
        }
        outThread.join()
        errThread.join()
        val code = process.exitValue()
        if (code != 0 && code != 2) error(stderr.toString().trim().ifEmpty { "open-review exited $code" })
        return Result(stdout.toString(), code)
    }
}
