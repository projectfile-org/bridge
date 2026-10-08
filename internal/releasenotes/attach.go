// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/core"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

const (
	attachNS      = pfmodel.ReleaseExtensionNS + ".attach"
	attachMaxSize = 100 << 20
)

// attachFiles resolves the `release.attach` globs against dir into regular files that stay inside it, sorted and unique by base name.
func attachFiles(dir string, read projectfile.ReadOptions) ([]string, error) {
	pf, _, err := projectfile.ReadWithOptions(dir, read)
	if err != nil {
		return nil, fmt.Errorf("read projectfile: %w", err)
	}
	raw, _ := projectfile.LookupExtension(pf, attachNS)
	patterns, _ := raw.([]any)
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	names := map[string]string{}
	for _, p := range patterns {
		pattern, _ := p.(string)
		if pattern == "" || filepath.IsAbs(pattern) || slices.Contains(strings.Split(filepath.ToSlash(pattern), "/"), "..") {
			genlog.Warn("attach pattern rejected, must be relative and stay inside the project", "pattern", pattern)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			genlog.Warn("attach pattern invalid", "pattern", pattern, "err", err.Error())
			continue
		}
		slices.Sort(matches)
		if len(matches) == 0 {
			genlog.Info("attach pattern matched nothing", "pattern", pattern)
		}
		for _, m := range matches {
			resolved, err := filepath.EvalSymlinks(m)
			info, statErr := os.Stat(resolved)
			switch {
			case err != nil || statErr != nil || !info.Mode().IsRegular():
				genlog.Debug("attach match skipped, not a regular file", "path", m)
			case !strings.HasPrefix(resolved, root+string(filepath.Separator)):
				genlog.Warn("attach match skipped, outside the project", "path", m, "resolved", resolved)
			case info.Size() > attachMaxSize:
				genlog.Warn("attach match skipped, too large", "path", m, "bytes", info.Size(), "limit", attachMaxSize)
			case names[filepath.Base(m)] != "":
				genlog.Warn("attach match skipped, name already taken", "path", m, "taken-by", names[filepath.Base(m)])
			default:
				names[filepath.Base(m)] = m
				files = append(files, m)
			}
		}
	}
	return files, nil
}

// attachAssets uploads each file as an asset of the release, replacing an asset of the same name.
func attachAssets(ctx context.Context, client *http.Client, po PublishOptions, rel release, base string) error {
	files, err := attachFiles(po.Dir, po.Read)
	if err != nil {
		return err
	}
	github := hostmatch.ResolveKind(po.Server) == hostmatch.KindGitHub
	existing := map[string]int64{}
	for _, a := range rel.Assets {
		existing[a.Name] = a.ID
	}
	for _, f := range files {
		name := filepath.Base(f)
		if po.DryRun {
			genlog.Info("dry run, asset not attached", "file", name, "replaces", existing[name] != 0)
			continue
		}
		if id := existing[name]; id != 0 {
			del := fmt.Sprintf("%s/%d/assets/%d", base, rel.ID, id)
			if github {
				del = fmt.Sprintf("%s/repos/%s/releases/assets/%d", strings.TrimSuffix(po.API, "/"), po.Repo, id)
			}
			if err := send(ctx, client, po.Token, http.MethodDelete, del, "", nil); err != nil {
				return err
			}
			genlog.Info("asset replaced", "file", name, "id", id)
		}
		data, err := os.ReadFile(f) // #nosec G304 -- f is resolved inside the project by attachFiles
		if err != nil {
			return err
		}
		target, contentType, body := base+"/"+strconv.FormatInt(rel.ID, 10)+"/assets?name="+url.QueryEscape(name), "application/octet-stream", data
		if github {
			upload, _, _ := strings.Cut(rel.UploadURL, "{")
			target = upload + "?name=" + url.QueryEscape(name)
		} else if body, contentType, err = multipartBody(name, data); err != nil {
			return err
		}
		if err := send(ctx, client, po.Token, http.MethodPost, target, contentType, body); err != nil {
			return err
		}
		genlog.Info("asset attached", "file", name, "bytes", len(data), "release", rel.ID)
	}
	return nil
}

// multipartBody wraps data as the `attachment` form field Forgejo expects.
func multipartBody(name string, data []byte) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("attachment", name)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// send makes one raw request with retry and fails on a non-2xx answer.
func send(ctx context.Context, client *http.Client, token, method, target, contentType string, body []byte) error {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, target, nil)
	}
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	resp, err := core.DoWithRetry(ctx, client, req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, target, err)
	}
	raw, err := core.ReadAndClose(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s %s: HTTP %d: %s", method, target, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
