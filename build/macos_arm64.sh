GOARCH=arm64 GOOS=darwin go build -o release/Emissary_macos_arm64_$1 -ldflags="-s -w" .
zip -r ./release/Emissary_macos_arm64_$1.zip release/Emissary_macos_arm64_$1
gpg --armor --output release/Emissary_macos_arm64_$1.zip.asc --detach-sig release/Emissary_macos_arm64_$1.zip
