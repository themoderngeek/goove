---
### Installing

These binaries are unsigned. After download:
```bash
tar -xzf goove-vX.Y.Z-darwin-arm64.tar.gz
xattr -d com.apple.quarantine ./goove   # bypass Gatekeeper for unsigned binaries
./goove
```
