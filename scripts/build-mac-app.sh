#!/usr/bin/env bash
set -euo pipefail
# Local technical preview: no Developer ID, notarization, services or model calls.
[[ "$(uname -s)" == Darwin ]] || { echo 'macOS is required' >&2; exit 1; }
[[ $# == 3 ]] || { echo 'Usage: build-mac-app.sh OUTPUT_DIRECTORY PREVIEW_VERSION SOURCE_SHA' >&2; exit 1; }
out="$1"; version="$2"; revision="$3"
[[ "$out" == /* && "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+-preview[0-9A-Za-z.-]*$ && "$revision" =~ ^[0-9a-f]{40}$ ]] || { echo 'Absolute output, preview version and full SHA required' >&2; exit 1; }
app="$out/Agent Messenger.app"
[[ ! -e "$app" && ! -e "$out/Agent-Messenger-mac.zip" ]] || { echo 'Output already exists; refusing overwrite' >&2; exit 1; }
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
go_bin="${GO_BIN:-go}"
sparkle="${SPARKLE_DIR:-$out/sparkle-sdk}"
bash scripts/fetch-sparkle.sh "$sparkle"
public_key="$(cat desktop/macos/update-public-key.txt)"
[[ "$public_key" =~ ^[A-Za-z0-9/+]{43}=$ ]] || { echo 'Invalid update public key' >&2; exit 1; }
CGO_ENABLED=0 "$go_bin" build -trimpath -buildvcs=false -ldflags "-X github.com/kolotiloff/agent-messenger/internal/buildinfo.Version=$version -X github.com/kolotiloff/agent-messenger/internal/buildinfo.Revision=$revision -X github.com/kolotiloff/agent-messenger/internal/buildinfo.UpdatePublicKey=$public_key" -o "$app/Contents/Resources/messenger" ./cmd/messenger
xcrun swiftc -target "$(uname -m)-apple-macos15.0" -swift-version 5 -O -F "$sparkle" -framework Sparkle -Xlinker -rpath -Xlinker @executable_path/../Frameworks -framework AppKit -framework WebKit desktop/macos/main.swift desktop/macos/Updater.swift -o "$app/Contents/MacOS/AgentMessenger"
cp desktop/macos/Info.plist "$app/Contents/Info.plist"
mkdir -p "$app/Contents/Frameworks"
ditto "$sparkle/Sparkle.framework" "$app/Contents/Frameworks/Sparkle.framework"
cp "$sparkle/LICENSE" "$app/Contents/Resources/Sparkle-LICENSE.txt"
/usr/libexec/PlistBuddy -c "Add :SUPublicEDKey string $public_key" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUEnableAutomaticChecks bool true" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUAllowsAutomaticUpdates bool false" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUAutomaticallyUpdate bool false" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUEnableSystemProfiling bool false" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUVerifyUpdateBeforeExtraction bool true" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :SUScheduledCheckInterval integer 3600" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :AMClientAPI integer 1" "$app/Contents/Info.plist"
# Keep the native About panel and future updater aligned with the bundled CLI.
# Non-numbered CI/development previews have build 0, never a release sequence.
build_number="${version##*.}"
[[ "$build_number" =~ ^[1-9][0-9]{0,8}$ ]] || build_number=0
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$app/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $build_number" "$app/Contents/Info.plist"
xcrun swiftc -swift-version 5 desktop/macos/icon.swift -o "$out/icon-generator"
"$out/icon-generator" "$out/AppIcon.iconset"
iconutil -c icns "$out/AppIcon.iconset" -o "$app/Contents/Resources/AppIcon.icns"
codesign --force --sign - "$app/Contents/Resources/messenger"
codesign --force --sign - "$app"
codesign --verify --deep --strict "$app"
"$app/Contents/Resources/messenger" version
/usr/bin/ditto -c -k --keepParent "$app" "$out/Agent-Messenger-mac.zip"
(cd "$out" && shasum -a 256 Agent-Messenger-mac.zip > Agent-Messenger-mac.zip.sha256)
printf 'Built: %s\n' "$app"
