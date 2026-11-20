#!/bin/bash
# Combine both native builds into the shared signer's handoff.
set -euo pipefail

arm_app="${1:?Apple silicon app bundle}"
intel_app="${2:?Intel app bundle}"
version="${3:?release version}"
destination="${4:?artifact directory}"

for bundle in "$arm_app" "$intel_app"; do
  codesign --verify --deep --strict "$bundle"
  test "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$bundle/Contents/Info.plist")" = "$version"
done
diff -qr "$arm_app/Contents/Resources" "$intel_app/Contents/Resources"
cmp "$arm_app/Contents/Info.plist" "$intel_app/Contents/Info.plist"
test "$(lipo -archs "$arm_app/Contents/MacOS/eipgam")" = arm64
test "$(lipo -archs "$intel_app/Contents/MacOS/eipgam")" = x86_64

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
app="$work/eipgam.app"
ditto "$arm_app" "$app"
lipo -create "$arm_app/Contents/MacOS/eipgam" "$intel_app/Contents/MacOS/eipgam" \
  -output "$app/Contents/MacOS/eipgam"
for arch in arm64 x86_64; do
  lipo "$app/Contents/MacOS/eipgam" -verify_arch "$arch"
done
codesign --force --sign - --identifier com.sichengchen.eipgam "$app"
codesign --verify --deep --strict "$app"

mkdir -p "$destination"
ditto -c -k --norsrc --keepParent "$app" "$destination/app.zip"
jq -n --arg sha "$(git rev-parse HEAD)" --arg version "$version" \
  '{sha: $sha, version: $version}' > "$destination/release.json"
