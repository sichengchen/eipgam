package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pacman's local database as the AUR's eipgam-bin 0.1.2342-1 leaves it
// (its PKGBUILD installs /usr/bin/eipgam), beside a package that doesn't
// own eipgam.
func pacmanRoot(t *testing.T) string {
	root := t.TempDir()
	local := filepath.Join(root, "var/lib/pacman/local")
	writeFile(t, filepath.Join(local, "ALPM_DB_VERSION"), "9\n")
	writeFile(t, filepath.Join(local, "bash-5.3.3-2/files"), "%FILES%\nusr/\nusr/bin/\nusr/bin/bash\nusr/bin/sh\n\n%BACKUP%\netc/bash.bashrc\t0b5ab6a3\n")
	writeFile(t, filepath.Join(local, "bash-5.3.3-2/desc"), "%NAME%\nbash\n\n%VERSION%\n5.3.3-2\n")
	writeFile(t, filepath.Join(local, "eipgam-bin-0.1.2342-1/files"), "%FILES%\nusr/\nusr/bin/\nusr/bin/eipgam\nusr/share/\nusr/share/applications/\nusr/share/applications/eipgam.desktop\nusr/share/icons/hicolor/256x256/apps/eipgam.png\nusr/share/licenses/eipgam-bin/LICENSE\n\n")
	writeFile(t, filepath.Join(local, "eipgam-bin-0.1.2342-1/desc"), "%NAME%\neipgam-bin\n\n%VERSION%\n0.1.2342-1\n\n%BASE%\neipgam-bin\n")
	return root
}

func TestPackagedByPacman(t *testing.T) {
	root := pacmanRoot(t)
	p := packagedIn(root, "linux", "/usr/bin/eipgam")
	if p == nil || p.Manager != "pacman" || p.Package != "eipgam-bin" {
		t.Fatalf("/usr/bin/eipgam from eipgam-bin: %+v", p)
	}
	if c := p.Command(); c != "yay -S eipgam-bin" {
		t.Fatalf("command %q", c)
	}
	if s := p.Say(); !strings.Contains(s, "pacman") || !strings.Contains(s, "yay -S eipgam-bin") {
		t.Fatalf("said %q", s)
	}
	// /bin is /usr/bin on Arch: the same file
	if p := packagedIn(root, "linux", "/bin/eipgam"); p == nil || p.Package != "eipgam-bin" {
		t.Fatalf("/bin/eipgam: %+v", p)
	}
	// a copy in /usr/local, ~/.local/bin or /opt that no package lists is
	// the user's own, and updates itself
	for _, exe := range []string{"/usr/local/bin/eipgam", "/home/u/.local/bin/eipgam", "/opt/eipgam/eipgam", "/usr/lib/eipgam/eipgam"} {
		if p := packagedIn(root, "linux", exe); p != nil {
			t.Fatalf("%s: %+v", exe, p)
		}
	}
	// not Linux: pacman's files aren't read
	if p := packagedIn(root, "darwin", "/usr/bin/eipgam"); p != nil {
		t.Fatalf("darwin: %+v", p)
	}
}

// A package named otherwise is found too, its name from the folder when
// its desc can't be read.
func TestPackagedByPacmanOtherName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "var/lib/pacman/local/ai-tools-2.0-3/files"), "%FILES%\nopt/ai/bin/eipgam\n")
	p := packagedIn(root, "linux", "/opt/ai/bin/eipgam")
	if p == nil || p.Package != "ai-tools" {
		t.Fatalf("%+v", p)
	}
}

// dpkg lists absolute paths, and on a merged /usr may list /bin for the
// /usr/bin the binary resolves to.
func TestPackagedByDpkg(t *testing.T) {
	root := t.TempDir()
	info := filepath.Join(root, "var/lib/dpkg/info")
	writeFile(t, filepath.Join(info, "coreutils.list"), "/.\n/bin\n/bin/ls\n")
	writeFile(t, filepath.Join(info, "eipgam:amd64.list"), "/.\n/usr\n/usr/bin\n/usr/bin/eipgam\n")
	p := packagedIn(root, "linux", "/usr/bin/eipgam")
	if p == nil || p.Manager != "dpkg" || p.Package != "eipgam" || p.Name() != "apt" || p.Command() != "sudo apt install --only-upgrade eipgam" {
		t.Fatalf("%+v", p)
	}
	if p := packagedIn(root, "linux", "/usr/bin/ls"); p == nil || p.Package != "coreutils" {
		t.Fatalf("merged /usr: %+v", p)
	}
	if p := packagedIn(root, "linux", "/usr/sbin/eipgam"); p != nil {
		t.Fatalf("not listed: %+v", p)
	}
}

