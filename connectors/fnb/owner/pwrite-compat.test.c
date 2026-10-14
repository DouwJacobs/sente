// SPDX-License-Identifier: GPL-3.0-only
#define _GNU_SOURCE
#define _LARGEFILE64_SOURCE
#include <assert.h>
#include <errno.h>
#include <fcntl.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

int main(void) {
    char path[] = "/tmp/sente-pwrite-test-XXXXXX";
    int fd = mkstemp(path);
    assert(fd >= 0);
    assert(unlink(path) == 0);
    assert(lseek(fd, 2, SEEK_SET) == 2);
    assert(pwrite(fd, "abc", 3, 5) == 3);
    assert(lseek(fd, 0, SEEK_CUR) == 2);
    char data[3];
    assert(pread(fd, data, 3, 5) == 3 && memcmp(data, "abc", 3) == 0);
    off64_t large = ((off64_t)1 << 32) + 5;
    assert(pwrite64(fd, "def", 3, large) == 3);
    assert(pread64(fd, data, 3, large) == 3 && memcmp(data, "def", 3) == 0);
    assert(lseek(fd, 0, SEEK_CUR) == 2);
    errno = 0;
    assert(pwrite(fd, "a", 1, -1) == -1 && errno == EINVAL);
    assert(close(fd) == 0);
    errno = 0;
    assert(pwrite(-1, "a", 1, 0) == -1 && errno == EBADF);
    return 0;
}
