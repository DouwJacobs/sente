# Chromium container sandbox

`chromium-seccomp.json` is the Playwright Docker default-deny profile from
microsoft/playwright commit `d9f2fd3e2232ace8e317a84eda6cb86e25bcf49c`,
`utils/docker/seccomp_profile.json`, adapted under Apache-2.0
(see PLAYWRIGHT-LICENSE). It permits clone/setns/unshare/chroot so non-root Chromium
can create its own user, PID and network namespaces. The added chroot permission lets Chromium isolate its zygote inside its new user namespace; no SYS_CHROOT capability is granted to Sente. It does not disable seccomp.
Source: https://github.com/microsoft/playwright/blob/d9f2fd3e2232ace8e317a84eda6cb86e25bcf49c/utils/docker/seccomp_profile.json

Compose applies this profile to the one Sente container, retains no-new-privileges,
drops all capabilities, uses a read-only root, provides private 256 MiB shared memory,
and uses an init process to reap children. No host IPC/network, privileged mode,
Docker socket, browser debugging port or sandbox-disabling flags are required.
Host kernel and security policy must permit unprivileged user namespaces.
A host restriction must be investigated; do not bypass it by disabling the sandbox.

The release-image check launches the same browser code with networking disabled,
checks a synthetic DOM, verifies Chromium's own namespace/seccomp status and
removes the private temporary profile. Live banking and MFA remain owner checks.
