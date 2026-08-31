#ifndef JADE_CPTY_H
#define JADE_CPTY_H

#include <sys/types.h>

pid_t jade_spawn_pty(const char *cwd, const char *shell, int *master_fd);
int jade_resize_pty(int master_fd, unsigned short columns, unsigned short rows);

#endif
