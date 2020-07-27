mkdir -p build
env GOOS=windows GOARCH=amd64 go build -o ./build/stClient-win.exe ./client
env GOOS=linux GOARCH=amd64 go build -o ./build/stClient-linux ./client
env GOOS=darwin GOARCH=amd64 go build -o ./build/stClient-mac ./client
env GOOS=windows GOARCH=amd64 go build -o ./build/stServer-win.exe ./server
env GOOS=linux GOARCH=amd64 go build -o ./build/stServer-linux ./server
env GOOS=darwin GOARCH=amd64 go build -o ./build/stServer-mac ./server
ls -alh ./build
mkdir -p output
cp ./cfip.txt ./output/cfip.txt
zip -q output/stServer-win.zip ./build/stServer-win.exe -j
zip -q output/stServer-mac.zip ./build/stServer-mac -j
zip -q output/stServer-linux.zip ./build/stServer-linux -j
zip -q output/stClient-win.zip ./build/stClient-win.exe -j
zip -q output/stClient-mac.zip ./build/stClient-mac -j
zip -q output/stClient-linux.zip ./build/stClient-linux -j
ls -alh ./output
