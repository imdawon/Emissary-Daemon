CGO_ENABLED=1 CC="x86_64-w64-mingw32-gcc" GOARCH=amd64 GOOS=windows go build -o release/Emissary_windows_amd64_$1.exe -ldflags="-s -w" .
zip -r ./release/Emissary_windows_amd64_$1.zip release/Emissary_windows_amd64_$1.exe
gpg --output release/Emissary_windows_amd64_$1.zip.sig --detach-sig release/Emissary_windows_amd64_$1.zip
