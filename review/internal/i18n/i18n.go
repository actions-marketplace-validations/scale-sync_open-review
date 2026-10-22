package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	currentLocale = "en"
	localeMu      sync.RWMutex
)

var catalog = map[string]map[string]string{
	"en": {
		"review.complete":   "Review complete: %d findings (%s)",
		"review.empty":      "No files to review",
		"review.skipped":    "All changed files were skipped by review policy",
		"config.connected":  "Connected to %s (%s)",
		"config.saved":      "Saved %s (%s) to %s.",
		"learnings.added":   "Added learning %s for repository %s",
		"learnings.deleted": "Deleted learning %s",
		"learnings.empty":   "No learnings stored for this repository",
	},
	"es": {
		"review.complete":   "Revisión completada: %d hallazgos (%s)",
		"review.empty":      "No hay archivos para revisar",
		"review.skipped":    "Todos los archivos modificados fueron omitidos por la política de revisión",
		"config.connected":  "Conectado a %s (%s)",
		"config.saved":      "Guardado %s (%s) en %s.",
		"learnings.added":   "Aprendizaje %s agregado para el repositorio %s",
		"learnings.deleted": "Aprendizaje %s eliminado",
		"learnings.empty":   "No hay aprendizajes almacenados para este repositorio",
	},
	"ja": {
		"review.complete":   "レビュー完了: %d 件の指摘 (%s)",
		"review.empty":      "レビュー対象のファイルがありません",
		"review.skipped":    "すべての変更ファイルがレビューポリシーによりスキップされました",
		"config.connected":  "%s (%s) に接続しました",
		"config.saved":      "%s (%s) を %s に保存しました。",
		"learnings.added":   "リポジトリ %s に学習 %s を追加しました",
		"learnings.deleted": "学習 %s を削除しました",
		"learnings.empty":   "このリポジトリの学習データはありません",
	},
	"zh": {
		"review.complete":   "审查完成: %d 条建议 (%s)",
		"review.empty":      "没有需要审查的文件",
		"review.skipped":    "所有修改的文件均被审查策略跳过",
		"config.connected":  "已连接到 %s (%s)",
		"config.saved":      "已将 %s (%s) 保存至 %s。",
		"learnings.added":   "已为仓库 %s 添加学习记录 %s",
		"learnings.deleted": "已删除学习记录 %s",
		"learnings.empty":   "该仓库没有存储学习记录",
	},
	"de": {
		"review.complete":   "Überprüfung abgeschlossen: %d Befunde (%s)",
		"review.empty":      "Keine Dateien zur Überprüfung",
		"review.skipped":    "Alle geänderten Dateien wurden durch die Richtlinie übersprungen",
		"config.connected":  "Verbunden mit %s (%s)",
		"config.saved":      "%s (%s) unter %s gespeichert.",
		"learnings.added":   "Erkenntnis %s für Repository %s hinzugefügt",
		"learnings.deleted": "Erkenntnis %s gelöscht",
		"learnings.empty":   "Keine Erkenntnisse für dieses Repository gespeichert",
	},
}

func Normalize(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if idx := strings.IndexAny(code, "-_."); idx >= 0 {
		code = code[:idx]
	}
	switch code {
	case "es", "ja", "zh", "de":
		return code
	default:
		return "en"
	}
}

func SetLocale(code string) {
	localeMu.Lock()
	defer localeMu.Unlock()
	currentLocale = Normalize(code)
}

func CurrentLocale() string {
	localeMu.RLock()
	defer localeMu.RUnlock()
	return currentLocale
}

func InitFromEnv() {
	for _, env := range []string{"OPEN_REVIEW_LOCALE", "LC_ALL", "LANG"} {
		if val := os.Getenv(env); val != "" {
			SetLocale(val)
			return
		}
	}
	SetLocale("en")
}

func T(key string, args ...any) string {
	localeMu.RLock()
	loc := currentLocale
	localeMu.RUnlock()

	messages, ok := catalog[loc]
	if !ok {
		messages = catalog["en"]
	}
	format, ok := messages[key]
	if !ok {
		format, ok = catalog["en"][key]
		if !ok {
			format = key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(format, args...)
	}
	return format
}
