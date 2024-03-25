GOARCH=arm64 GOOS=darwin go build -o release/Emissary_macos_arm64_$1 -ldflags="-s -w" .
cd release
pwd
zip -r Emissary_macos_arm64_$1.zip Emissary_macos_arm64_$1
gpg --armor --output Emissary_macos_arm64_$1.zip.asc --detach-sig Emissary_macos_arm64_$1.zip
