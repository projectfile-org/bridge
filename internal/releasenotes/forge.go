// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/readme"
	"projectfile.org/projectfile/bridge/internal/derive"
	"projectfile.org/projectfile/bridge/internal/derive/ocisinks"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// ForgeMarker opens the forge half in a release body; publish replaces everything after it.
const ForgeMarker = "<!-- pf-bridge release-notes forge -->"

const (
	cosignKeyPath = "org.projectfile.signing.cosign.public-key"
	cosignTlog    = "org.projectfile.signing.cosign.tlog"
	signedNode    = "org.projectfile.ci.nodes.image-is-signed"
	attestedNode  = "org.projectfile.ci.nodes.image-is-attested"
	gpgReleaseKey = "${people[0].handles.gpg-key.release}"
	hostCommand   = "${org.projectfile.artifacts{kind=binary}.command}"
	// hostSignatureURL mirrors the release-binary install command in m6e/library traits/binary-release.yaml.
	hostSignatureURL = "${org.projectfile.forge.remotes.github.url}/releases/${org.projectfile.readme.download}/${org.projectfile.artifacts{kind=binary}.host-asset}.asc"
	binaryEntry      = "release-binary"
	binaryPrefix     = "Download the prebuilt binary for your platform from this release:"
)

// ForgeOptions names the forge whose release the forge half describes and what that release holds.
type ForgeOptions struct {
	Options
	Forge    string
	Server   string
	Repo     string
	Previous string
	Assets   []string
	Magnets  map[string]string
	// Inspect reads one image reference from its registry; nil omits digests and the platform table.
	Inspect func(ref string) (ImageInfo, error)
}

// RenderForge returns the forge half: what this forge published for the tag and how to fetch and verify it.
func RenderForge(fo ForgeOptions) (string, error) {
	if fo.Timeout == 0 {
		fo.Timeout = defaultTimeout
	}
	pf, _, err := projectfile.ReadWithOptions(fo.Dir, fo.Read)
	if err != nil {
		return "", fmt.Errorf("read projectfile: %w", err)
	}
	narrowPublish(pf, fo.Forge)
	pinRelease(pf, fo.Options)
	derive.AddVirtual(pf)
	repoURL := strings.TrimSuffix(fo.Server, "/") + "/" + fo.Repo
	setPath(pf.Extensions, repoURL, strings.Split("org.projectfile.forge.remotes.github.url", ".")...)
	setBinaryPrefix(pf)
	applyReleaseProse(pf)

	var b strings.Builder
	b.WriteString(ForgeMarker + "\n")
	blocks, err := readme.Blocks(pf, fo.Dir, "installation", "usage")
	if err != nil {
		return "", err
	}
	if blocks != "" {
		b.WriteString("\n" + blocks + "\n")
	}
	prev := fo.Previous
	if prev == "" {
		if prev, err = previousTag(git{dir: fo.Dir, timeout: fo.Timeout}, fo.Tag, fo.Tag, fo.Prefix); err != nil {
			genlog.Warn("compare link omitted, previous tag unreadable", "tag", fo.Tag, "err", err.Error())
		}
	}
	images := inspectSinks(pf, fo.Inspect, strings.TrimPrefix(prev, fo.Prefix))
	writeDigests(&b, images)
	writeVerify(&b, pf, fo.Assets, images)
	writeTorrents(&b, fo.Magnets, hostmatch.ResolveKind(fo.Server) == hostmatch.KindGitHub)
	if prev != "" {
		fmt.Fprintf(&b, "\n**Full changes:** %s/compare/%s...%s\n", repoURL, prev, fo.Tag)
	}
	return b.String(), nil
}

// narrowPublish keeps only this forge's publish route, so every sink another forge pushes to drops out.
func narrowPublish(pf *projectfile.Document, forge string) {
	raw, ok := projectfile.LookupExtension(pf, pfmodel.PublishExtensionNS)
	routes, _ := raw.(map[string]any)
	if !ok || routes[forge] == nil {
		genlog.Warn("forge has no publish route, every sink kept", "forge", forge)
		return
	}
	for name := range routes {
		if name != forge {
			delete(routes, name)
			genlog.Debug("publish route dropped", "route", name, "forge", forge)
		}
	}
}

