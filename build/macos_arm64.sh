GOARCH=arm64 GOOS=darwin go build -o release/Emissary_macos_arm64_$1 -ldflags="-s -w" .
zip -r ./release/Emissary_macos_arm64_$1.zip release/Emissary_macos_arm64_$1
gpg --output release/Emissary_macos_arm64_$1.zip.sig --detach-sig release/Emissary_macos_arm64_$1.zip
