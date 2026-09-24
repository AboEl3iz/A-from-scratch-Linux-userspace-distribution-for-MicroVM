/*
 * Karim MicroVM OS - Static C Shell (/bin/sh)
 * Supports builtins (echo, sleep, true, cd, exit), statement chaining (;),
 * while-do-done loops, and external executable invocation via fork/execvp.
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/wait.h>
#include <sys/stat.h>
#include <ctype.h>

#define MAX_ARGS 64
#define MAX_CMD_LEN 1024

static void execute_single_command(char *cmdline);

static char *trim_spaces(char *str) {
	while (isspace((unsigned char)*str)) str++;
	if (*str == 0) return str;
	char *end = str + strlen(str) - 1;
	while (end > str && isspace((unsigned char)*end)) end--;
	end[1] = '\0';
	return str;
}

static void unquote(char *str) {
	size_t len = strlen(str);
	if (len >= 2 && ((str[0] == '\'' && str[len-1] == '\'') || (str[0] == '"' && str[len-1] == '"'))) {
		memmove(str, str + 1, len - 2);
		str[len - 2] = '\0';
	}
}

static void builtin_echo(int argc, char *argv[]) {
	for (int i = 1; i < argc; i++) {
		char arg_copy[MAX_CMD_LEN];
		strncpy(arg_copy, argv[i], sizeof(arg_copy) - 1);
		arg_copy[sizeof(arg_copy) - 1] = '\0';
		unquote(arg_copy);
		printf("%s%s", arg_copy, (i == argc - 1) ? "" : " ");
	}
	printf("\n");
	fflush(stdout);
}

static void builtin_sleep(int argc, char *argv[]) {
	if (argc >= 2) {
		int seconds = atoi(argv[1]);
		if (seconds > 0) {
			sleep(seconds);
		}
	}
}

static void execute_single_command(char *cmdline) {
	char clean_buf[MAX_CMD_LEN];
	strncpy(clean_buf, cmdline, sizeof(clean_buf) - 1);
	clean_buf[sizeof(clean_buf) - 1] = '\0';

	char *trimmed = trim_spaces(clean_buf);
	if (strlen(trimmed) == 0) return;

	// Check for while loop construct: while ...; do ...; done
	if (strncmp(trimmed, "while ", 6) == 0) {
		char *do_ptr = strstr(trimmed, "do ");
		char *done_ptr = strrchr(trimmed, ';');
		if (done_ptr && strstr(done_ptr, "done")) {
			*done_ptr = '\0';
		} else {
			done_ptr = strstr(trimmed, "done");
			if (done_ptr) *done_ptr = '\0';
		}

		if (do_ptr) {
			char *body = do_ptr + 3;
			while (1) {
				char body_copy[MAX_CMD_LEN];
				strncpy(body_copy, body, sizeof(body_copy) - 1);
				body_copy[sizeof(body_copy) - 1] = '\0';

				char *stmt_save = NULL;
				char *stmt = strtok_r(body_copy, ";", &stmt_save);
				while (stmt != NULL) {
					execute_single_command(stmt);
					stmt = strtok_r(NULL, ";", &stmt_save);
				}
			}
			return;
		}
	}

	char *args[MAX_ARGS];
	int argc = 0;
	char *token_save = NULL;
	char *token = strtok_r(trimmed, " \t\r\n", &token_save);

	while (token != NULL && argc < MAX_ARGS - 1) {
		args[argc++] = token;
		token = strtok_r(NULL, " \t\r\n", &token_save);
	}
	args[argc] = NULL;

	if (argc == 0) return;

	if (strcmp(args[0], "exit") == 0) {
		exit(0);
	} else if (strcmp(args[0], "cd") == 0) {
		const char *dir = (argc > 1) ? args[1] : "/";
		if (chdir(dir) < 0) perror("cd");
		return;
	} else if (strcmp(args[0], "echo") == 0) {
		builtin_echo(argc, args);
		return;
	} else if (strcmp(args[0], "sleep") == 0) {
		builtin_sleep(argc, args);
		return;
	} else if (strcmp(args[0], "true") == 0) {
		return;
	}

	pid_t pid = fork();
	if (pid == 0) {
		execvp(args[0], args);
		fprintf(stderr, "sh: command not found: %s\n", args[0]);
		_exit(127);
	} else if (pid > 0) {
		int status;
		waitpid(pid, &status, 0);
	} else {
		perror("fork");
	}
}

static void execute_script_line(char *cmdline) {
	char line_buf[MAX_CMD_LEN];
	strncpy(line_buf, cmdline, sizeof(line_buf) - 1);
	line_buf[sizeof(line_buf) - 1] = '\0';

	char *while_ptr = strstr(line_buf, "while ");
	if (while_ptr && while_ptr > line_buf) {
		char pre_cmd[MAX_CMD_LEN];
		size_t len = while_ptr - line_buf;
		strncpy(pre_cmd, line_buf, len);
		pre_cmd[len] = '\0';

		char *saveptr = NULL;
		char *stmt = strtok_r(pre_cmd, ";", &saveptr);
		while (stmt != NULL) {
			execute_single_command(stmt);
			stmt = strtok_r(NULL, ";", &saveptr);
		}

		execute_single_command(while_ptr);
		return;
	}

	if (while_ptr == line_buf) {
		execute_single_command(line_buf);
		return;
	}

	char *saveptr = NULL;
	char *stmt = strtok_r(line_buf, ";", &saveptr);
	while (stmt != NULL) {
		execute_single_command(stmt);
		stmt = strtok_r(NULL, ";", &saveptr);
	}
}

int main(int argc, char *argv[]) {
	if (argc >= 3 && strcmp(argv[1], "-c") == 0) {
		execute_script_line(argv[2]);
		return 0;
	}

	printf("Karim MicroVM OS Shell (/bin/sh)\n");
	char line[MAX_CMD_LEN];

	while (1) {
		printf("karim# ");
		fflush(stdout);

		if (fgets(line, sizeof(line), stdin) == NULL) {
			printf("\n");
			break;
		}

		execute_script_line(line);
	}

	return 0;
}
