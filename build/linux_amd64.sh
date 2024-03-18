GOARCH=amd64 GOOS=linux go build -o release/Emissary_linux_amd64_$1 -ldflags="-s -w" .
zip -r ./release/Emissary_linux_amd64_$1.zip release/Emissary_linux_amd64_$1
gpg --output release/Emissary_linux_amd64_$1.zip.sig --detach-sig release/Emissary_linux_amd64_$1.zip
