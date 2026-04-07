Helm chart release for Ayatsuri.

This GitHub release publishes the packaged `ayatsuri` Helm chart for the Helm repository at `https://ayatsuricloud.github.io/ayatsuri`. It is not a Ayatsuri application binary release.

Install the chart with:

```bash
helm repo add ayatsuri https://ayatsuricloud.github.io/ayatsuri
helm repo update
helm install ayatsuri ayatsuri/ayatsuri --set persistence.storageClass=<your-rwx-storage-class>
```

Application releases use separate `vX.Y.Z` tags.
