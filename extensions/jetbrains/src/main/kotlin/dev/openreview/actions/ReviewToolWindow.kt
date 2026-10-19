package dev.openreview.actions

import com.google.gson.JsonObject
import com.google.gson.JsonParser
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.command.WriteCommandAction
import com.intellij.openapi.editor.Document
import com.intellij.openapi.editor.impl.DocumentMarkupModel
import com.intellij.openapi.editor.markup.EffectType
import com.intellij.openapi.editor.markup.RangeHighlighter
import com.intellij.openapi.editor.markup.TextAttributes
import com.intellij.openapi.fileEditor.FileDocumentManager
import com.intellij.openapi.fileEditor.OpenFileDescriptor
import com.intellij.openapi.progress.ProgressIndicator
import com.intellij.openapi.progress.ProgressManager
import com.intellij.openapi.progress.Task
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.util.io.FileUtil
import com.intellij.openapi.vfs.LocalFileSystem
import com.intellij.ui.content.ContentFactory
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import dev.openreview.services.CliRunner
import java.awt.BorderLayout
import java.awt.FlowLayout
import java.io.File
import java.nio.file.Files
import java.nio.file.Path
import java.util.UUID
import javax.swing.DefaultListModel
import javax.swing.JButton
import javax.swing.JList
import javax.swing.JPanel
import javax.swing.JScrollPane
import com.intellij.ui.JBColor

data class Finding(val id: String, val file: String, val line: Int, val endLine: Int, val title: String, val explanation: String, val snippet: String, val fix: String) {
    override fun toString() = "$file:$line  $title"
}

class ReviewToolWindow : ToolWindowFactory {
    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val panel = ReviewPanel(project)
        toolWindow.contentManager.addContent(ContentFactory.getInstance().createContent(panel, "Findings", false))
    }
}

private class ReviewPanel(private val project: Project) : JPanel(BorderLayout()) {
    private val root = File(project.basePath ?: ".")
    private val runner = CliRunner(System.getProperty("openReview.cliPath", "open-review"))
    private val findings = DefaultListModel<Finding>()
    private val allFindings = mutableListOf<Finding>()
    private val list = JList(findings)
    private var sessionDir: Path? = null
    private var sessionId: String? = null
    private val marks = mutableListOf<Pair<Document, RangeHighlighter>>()

    init {
        val toolbar = JPanel(FlowLayout(FlowLayout.LEFT))
        fun button(label: String, action: () -> Unit) { toolbar.add(JButton(label).apply { addActionListener { action() } }) }
        button("Preview") { preview() }
        button("Review") { review() }
        button("Refresh status") { refresh() }
        button("Dismiss") { triage("ignored") }
        button("False positive") { triage("false_positive") }
        button("Apply suggestion") { applySuggestion() }
        button("Provider") { setupProvider() }
        add(toolbar, BorderLayout.NORTH)
        add(JScrollPane(list), BorderLayout.CENTER)
        list.addListSelectionListener { if (!it.valueIsAdjusting) navigate() }
    }

    private fun background(title: String, args: List<String>, input: String? = null, onResult: (String) -> Unit) {
        ProgressManager.getInstance().run(object : Task.Backgroundable(project, title, true) {
            override fun run(indicator: ProgressIndicator) {
                try {
                    val result = runner.run(root, args, { indicator.isCanceled }, input)
                    ApplicationManager.getApplication().invokeLater { onResult(result.output) }
                } catch (failure: Exception) {
                    ApplicationManager.getApplication().invokeLater { Messages.showErrorDialog(project, failure.message ?: "Review failed", "open-review") }
                }
            }
        })
    }

    private fun preview() {
        background("Previewing review scope", listOf("review", "--repo", root.path, "--preview", "--output", "json")) { raw ->
            val coverage = JsonParser.parseString(raw).asJsonObject.getAsJsonArray("coverage")
            val text = coverage.joinToString("\n") { item ->
                val row = item.asJsonObject
                "${row.get("state").asString}: ${row.get("path").asString}"
            }
            Messages.showInfoMessage(project, text.ifEmpty { "No changed files" }, "open-review scope")
        }
    }

