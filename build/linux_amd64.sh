GOARCH=amd64 GOOS=linux go build -o release/Emissary_linux_amd64_$1 -ldflags="-s -w" .
cd release
zip -r Emissary_linux_amd64_$1.zip Emissary_linux_amd64_$1
gpg --armor --output Emissary_linux_amd64_$1.zip.asc --detach-sig Emissary_linux_amd64_$1.zip
