# Releasing The Ayatsuri Helm Chart

This document covers publication of the Helm chart in `charts/ayatsuri`.

## One-Time Repository Setup

1. Create a `gh-pages` branch in `ayatsuri-lab/ayatsuri`.
2. In GitHub repository settings, enable GitHub Pages from the `gh-pages` branch root.
3. Ensure GitHub Actions can write repository contents with `GITHUB_TOKEN`.

The publish workflow in this repository assumes the `gh-pages` branch already exists. That matches the upstream `helm/chart-releaser-action` prerequisites.

## What Triggers A Chart Release

- `.github/workflows/chart-release.yaml` runs on pushes to `main` that touch packaged chart files in `charts/ayatsuri`.
- A new chart release is created only when `charts/ayatsuri/Chart.yaml` contains a chart `version` that has not already been published.
- Chart releases are named `helm-ayatsuri-<chart-version>`.
- The workflow sets `skip_existing: true` so reruns do not fail if the GitHub release tag already exists.
- The workflow sets `mark_as_latest: false` so chart releases do not replace the repository's application release as GitHub's "latest release".
- The release name template and fixed chart release body are defined in `cr.yaml` and `charts/ayatsuri/RELEASE.md`.

## Before Merging A Chart Release

1. Update `charts/ayatsuri/Chart.yaml -> version`.
   Any change to packaged chart files, including `charts/ayatsuri/README.md` and `charts/ayatsuri/RELEASE.md`, requires a new chart version.
2. Update `values.yaml -> image.tag` only if you want the chart default image to change from `latest` to a different tag.
3. Ensure the chart CI workflow passes:
   - `helm lint ./charts/ayatsuri`
   - `helm template ayatsuri ./charts/ayatsuri --set persistence.storageClass=nfs-client`
   - `helm package ./charts/ayatsuri`

`charts/ayatsuri/RELEASING.md` is ignored by `.helmignore` and is not part of the packaged chart. `charts/ayatsuri/RELEASE.md` is packaged and becomes the GitHub release body for chart releases.

## After Merge

1. Confirm the `ReleaseHelmCharts` workflow succeeded on `main`.
2. Confirm a GitHub Release named `helm-ayatsuri-<chart-version>` exists and includes the `.tgz` package.
3. Confirm `gh-pages/index.yaml` contains the new chart version.
4. Confirm the published repository works:

```bash
helm repo add ayatsuri https://ayatsuricloud.github.io/ayatsuri
helm repo update
helm search repo ayatsuri
helm pull ayatsuri/ayatsuri --version <chart-version>
```

## Removing A Broken Chart Version

No automation is provided for yanking a published chart version.

If a published version must be removed:

1. Delete the GitHub Release `helm-ayatsuri-<chart-version>` and its chart asset.
2. Remove that version entry from `gh-pages/index.yaml`.
3. Commit the `index.yaml` change to `gh-pages`.
4. Publish a new chart version. Do not reuse the removed version number.
