#include "CPTY.h"

#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <unistd.h>
#include <util.h>

pid_t jade_spawn_pty(const char *cwd, const char *shell, int *master_fd) {
    if (cwd == NULL || shell == NULL || master_fd == NULL) {
        errno = EINVAL;
        return -1;
    }
    struct winsize size = {.ws_row = 28, .ws_col = 100};
    pid_t pid = forkpty(master_fd, NULL, NULL, &size);
    if (pid != 0) {
        return pid;
    }
    if (chdir(cwd) != 0) {
        _exit(126);
    }
    setenv("TERM", "xterm-ghostty", 1);
    setenv("COLORTERM", "truecolor", 1);
    execl(shell, shell, "-l", (char *)NULL);
    _exit(127);
}

int jade_resize_pty(int master_fd, unsigned short columns, unsigned short rows) {
    if (columns < 2 || rows < 2) {
        errno = EINVAL;
        return -1;
    }
    struct winsize size = {.ws_row = rows, .ws_col = columns};
    return ioctl(master_fd, TIOCSWINSZ, &size);
}