func TestPackagedByRpm(t *testing.T) {
	root := t.TempDir()
	was := rpmOwner
	t.Cleanup(func() { rpmOwner = was })
	asked := ""
	rpmOwner = func(path string) string { asked = path; return "eipgam" }
	// no rpm database: rpm isn't asked
	if p := packagedIn(root, "linux", "/usr/bin/eipgam"); p != nil || asked != "" {
		t.Fatalf("no rpm here: %+v, asked %q", p, asked)
	}
	if err := os.MkdirAll(filepath.Join(root, "var/lib/rpm"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := packagedIn(root, "linux", "/usr/bin/eipgam")
	if p == nil || p.Manager != "rpm" || asked != "/usr/bin/eipgam" || p.Command() != "sudo dnf upgrade eipgam" {
		t.Fatalf("%+v, asked %q", p, asked)
	}
	rpmOwner = func(string) string { return "" }
	if p := packagedIn(root, "linux", "/usr/bin/eipgam"); p != nil {
		t.Fatalf("not owned: %+v", p)
	}
}

func TestPackagedByHomebrewAndNix(t *testing.T) {
	root := t.TempDir()
	for exe, want := range map[string]string{
		"/opt/homebrew/Cellar/eipgam/0.1.126/bin/eipgam":              "homebrew eipgam brew upgrade eipgam",
		"/home/linuxbrew/.linuxbrew/Cellar/eipgam/0.1.126/bin/eipgam": "homebrew eipgam brew upgrade eipgam",
		"/nix/store/abc123-eipgam-0.1.2342/bin/eipgam":                "nix  ",
	} {
		goos := "linux"
		if strings.HasPrefix(exe, "/opt/homebrew") {
			goos = "darwin"
		}
		p := packagedIn(root, goos, exe)
		if p == nil || p.Manager+" "+p.Package+" "+p.Command() != want {
			t.Fatalf("%s: %+v", exe, p)
		}
	}
	if p := packagedIn(root, "darwin", "/Applications/eipgam.app/Contents/MacOS/eipgam"); p != nil {
		t.Fatalf("the app: %+v", p)
	}
}

// Scoop's app folder as `scoop install eipgam-app` from the eipgam bucket
// leaves it: the version folder holds the binary with the
// scoop-install.json and scoop-manifest.json Scoop writes (seen on
// xiaomi-gpu, Scoop 0.6.0), install.json and manifest.json before 0.6,
// and current is a junction to it.
func TestPackagedByScoop(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scoop")
	for i, v := range []string{"0.1.4478", "current"} {
		dir := filepath.Join(root, "apps", "eipgam-app", v)
		pre := []string{"scoop-", ""}[i]
		writeFile(t, filepath.Join(dir, "eipgam.exe"), "MZ")
		writeFile(t, filepath.Join(dir, pre+"install.json"), `{"bucket":"eipgam","architecture":"64bit"}`)
		writeFile(t, filepath.Join(dir, pre+"manifest.json"), `{"version":"0.1.4478"}`)
		p := packagedIn("/", "windows", filepath.Join(dir, "eipgam.exe"))
		if p == nil || p.Manager != "scoop" || p.Package != "eipgam-app" {
			t.Fatalf("%s: got %+v, want scoop's eipgam-app", v, p)
		}
		if c := p.Command(); c != "scoop update eipgam-app" {
			t.Fatalf("command %q", c)
		}
		if s := p.Say(); !strings.Contains(s, "Scoop") || !strings.HasSuffix(s, "scoop update eipgam-app") {
			t.Fatalf("say %q", s)
		}
	}
	// a eipgam.exe kept by hand in a folder named apps, without Scoop's
	// files, is eipgam's to update
	own := filepath.Join(t.TempDir(), "apps", "eipgam", "bin")
	writeFile(t, filepath.Join(own, "eipgam.exe"), "MZ")
	writeFile(t, filepath.Join(own, "manifest.json"), `{}`)
	writeFile(t, filepath.Join(own, "scoop-install.json"), `{}`)
	if p := packagedIn("/", "windows", filepath.Join(own, "eipgam.exe")); p != nil {
		t.Fatalf("own folder: got %+v", p)
	}
	// only on Windows
	dir := filepath.Join(root, "apps", "eipgam-app", "current")
	if p := packagedIn("/", "linux", filepath.Join(dir, "eipgam.exe")); p != nil {
		t.Fatalf("linux: got %+v", p)
	}
}

// winget's portable package folder, as `winget install SichengChen.eipgam`
// leaves it per user and machine-wide.
func TestPackagedByWinget(t *testing.T) {
	for _, exe := range []string{
		`C:\Users\yeton\AppData\Local\Microsoft\WinGet\Packages\SichengChen.eipgam_Microsoft.Winget.Source_8wekyb3d8bbwe\eipgam.exe`,
		`C:\Program Files\WinGet\Packages\SichengChen.eipgam_Microsoft.Winget.Source_8wekyb3d8bbwe\eipgam.exe`,
		// winget install --manifest, as seen on xiaomi-gpu (winget 1.29)
		`C:\Users\yeton\AppData\Local\Microsoft\WinGet\Packages\SichengChen.eipgam__DefaultSource\eipgam.exe`,
	} {
		p := packagedIn("/", "windows", exe)
		if p == nil || p.Manager != "winget" || p.Package != "SichengChen.eipgam" {
			t.Fatalf("%s: got %+v", exe, p)
		}
		if c := p.Command(); c != "winget upgrade --id SichengChen.eipgam" {
			t.Fatalf("command %q", c)
		}
	}
	if p := packagedIn("/", "windows", `C:\Users\yeton\Downloads\eipgam-windows-amd64.exe`); p != nil {
		t.Fatalf("downloaded: got %+v", p)
	}
	p := packagedIn("/", "windows", `C:\Users\yeton\AppData\Local\Microsoft\WinGet\Packages\odd\eipgam.exe`)
	if p == nil || p.Manager != "winget" || p.Command() != "winget upgrade eipgam" {
		t.Fatalf("unknown id: got %+v", p)
	}
}

// A path whose links can't be resolved is kept as given, so a Scoop
// install's current junction (which EvalSymlinks fails on) is still
// judged by PackagedBy, instead of Executable failing and the check being
// skipped.
func TestResolvedKeepsUnresolvablePath(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone", "eipgam.exe")
	if got := resolved(missing); got != missing {
		t.Fatalf("resolved(%q) = %q", missing, got)
	}
	real := filepath.Join(dir, "eipgam")
	writeFile(t, real, "x")
	want, _ := filepath.EvalSymlinks(real)
	if got := resolved(real); got != want {
		t.Fatalf("resolved(%q) = %q, want %q", real, got, want)
	}
}