// pinRelease points every image tag, readme tag and download at this release.
func pinRelease(pf *projectfile.Document, opts Options) {
	for _, pin := range []struct{ value, path string }{
		{strings.TrimPrefix(opts.Tag, opts.Prefix), "org.projectfile.image.tag"},
		{opts.Tag, "org.projectfile.readme.tag"},
		{"download/" + opts.Tag, "org.projectfile.readme.download"},
	} {
		setPath(pf.Extensions, pin.value, strings.Split(pin.path, ".")...)
		genlog.Debug("install pinned", "path", pin.path, "value", pin.value)
	}
	pinLadder(pf, strings.TrimPrefix(opts.Tag, opts.Prefix))
}

// stableVersion matches a release version with no prerelease or build suffix.
var stableVersion = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

// pinLadder names this release's shorter tags in readme.ladder; a prerelease publishes none, so prose naming them drops.
func pinLadder(pf *projectfile.Document, version string) {
	ladder := map[string]any{}
	if m := stableVersion.FindStringSubmatch(version); m != nil {
		ladder = map[string]any{"minor": m[1] + "." + m[2], "major": m[1]}
	}
	readme, _ := projectfile.LookupExtension(pf, pfmodel.ReadmeExtensionNS)
	if m, ok := readme.(map[string]any); ok {
		m["ladder"] = ladder
	}
	genlog.Debug("ladder pinned", "version", version, "tags", len(ladder))
}

// setBinaryPrefix rewords the binary block for a release page, which is the download.
func setBinaryPrefix(pf *projectfile.Document) {
	raw, _ := projectfile.LookupExtension(pf, pfmodel.ReadmeExtensionNS+".installation")
	entries, _ := raw.([]any)
	for _, e := range entries {
		if m, ok := e.(map[string]any); ok && m["name"] == binaryEntry {
			m["prefix"] = map[string]any{"en": binaryPrefix}
			genlog.Debug("binary prefix reworded", "entry", binaryEntry)
		}
	}
}

// applyReleaseProse swaps in the prefix and postfix an installation entry declares under `release` for a release page.
func applyReleaseProse(pf *projectfile.Document) {
	raw, _ := projectfile.LookupExtension(pf, pfmodel.ReadmeExtensionNS+".installation")
	entries, _ := raw.([]any)
	for _, e := range entries {
		m, _ := e.(map[string]any)
		release, ok := m["release"].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"prefix", "postfix"} {
			if v, ok := release[key]; ok {
				m[key] = v
				genlog.Debug("release prose applied", "entry", m["name"], "field", key)
			}
		}
		delete(m, "release")
	}
}

// writeVerify lists the cosign and gpg commands that prove the published artifacts, one subsection per kind.
func writeVerify(b *strings.Builder, pf *projectfile.Document, assets []string, images []sinkImage) {
	image, binary := imageVerify(pf, images), binaryVerify(pf, assets)
	if len(image) == 0 && len(binary) == 0 {
		return
	}
	b.WriteString("\n## Verify\n\nCheck that what you downloaded is what this project published.\n")
	if len(image) > 0 {
		proof := "The signature proves this project’s CI built and pushed the image."
		if _, attested := projectfile.LookupExtension(pf, attestedNode); attested {
			proof = "The signature proves this project’s CI built and pushed the image; the attestation carries its software bill of materials."
		}
		b.WriteString("\n### Container image\n\n" + proof + "\n\n```sh\n" + strings.Join(image, "\n") + "\n```\n")
	}
	if len(binary) > 0 {
		b.WriteString("\n### Prebuilt binary\n\nThe signature proves the binary is the one the maintainer’s release key signed.\n\n```sh\n" + strings.Join(binary, "\n") + "\n```\n")
	}
}

// imageVerify returns the cosign commands for every sink ref, naming the key once.
func imageVerify(pf *projectfile.Document, images []sinkImage) []string {
	key, _ := projectfile.LookupExtension(pf, cosignKeyPath)
	keyURL, _ := key.(string)
	_, signed := projectfile.LookupExtension(pf, signedNode)
	_, attested := projectfile.LookupExtension(pf, attestedNode)
	genlog.Debug("cosign verify", "signed", signed, "attested", attested, "key", or(keyURL, "<none>"))
	if !signed || keyURL == "" {
		return nil
	}
	flags := `--key "$COSIGN_KEY"`
	// A signature never uploaded to Rekor verifies only with the transparency-log check off.
	if tlog, _ := projectfile.LookupExtension(pf, cosignTlog); tlog != true {
		flags += " --insecure-ignore-tlog=true"
	}
	lines := []string{"COSIGN_KEY=" + keyURL}
	refs := sinkRefs(pf)
	for i, ref := range refs {
		refs[i] = pinned(ref, images)
	}
	for _, ref := range refs {
		lines = append(lines, "cosign verify "+flags+" "+ref)
	}
	if attested {
		for _, ref := range refs {
			lines = append(lines, "cosign verify-attestation "+flags+" --type cyclonedx "+ref)
		}
	}
	return lines
}

