#define FUSE_USE_VERSION 31

/* A basic example of a virtual file system using libfuse library.
 * Mounts a virtual file system to the mounting point specified,
 * and includes a "Hello" file containing "Hello World!".
 * 
 * Compile with:
 *
 *     gcc -Wall hello.c `pkg-config fuse3 --cflags --libs` -o hello
 *
 * Run with:
 *
 *     ./hello <path-to-mounting-point>
 */

#include <fuse.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <fcntl.h>
#include <stddef.h>
#include <assert.h>

static struct options {
    const char * f_name;
    const char * contents;
} options;

#define OPTION(t, p) \
    {t, offsetof(struct options, p), 1}

static const struct fuse_opt option_spec[] = {
    OPTION("--name=%s", f_name),
    OPTION("--contents=%s", contents),
    FUSE_OPT_END
};

static void * hello_init(struct fuse_conn_info * conn, struct fuse_config * confg) {
    (void) conn;
    confg->kernel_cache = 1; // enable caching
    return NULL;
}

static int hello_getattr(const char * path, struct stat * sbuf, struct fuse_file_info * file_info) {
    (void) file_info;
    int result = 0;

    memset(sbuf, 0, sizeof(struct stat));
    
    if (strcmp(path, "/") == 0) { // if we're in root dir
        sbuf->st_mode = S_IFDIR | 0755; // read, write, and execute permissions for the owner and read and execute permissions for others 
        sbuf->st_nlink = 2; // 2 links, dir and parent dir
    } else if (strcmp(path + 1, options.f_name) == 0) {
        sbuf->st_mode = S_IFREG | 0444; // read only for everyone
        sbuf->st_nlink = 1;
        sbuf->st_size = strlen(options.contents); // size of file in bites
    } else {
        result = -ENOENT; // file doesn't exist
    }

    return result;
}

static int hello_readdir(const char * path, void * buff, fuse_fill_dir_t filler, off_t offset, struct fuse_file_info * file_info, enum fuse_readdir_flags flags) {
    (void) offset;
    (void) file_info;
    (void) flags;

    if (strcmp(path, "/") != 0) {
        return -ENOENT; // dir doesn't exist
    }

    filler(buff, ".", NULL, 0, 0); // simple dir, can just add 0
    filler(buff, "..", NULL, 0, 0);
    filler(buff, options.f_name, NULL, 0, 0); // the hello file

    return 0;
}

static int hello_open(const char * path, struct fuse_file_info * file_info) {
    if (strcmp(path + 1, options.f_name) != 0) {
        return -ENOENT; // file doesnt exist
    }

    if ((file_info->flags & O_ACCMODE) != O_RDONLY) {
        return -EACCES; // perms denied, file isn't read only
    }

    return 0;
}

static int hello_read(const char * path, char * buff, size_t size, off_t offset, struct fuse_file_info * file_info) {
    (void) file_info;
    if (strcmp(path + 1, options.f_name) != 0) {
        return -ENOENT;
    }

    size_t len = strlen(options.contents);
    if (offset < len) { // read data if in valid range
        if (offset + size > len) { // size reads to end of data (if too big)
            size = len - offset;
        }
        memcpy(buff, options.contents + offset, size);
    } else { // if the offset is greater/equal, there is no data to read
        size = 0;
    }

    return size;
}

// handles callback functions
static const struct fuse_operations hello_oper = {
	.init       = hello_init,
	.getattr	= hello_getattr,
	.readdir	= hello_readdir,
	.open		= hello_open,
	.read		= hello_read,
};

int main(int argc, char ** argv) {
    int fs;
    struct fuse_args args = FUSE_ARGS_INIT(argc, argv);

    options.f_name = strdup("Hello"); // default values
    options.contents = strdup("Hello World!\n");

    if (fuse_opt_parse(&args, &options, option_spec, NULL) == -1) { // parse the command line
        return 1;
    }

    fs = fuse_main(args.argc, args.argv, &hello_oper, NULL); // runs the FUSE filesystem
    fuse_opt_free_args(&args); // frees memory
    return fs;
}
