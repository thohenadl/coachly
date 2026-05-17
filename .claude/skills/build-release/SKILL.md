---
name: build-release
description: Build a distributable release of coachly. Use when the user says "cut a release", "build for friends", "build the app", "make a release zip", or any variant asking for the packaged Mac/Windows zips. Runs the Tailwind build, then the platform build scripts in scripts/, and prints the final artifact sizes. Asks for a version number if not supplied.
---

# Skill: build-release

You are building a distributable release of coachly for Mac (and optionally Windows). Follow this exact sequence — do not skip steps and do not invent new ones.

## Pre-flight

1. Make sure `git status` is clean OR confirm with the user that uncommitted changes are intentional.
2. Ask the user for a `VERSION` if they haven't given one. Default suggestion: bump the patch of the last release tag (`git tag --sort=-creatordate | head -1`).

## Execute

Run from the repo root:

```bash
VERSION=<version> ./scripts/build-mac.sh
VERSION=<version> ./scripts/build-win.sh
```

The scripts already invoke `./scripts/tailwind.sh` internally — do not run it separately unless one of them fails.

## Verify

After both scripts succeed:

```bash
ls -lh dist/
```

Both `coachly-mac.zip` and `coachly-win.zip` must exist. Typical sizes are 9-12 MB for Mac and 5-6 MB for Windows. If a zip is significantly larger, something pulled in an unexpected dependency — investigate before sharing.

Quick sanity launch (Mac only, in the background):

```bash
open dist/coachly.app
sleep 3
pkill coachly
```

## Report back

Summarize: version, both zip sizes, path. Don't write any other files. Don't tag or push — the user does that.