// binaryVerify returns the gpg commands for the host binary the install block downloaded, else for every signed asset.
func binaryVerify(pf *projectfile.Document, assets []string) []string {
	var sigs []string
	for _, a := range assets {
		if strings.HasSuffix(a, ".asc") {
			sigs = append(sigs, a)
		}
	}
	fpr, ok := interp.ExpandChecked(pf, gpgReleaseKey)
	genlog.Debug("gpg verify", "signatures", len(sigs), "key", ok)
	if len(sigs) == 0 || !ok || fpr == "" {
		return nil
	}
	lines := []string{"gpg --keyserver hkps://keys.openpgp.org --recv-keys " + fpr}
	if cmd, sig, ok := hostSignature(pf); ok {
		return append(lines, "curl --fail --location --output "+cmd+".asc "+sig, "gpg --verify "+cmd+".asc "+cmd)
	}
	for _, s := range sigs {
		lines = append(lines, "gpg --verify "+s+" "+strings.TrimSuffix(s, ".asc"))
	}
	return lines
}

// hostSignature resolves the installed file name and the URL of its host asset’s signature.
func hostSignature(pf *projectfile.Document) (cmd, url string, ok bool) {
	cmd, okCmd := interp.ExpandChecked(pf, hostCommand)
	url, okURL := interp.ExpandChecked(pf, hostSignatureURL)
	genlog.Debug("host signature", "command", cmd, "resolved", okCmd && okURL)
	return cmd, url, okCmd && okURL && cmd != ""
}

// sinkRefs returns the composed image ref of every sink this forge pushed to, in name order.
func sinkRefs(pf *projectfile.Document) []string {
	var refs []string
	for name, v := range ocisinks.Refs(pf) {
		entry, _ := v.(map[string]any)
		if ref, ok := entry[pfmodel.SinkRefKey].(string); ok && ref != "" {
			refs = append(refs, ref)
			genlog.Debug("verify ref", "sink", name, "ref", ref)
		}
	}
	slices.Sort(refs)
	return refs
}

// writeTorrents lists one magnet link per torrented asset, fenced where the forge strips the magnet scheme.
func writeTorrents(b *strings.Builder, magnets map[string]string, fenced bool) {
	if len(magnets) == 0 {
		return
	}
	names := make([]string, 0, len(magnets))
	for name := range magnets {
		names = append(names, name)
	}
	slices.Sort(names)
	b.WriteString("\n## Download via BitTorrent\n\nFetch this release over BitTorrent with a magnet link, or with the attached `.torrent` file:\n\n")
	for _, name := range names {
		label, uri := strings.TrimSuffix(name, ".magnet"), strings.TrimSpace(magnets[name])
		if fenced {
			fmt.Fprintf(b, "- 🧲 %s\n\n  ```text\n  %s\n  ```\n\n", label, uri)
			continue
		}
		fmt.Fprintf(b, "- [🧲 %s](%s)\n", label, uri)
	}
	genlog.Debug("torrents listed", "count", len(names), "fenced", fenced)
}

// MergeBody replaces the forge half of body with half, keeping the tag's human half above it.
func MergeBody(body, half string) string {
	if i := strings.Index(body, ForgeMarker); i >= 0 {
		body = body[:i]
	}
	return strings.TrimRight(body, "\n") + "\n\n" + strings.TrimRight(half, "\n") + "\n"
}

// sinkImage is one sink's image reference with what its registry holds for it.
type sinkImage struct {
	Ref  string
	Info ImageInfo
	Prev *ImageInfo
}

