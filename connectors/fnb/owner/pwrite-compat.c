// SPDX-License-Identifier: GPL-3.0-only
// Alpine musl may implement pwrite through pwritev2. Chromium's child sandbox
// permits the equivalent established pwrite64 syscall but traps pwritev2.
// This library changes syscall selection, never permissions or write semantics.
#define _GNU_SOURCE
#include <errno.h>
#include <pthread.h>
#include <sys/types.h>
#include <unistd.h>
#include <sys/syscall.h>
#if !defined(__linux__) || !defined(__x86_64__)
#error This compatibility adapter is supported only by the linux/amd64 image.
#endif

ssize_t pwrite(int fd, const void *buffer, size_t length, off_t offset) {
    pthread_testcancel();
    ssize_t result = syscall(SYS_pwrite64, fd, buffer, length, offset);
    int saved_errno = errno;
    pthread_testcancel();
    errno = saved_errno;
    return result;
}
extern __typeof(pwrite) pwrite64 __attribute__((alias("pwrite")));