    private fun review() {
        val dir = Path.of(com.intellij.openapi.application.PathManager.getSystemPath(), "open-review", UUID.randomUUID().toString())
        Files.createDirectories(dir)
        sessionDir = dir
        background("Reviewing workspace", listOf("review", "--repo", root.path, "--output", "json", "--quiet", "--session-dir", dir.toString())) { raw ->
            val result = JsonParser.parseString(raw).asJsonObject
            val sessions = Files.list(dir).use { stream -> stream.filter { Files.isDirectory(it) }.toList() }
            if (sessions.size != 1) { Messages.showErrorDialog(project, "Review session was not saved", "open-review"); return@background }
            sessionId = sessions[0].fileName.toString()
            allFindings.clear()
            result.getAsJsonArray("findings")?.forEach { element ->
                val row: JsonObject = element.asJsonObject
                fun value(key: String) = row.get(key)?.takeUnless { it.isJsonNull }?.asString ?: ""
                allFindings.add(Finding(value("id"), value("file"), row.get("startLine").asInt, row.get("endLine").asInt, value("title"), value("explanation"), value("citedSnippet"), value("suggestedFix")))
            }
            refresh()
        }
    }

    private fun refresh() {
        val dir = sessionDir ?: return
        val id = sessionId ?: return
        background("Refreshing finding status", listOf("sessions", "show", "--dir", dir.toString(), "--repo", root.path, id)) { raw ->
            val states = JsonParser.parseString(raw).asJsonObject.getAsJsonObject("triage")
            findings.clear()
            for (item in allFindings) {
                val state = states?.get(item.id)?.asString
                if (state == null || state == "open") findings.addElement(item)
            }
            markLines()
        }
    }

    private fun triage(state: String) {
        val item = list.selectedValue ?: return
        val dir = sessionDir ?: return
        val id = sessionId ?: return
        background("Updating finding", listOf("sessions", "triage", "--dir", dir.toString(), "--repo", root.path, id, item.id, state)) { refresh() }
    }

    private fun fileFor(item: Finding): File? {
        val file = File(root, item.file).canonicalFile
        return file.takeIf { FileUtil.isAncestor(root.canonicalFile, it, true) && it.isFile }
    }

    private fun navigate() {
        val item = list.selectedValue ?: return
        val file = fileFor(item) ?: return
        val virtual = LocalFileSystem.getInstance().refreshAndFindFileByIoFile(file) ?: return
        OpenFileDescriptor(project, virtual, (item.line - 1).coerceAtLeast(0), 0).navigate(true)
    }

    private fun markLines() {
        for ((document, mark) in marks) DocumentMarkupModel.forDocument(document, project, true).removeHighlighter(mark)
        marks.clear()
        for (index in 0 until findings.size()) {
            val item = findings[index]
            val file = fileFor(item) ?: continue
            val virtual = LocalFileSystem.getInstance().refreshAndFindFileByIoFile(file) ?: continue
            val document = FileDocumentManager.getInstance().getDocument(virtual) ?: continue
            val line = (item.line - 1).coerceIn(0, (document.lineCount - 1).coerceAtLeast(0))
            val attributes = TextAttributes().apply { effectColor = JBColor.RED; effectType = EffectType.WAVE_UNDERSCORE }
            val mark = DocumentMarkupModel.forDocument(document, project, true).addLineHighlighter(line, 1000, attributes)
            marks.add(document to mark)
        }
    }

    private fun applySuggestion() {
        val item = list.selectedValue ?: return
        if (item.snippet.isBlank() || item.fix.isBlank()) return
        val file = fileFor(item) ?: return
        val virtual = LocalFileSystem.getInstance().refreshAndFindFileByIoFile(file) ?: return
        val document = FileDocumentManager.getInstance().getDocument(virtual) ?: return
        val start = (item.line - 1).coerceIn(0, document.lineCount - 1)
        val end = (item.endLine - 1).coerceIn(start, document.lineCount - 1)
        val from = document.getLineStartOffset(start)
        val to = document.getLineEndOffset(end)
        if (document.getText(com.intellij.openapi.util.TextRange(from, to)).trim() != item.snippet.trim()) {
            Messages.showWarningDialog(project, "Finding text changed; review again before applying", "open-review")
            return
        }
        if (Messages.showYesNoDialog(project, "Replace ${item.file}:${item.line}-${item.endLine} with:\n${item.fix.take(500)}", "Apply suggested text?", null) != Messages.YES) return
        WriteCommandAction.runWriteCommandAction(project) { document.replaceString(from, to, item.fix) }
    }

    private fun setupProvider() {
        val provider = Messages.showInputDialog(project, "Provider (openai, anthropic, gemini)", "open-review provider", null) ?: return
        val model = Messages.showInputDialog(project, "Model name", "open-review provider", null) ?: return
        val key = Messages.showPasswordDialog("API key", "open-review provider") ?: return
        if (provider.isBlank() || model.isBlank() || key.isBlank()) return
        background("Saving provider", listOf("config", "set", "--provider", provider, "--model", model, "--api-key-stdin"), key) { Messages.showInfoMessage(project, "Provider saved", "open-review") }
    }
}
