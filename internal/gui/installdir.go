//go:build !nogui

package gui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sichengchen/eipgam/internal/update"
)

// The Windows app opened from Downloads installs itself (update.Settle),
// and the first time it asks where, in a dialog of Windows' own
// (installdir_windows.go): the folder, Install, Change… and Cancel, as an
// installer's folder page has them (feng_21795_78266 on Discord).

// installWords are the dialog's words in one language: its title, the
// question, the two buttons, the folder picker's title, and what is said
// when the folder can't be used: one only an administrator can write
// to, one downloads land in, and any other failure; and, under it all,
// that the download it was started from can be deleted once installed
// (keep): the install copies it and leaves it where it is
// (@jiangshangmuyun on X found "the original is still there").
type installWords struct {
	title, ask, install, change, pick string
	admin, downloads, failed          string
	keep                              string
}

var installText = map[string]installWords{
	"en": {
		title: "eipgam", ask: "Install eipgam in this folder?", install: "Install", change: "Change…",
		pick:      "Choose where to install eipgam",
		admin:     "Only an administrator can write to %s. eipgam is installed for you alone and updates itself, so choose a folder of your own.",
		downloads: "Downloads land in %s. Choose another folder.",
		failed:    "eipgam can't be installed in %s: %v",
		keep:      "Once eipgam is installed, you can delete %s.",
	},
	"zh": {
		title: "eipgam", ask: "把 eipgam 安装到这个文件夹？", install: "安装", change: "更改…",
		pick:      "选择 eipgam 的安装位置",
		admin:     "只有管理员才能写入 %s。eipgam 只为你安装，并会自己更新，请选择一个你自己的文件夹。",
		downloads: "%s 是下载存放的地方，请选择另一个文件夹。",
		failed:    "eipgam 无法安装到 %s：%v",
		keep:      "安装完成后，可以删除 %s。",
	},
	"zh-TW": {
		title: "eipgam", ask: "把 eipgam 安裝到這個資料夾？", install: "安裝", change: "變更…",
		pick:      "選擇 eipgam 的安裝位置",
		admin:     "只有系統管理員才能寫入 %s。eipgam 只為你安裝，並會自己更新，請選擇一個你自己的資料夾。",
		downloads: "%s 是下載存放的地方，請選擇另一個資料夾。",
		failed:    "eipgam 無法安裝到 %s：%v",
		keep:      "安裝完成後，可以刪除 %s。",
	},
	"ja": {
		title: "eipgam", ask: "このフォルダーに eipgam をインストールしますか？", install: "インストール", change: "変更…",
		pick:      "eipgam のインストール先を選択",
		admin:     "%s に書き込めるのは管理者だけです。eipgam はあなた専用にインストールされ、自分で更新するので、あなた自身のフォルダーを選んでください。",
		downloads: "%s はダウンロードの保存先です。別のフォルダーを選んでください。",
		failed:    "%s に eipgam をインストールできません：%v",
		keep:      "インストール後は %s を削除してかまいません。",
	},
	"de": {
		title: "eipgam", ask: "eipgam in diesem Ordner installieren?", install: "Installieren", change: "Ändern…",
		pick:      "Speicherort für eipgam wählen",
		admin:     "Nur ein Administrator darf in %s schreiben. eipgam wird nur für Sie installiert und aktualisiert sich selbst, wählen Sie also einen eigenen Ordner.",
		downloads: "In %s landen Downloads. Wählen Sie einen anderen Ordner.",
		failed:    "eipgam kann nicht in %s installiert werden: %v",
		keep:      "Nach der Installation können Sie %s löschen.",
	},
}

// installWordsFor is the dialog's words in lang (trayLang's), English
// where it has none.
func installWordsFor(lang string) installWords {
	if w, ok := installText[lang]; ok {
		return w
	}
	return installText["en"]
}

// footer says that exe, the download the dialog was opened from, can be
// deleted once eipgam is installed; "" when exe isn't known.
func (w installWords) footer(exe string) string {
	if exe == "" {
		return ""
	}
	return fmt.Sprintf(w.keep, exe)
}

// refusal is what the dialog says about dir, which check refused with err.
func (w installWords) refusal(dir string, err error) string {
	switch {
	case errors.Is(err, update.ErrNeedsAdmin):
		return fmt.Sprintf(w.admin, dir)
	case errors.Is(err, update.ErrDownloadFolder):
		return fmt.Sprintf(w.downloads, dir)
	}
	return fmt.Sprintf(w.failed, dir, err)
}

// unanswerable says why the install question would go unanswered, ""
// when someone can answer it: ci is the CI variable every CI service
// sets (GitHub's, GitLab's, Azure's: true or 1), visible whether the
// desktop is one a person sees (installdir_windows.go).
func unanswerable(ci string, visible bool) string {
	switch strings.ToLower(strings.TrimSpace(ci)) {
	case "", "0", "false", "no":
	default:
		return "CI=" + ci
	}
	if !visible {
		return "no one sees this desktop"
	}
	return ""
}
