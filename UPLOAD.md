# GitHub upload instructions

From inside the extracted project directory:

```bash
git init
git branch -M main
git add .
git commit -m "Release TixoTunnel NEXUS 1.0.0"
git remote add origin https://github.com/TheTixoCloud/TixoTunnel.git
git push -u origin main --force
```

Create and push the version tag:

```bash
git tag -a v1.0.0 -m "TixoTunnel NEXUS 1.0.0"
git push origin v1.0.0 --force
```

The repository must keep these exact paths because the built-in updater downloads them directly:

```text
TixoTunnel.sh
core/tixotunnel-core
core/tixotunnel-core.engine
```