// inspectSinks reads every sink ref, and its previous version's, through inspect; an unreadable ref is skipped, so the notes degrade to tags.
func inspectSinks(pf *projectfile.Document, inspect func(string) (ImageInfo, error), prevVersion string) []sinkImage {
	if inspect == nil {
		return nil
	}
	var out []sinkImage
	for _, ref := range sinkRefs(pf) {
		info, err := inspect(ref)
		if err != nil || info.Digest == "" {
			genlog.Warn("image digest omitted, registry unreadable", "ref", ref, "err", fmt.Sprint(err))
			continue
		}
		si := sinkImage{Ref: ref, Info: info}
		if prevVersion != "" {
			if p, err := inspect(repoOf(ref) + ":" + prevVersion); err == nil {
				si.Prev = &p
			} else {
				genlog.Debug("previous image unreadable, no size delta", "ref", ref, "previous", prevVersion, "err", err.Error())
			}
		}
		out = append(out, si)
		genlog.Debug("image inspected", "ref", ref, "digest", info.Digest, "platforms", len(info.Platforms))
	}
	return out
}

// repoOf drops the tag or digest from a reference.
func repoOf(ref string) string {
	ref, _, _ = strings.Cut(ref, "@")
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	return ref
}

// pinned returns ref as repo@digest when its registry was read, else ref unchanged.
func pinned(ref string, images []sinkImage) string {
	for _, im := range images {
		if im.Ref == ref {
			return repoOf(ref) + "@" + im.Info.Digest
		}
	}
	return ref
}

// writeDigests pins each sink's image by digest and tabulates the platforms of the first multi-arch one.
func writeDigests(b *strings.Builder, images []sinkImage) {
	if len(images) == 0 {
		return
	}
	b.WriteString("\n## Image digests\n\nA tag can move; a digest cannot. Pull this exact build:\n\n```sh\n")
	var table, prev *ImageInfo
	for i, im := range images {
		fmt.Fprintf(b, "docker pull %s@%s\n", repoOf(im.Ref), im.Info.Digest)
		if table == nil && len(im.Info.Platforms) > 0 {
			table, prev = &images[i].Info, images[i].Prev
		}
	}
	b.WriteString("```\n")
	if table == nil {
		return
	}
	b.WriteString("\n| Platform | Digest | Size |\n| --- | --- | --- |\n")
	for _, p := range table.Platforms {
		fmt.Fprintf(b, "| %s | `%s` | %s |\n", p.Name, p.Digest, sizeCell(p, prev))
	}
	b.WriteString("\nSizes are compressed layer bytes as stored in the registry.\n")
	if prev != nil {
		writePackageDelta(b, prev.Packages, table.Packages)
	}
}

// maxChangedListed caps the version changes named one by one; the rest are only counted.
const maxChangedListed = 12

// writePackageDelta states which packages the SBOM gained, lost or moved to another version since the previous release.
func writePackageDelta(b *strings.Builder, before, after map[string]string) {
	if before == nil || after == nil {
		return
	}
	var added, removed, changed []string
	for name, v := range after {
		switch old, ok := before[name]; {
		case !ok:
			added = append(added, name)
		case old != v:
			changed = append(changed, fmt.Sprintf("%s %s → %s", name, old, v))
		}
	}
	for name := range before {
		if _, ok := after[name]; !ok {
			removed = append(removed, name)
		}
	}
	genlog.Debug("package delta", "added", len(added), "removed", len(removed), "changed", len(changed))
	if len(added)+len(removed)+len(changed) == 0 {
		b.WriteString("\nNo package changed since the previous release.\n")
		return
	}
	slices.Sort(changed)
	fmt.Fprintf(b, "\n**Packages since the previous release:** %d updated, %d added, %d removed.\n", len(changed), len(added), len(removed))
	for i, c := range changed {
		if i == maxChangedListed {
			fmt.Fprintf(b, "- … and %d more\n", len(changed)-i)
			break
		}
		b.WriteString("- " + c + "\n")
	}
}

// sizeCell renders a platform's size, with the change against the same platform of the previous release when known.
func sizeCell(p Platform, prev *ImageInfo) string {
	if p.Size <= 0 {
		return "—"
	}
	cell := fmt.Sprintf("%.1f MB", float64(p.Size)/1e6)
	if prev == nil {
		return cell
	}
	for _, q := range prev.Platforms {
		if q.Name == p.Name && q.Size > 0 {
			return fmt.Sprintf("%s (%s)", cell, signedMB(p.Size-q.Size))
		}
	}
	return cell
}

// signedMB formats a byte delta in megabytes with an explicit sign and a true minus.
func signedMB(delta int64) string {
	if delta < 0 {
		return fmt.Sprintf("−%.1f MB", float64(-delta)/1e6)
	}
	return fmt.Sprintf("+%.1f MB", float64(delta)/1e6)
}
