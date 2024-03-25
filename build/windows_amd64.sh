CGO_ENABLED=1 CC="x86_64-w64-mingw32-gcc" GOARCH=amd64 GOOS=windows go build -o release/Emissary_windows_amd64_$1.exe -ldflags="-s -w" .
cd release
zip -r Emissary_windows_amd64_$1.zip Emissary_windows_amd64_$1.exe
gpg --armor --output Emissary_windows_amd64_$1.zip.asc --detach-sig Emissary_windows_amd64_$1.zip
