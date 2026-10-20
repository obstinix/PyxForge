#!/usr/bin/env bash
# Attaches PyxForge's Windows release files to the GitHub Release for a tag, and proves it.
#
#   release-assets.sh <tag> <version> <dir>
#
# <dir> holds PyxForge-Setup-<version>.exe, PyxForge-Portable-<version>.zip and SHA256SUMS.txt.
#
# - No release for the tag: one is created as a DRAFT (a pre-release when the version has a
#   suffix such as -rc.2) with the three files. Publishing stays a maintainer's step.
# - A release with all three files: nothing is uploaded; they are checked as below.
# - A release with none of them: they are uploaded. Its draft or published state is kept.
# - A release with only some of them: it stops. Mixing files from two builds would break
#   SHA256SUMS.txt, and replacing files is a maintainer's decision.
#
# Then it downloads the release's three files and checks them against SHA256SUMS.txt, and
# fails unless all three are attached and not empty. Any failed gh call fails the script.
# The result goes to the log and to a notice annotation (readable without signing in).
set -euo pipefail

tag=$1
version=$2
dir=$3
repo=${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is not set}
files=("PyxForge-Setup-$version.exe" "PyxForge-Portable-$version.zip" "SHA256SUMS.txt")

for f in "${files[@]}"; do
	if [ ! -s "$dir/$f" ]; then
		echo "::error::$dir/$f is missing or empty"
		exit 1
	fi
done
(cd "$dir" && sha256sum -c SHA256SUMS.txt)

if ! gh release view "$tag" --repo "$repo" --json tagName >/dev/null 2>&1; then
	echo "No release for $tag: creating a draft."
	flags=(--draft --verify-tag --title "PyxForge $version"
		--notes "Windows x64 installer and portable archive for PyxForge $version. Check the files against SHA256SUMS.txt. The installer is not code-signed yet, so Windows SmartScreen may warn on first run; see docs/INSTALL.md.")
	case "$version" in *-*) flags+=(--prerelease --latest=false) ;; esac
	gh release create "$tag" --repo "$repo" "${flags[@]}" "${files[@]/#/$dir/}"
else
	attached=$(gh release view "$tag" --repo "$repo" --json assets --jq '.assets[].name')
	present=()
	missing=()
	for f in "${files[@]}"; do
		if grep -qxF "$f" <<<"$attached"; then present+=("$f"); else missing+=("$f"); fi
	done
	if [ ${#missing[@]} -eq 0 ]; then
		echo "The release for $tag already has all three files; nothing uploaded."
	elif [ ${#present[@]} -eq 0 ]; then
		echo "Uploading to the existing release for $tag: ${missing[*]}"
		gh release upload "$tag" --repo "$repo" "${missing[@]/#/$dir/}"
	else
		echo "::error::The release for $tag has ${present[*]} but not ${missing[*]}. Files from two builds would not match SHA256SUMS.txt: delete the attached ones on the release page, then run this again."
		exit 1
	fi
fi

# Check what the release really holds, by downloading it.
check=$(mktemp -d)
trap 'rm -rf "$check"' EXIT
for f in "${files[@]}"; do
	gh release download "$tag" --repo "$repo" --dir "$check" --pattern "$f"
	if [ ! -s "$check/$f" ]; then
		echo "::error::$f is not attached to the release for $tag, or is empty"
		exit 1
	fi
done
(cd "$check" && sha256sum -c SHA256SUMS.txt)

info=$(gh release view "$tag" --repo "$repo" --json url,isDraft,isPrerelease \
	--jq '"\(.url) (draft: \(.isDraft), pre-release: \(.isPrerelease))"')
sizes=$(cd "$check" && for f in "${files[@]}"; do printf '%s %s bytes; ' "$f" "$(wc -c <"$f" | tr -d ' ')"; done)
echo "Release for $tag: $info"
echo "Attached and checked: $sizes"
echo "::notice title=Release $tag::$info. Attached and checked against SHA256SUMS.txt: $sizes"
