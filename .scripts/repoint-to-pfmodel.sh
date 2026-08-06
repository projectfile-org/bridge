#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
# SPDX-License-Identifier: MIT
#
# repoint-to-pfmodel.sh — rewrite projectfile.<moved-symbol> → pfmodel.<symbol>
# across the bridge tree, and add the pfmodel import where it landed.
#
# Core 2.0 cut: the bridge-owned extension shapes/accessors/repository helpers
# moved from core/internal/projectfile to bridge/internal/pfmodel. Every call
# site that used to read them off the projectfile façade now reaches them in
# pfmodel. This script does the mechanical rewrite; the import-block tidy is
# left to `goimports`/`gofmt` (or done manually if goimports is unavailable).
#
# Usage: .scripts/repoint-to-pfmodel.sh [file...]
#   no args  → scan every *.go under internal/ (non-test) plus the listed tests
#   file ... → operate only on the listed files
set -euo pipefail

# Symbols that moved from core (projectfile.X) to pfmodel (pfmodel.X).
# Keep in sync with bridge/internal/pfmodel/*.go.
MOVED_SYMBOLS=(
	# accessors
	GetCitationExtension GetCodeOfConductExtension GetCodeOwnersExtension
	GetContributingExtension GetConventionsExtension GetReadmeExtension
	GetReleaseExtension GetSupportExtension GetSecurityExtension
	GetCLIExtension SetCLIExtension GetForgeExtension GetFundingExtension
	GetIgnoresExtension GetEditorsExtension GetVulnerabilitiesExtension
	ConventionsStyleGuideURL HasExtension
	# repository/link helpers
	PrimaryRepository EnsurePrimaryRepository IssuesRepository
	CitableRepositoryURL EnsureCitableSourceLink
	LinkByType LinkURL LinksByType SetLink AddLink
	# people/copyright/display helpers
	CopyrightHolders CopyrightHolderNames FlatPersonName
	ForgePersonHandle ForgeOrgHandle ContactEmail DisplayName
	# types
	CitationExtension PreferredCitation ReadmeExtension ReadmeExtra Shield
	CodeOwnersExtension CodeOwnersEntry CodeOfConductExtension
	ContributingExtension ConventionsExtension LangConventions
	SupportExtension EOLEntry ReleaseExtension ReleaseBranch
	SecurityExtension CLIExtension CLIDeriveToggles
	ForgeExtension ForgeFieldsToggles EditorsExtension
	FundingExtension FundingChannel FundingPlan FundingHistory
	IgnoresExtension IgnoreTargetOverride
	VulnerabilitiesExtension VulnerabilitySuppress
	# namespace constants
	ForgeExtensionNS FundingExtensionNS SecurityExtensionNS
	IgnoresExtensionNS VulnerabilitiesExtensionNS ReleaseExtensionNS
	EditorsExtensionNS CodeOwnersExtensionNS ContributingExtensionNS
	ReadmeExtensionNS
)

# Build a single sed expression file so we make one pass per file.
EXPR_FILE=$(mktemp)
trap 'rm -f "$EXPR_FILE"' EXIT
for sym in "${MOVED_SYMBOLS[@]}"; do
	# Word-boundary rewrite: projectfile.<sym> → pfmodel.<sym>.
	# printf interprets \b as backspace, so escape it as \\b to emit the
	# literal two-char sequence sed needs for its word-boundary anchor.
	printf 's/\\bprojectfile\.%s\\b/pfmodel.%s/g\n' "$sym" "$sym" >> "$EXPR_FILE"
done

# Decide the file list.
if [[ $# -gt 0 ]]; then
	FILES=("$@")
else
	# Every .go under internal/ — test files included, the rewrite is the same.
	mapfile -t FILES < <(fd -e go . internal --hidden)
fi

changed=0
for f in "${FILES[@]}"; do
	if grep -qE '\bpfmodel\.' "$f" || ! sed -n '1,40p' "$f" | grep -q 'projectfile'; then
		# Quick pre-check: only touch files that actually reference a moved symbol.
		:
	fi
	if sed -Ef "$EXPR_FILE" "$f" | cmp -s - "$f"; then
		continue
	fi
	sed -i -Ef "$EXPR_FILE" "$f"
	changed=$((changed + 1))
	echo "rewrote: $f"
done
echo "done: $changed file(s) rewritten"
echo "NOTE: add the pfmodel import where needed, then run gofmt."
