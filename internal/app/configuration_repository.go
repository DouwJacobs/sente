package app

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"
)

var configurationRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

func validateConfigurationRepository(s configurationSource) (*url.URL, error) {
	u, err := url.Parse(s.RepositoryURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || u.Path == "" {
		return nil, fail(400, "Use a public HTTPS Git repository URL without credentials")
	}
	if len(s.RepositoryURL) > 1000 {
		return nil, fail(400, "Repository URL is too long")
	}
	if s.Ref != "" && (!configurationRef.MatchString(s.Ref) || strings.Contains(s.Ref, "..") || strings.Contains(s.Ref, "//")) {
		return nil, fail(400, "Use a branch, tag or commit reference")
	}
	if s.Path == "" || len(s.Path) > 500 || strings.ContainsAny(s.Path, "\\:\x00\r\n") || path.IsAbs(s.Path) || path.Clean(s.Path) != s.Path || strings.HasPrefix(s.Path, "../") || s.Path == ".." || !strings.HasSuffix(s.Path, ".json") {
		return nil, fail(400, "Use a relative JSON file path within the repository")
	}
	return u, nil
}
func publicConfigurationAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, prefix := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "fec0::/10", "2001:db8::/32"} {
		if netip.MustParsePrefix(prefix).Contains(ip) {
			return false
		}
	}
	return true
}

// Git reads blobs without checking out content or running repository hooks/filters.
// Isolated environment/config and HTTPS-only transport prevent local credential reuse.
// Resolve and pin public addresses for the fetch; redirects and proxies are disabled.
func fetchConfigurationRepository(ctx context.Context, s configurationSource) ([]byte, string, error) {
	u, err := validateConfigurationRepository(s)
	if err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, "", fail(400, "Repository host could not be resolved")
	}
	for _, ip := range addresses {
		if !publicConfigurationAddress(ip) {
			return nil, "", fail(400, "Use a repository hosted on a public address")
		}
	}
	// curl resolve support in Git pins DNS while retaining TLS certificate validation.
	address := addresses[0].Unmap().String()
	if addresses[0].Is6() {
		address = "[" + address + "]"
	}
	directory, err := os.MkdirTemp("", "sente-configuration-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(directory)
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil, "", fail(400, "Install Git on the Sente host to pull repositories, or import a file")
	}
	run := func(args ...string) ([]byte, error) {
		options := []string{"-c", "core.hooksPath=/dev/null", "-c", "init.templateDir=", "-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "http.followRedirects=false", "-c", "http.proxy=", "-c", "http.sslVerify=true", "-c", fmt.Sprintf("http.curloptResolve=%s:443:%s", u.Hostname(), address)}
		cmd := exec.CommandContext(ctx, gitPath, append(options, args...)...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + directory, "XDG_CONFIG_HOME=" + directory, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "LC_ALL=C"}
		var output limitedConfigurationBuffer
		cmd.Stdout = &output
		cmd.Stderr = nil // Never echo arbitrary remote output into errors/logs.
		err := cmd.Run()
		if err != nil {
			return nil, err
		}
		return output.Bytes(), nil
	}
	repo := directory + "/repository"
	if _, err = run("init", "--bare", repo); err != nil {
		return nil, "", fail(400, "Repository pull failed; check the URL and Git installation")
	}
	ref := s.Ref
	if ref == "" {
		ref = "HEAD"
	}
	if _, err = run("--git-dir", repo, "fetch", "--depth=1", "--filter=blob:limit=33554432", "--no-tags", "--no-recurse-submodules", "--", s.RepositoryURL, ref); err != nil {
		return nil, "", fail(400, "Repository pull failed; check public access and the branch, tag or commit")
	}
	revision, err := run("--git-dir", repo, "rev-parse", "FETCH_HEAD")
	if err != nil {
		return nil, "", fail(400, "Could not read the repository revision")
	}
	commit := strings.TrimSpace(string(revision))
	object, err := run("--git-dir", repo, "ls-tree", commit, "--", s.Path)
	if err != nil || !bytes.HasPrefix(object, []byte("100644 blob ")) && !bytes.HasPrefix(object, []byte("100755 blob ")) {
		return nil, "", fail(400, "Choose a regular JSON file in the repository")
	}
	blob, err := run("--git-dir", repo, "show", commit+":"+s.Path)
	if err != nil {
		return nil, "", fail(400, "Could not read the configuration file; it must be at most 32 MiB")
	}
	return blob, commit, nil
}

type limitedConfigurationBuffer struct{ bytes.Buffer }

func (b *limitedConfigurationBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxRulesetBytes {
		return 0, fmt.Errorf("configuration output exceeds 32 MiB")
	}
	return b.Buffer.Write(p)
}
