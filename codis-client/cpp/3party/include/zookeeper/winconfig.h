/* Define to 1 if you have the <arpa/inet.h> header file. */
#undef HAVE_ARPA_INET_H

/* Define to 1 if you have the <dlfcn.h> header file. */
#undef HAVE_DLFCN_H

/* Define to 1 if you have the <fcntl.h> header file. */
#undef HAVE_FCNTL_H

/* Define to 1 if you have the file `generated/zookeeper.jute.c'. */
#define HAVE_GENERATED_ZOOKEEPER_JUTE_C 1

/* Define to 1 if you have the file `generated/zookeeper.jute.h'. */
#define HAVE_GENERATED_ZOOKEEPER_JUTE_H 1

/* Define to 1 if you have the `getcwd' function. */
#undef HAVE_GETCWD

/* Define to 1 if you have the `gethostbyname' function. */
#undef HAVE_GETHOSTBYNAME

/* Define to 1 if you have the `gethostname' function. */
#define HAVE_GETHOSTNAME 1

/* Define to 1 if you have the `getlogin' function. */
#undef HAVE_GETLOGIN

/* Define to 1 if you have the `getpwuid_r' function. */
#undef HAVE_GETPWUID_R

/* Define to 1 if you have the `gettimeofday' function. */
#undef HAVE_GETTIMEOFDAY

/* Define to 1 if you have the `getuid' function. */
#undef HAVE_GETUID

/* Define to 1 if you have the <inttypes.h> header file. */
#undef HAVE_INTTYPES_H

/* Define to 1 if you have the `memmove' function. */
#undef HAVE_MEMMOVE

/* Define to 1 if you have the <memory.h> header file. */
#undef HAVE_MEMORY_H

/* Define to 1 if you have the `memset' function. */
#undef HAVE_MEMSET

/* Define to 1 if you have the <netdb.h> header file. */
#undef HAVE_NETDB_H

/* Define to 1 if you have the <netinet/in.h> header file. */
#undef HAVE_NETINET_IN_H

/* Define to 1 if you have the `poll' function. */
#undef HAVE_POLL

/* Define to 1 if you have the `socket' function. */
#undef HAVE_SOCKET

/* Define to 1 if you have the <stdint.h> header file. */
#undef HAVE_STDINT_H

/* Define to 1 if you have the <stdlib.h> header file. */
#define HAVE_STDLIB_H 1

/* Define to 1 if you have the `strchr' function. */
#define HAVE_STRCHR 1

/* Define to 1 if you have the `strdup' function. */
#define HAVE_STRDUP 1

/* Define to 1 if you have the `strerror' function. */
#define HAVE_STRERROR 1

/* Define to 1 if you have the <strings.h> header file. */
#undef HAVE_STRINGS_H

/* Define to 1 if you have the <string.h> header file. */
#undef HAVE_STRING_H

/* Define to 1 if you have the `strtol' function. */
#undef HAVE_STRTOL

/* Define to 1 if you have the <sys/socket.h> header file. */
#undef HAVE_SYS_SOCKET_H

/* Define to 1 if you have the <sys/stat.h> header file. */
#undef HAVE_SYS_STAT_H

/* Define to 1 if you have the <sys/time.h> header file. */
#undef HAVE_SYS_TIME_H

/* Define to 1 if you have the <sys/types.h> header file. */
#undef HAVE_SYS_TYPES_H

/* Define to 1 if you have the <sys/utsname.h> header file. */
#undef HAVE_SYS_UTSNAME_H

/* Define to 1 if you have the <unistd.h> header file. */
#undef HAVE_UNISTD_H

/* Define to the sub-directory in which libtool stores uninstalled libraries.
   */
#define LT_OBJDIR

/* Define to 1 if your C compiler doesn't accept -c and -o together. */
/* #undef NO_MINUS_C_MINUS_O */

/* Name of package */
#define PACKAGE "c-client-src"

/* Define to the address where bug reports for this package should be sent. */
#define PACKAGE_BUGREPORT "user@zookeeper.apache.org"

/* Define to the full name of this package. */
#define PACKAGE_NAME "zookeeper C client"

/* Define to the full name and version of this package. */
#define PACKAGE_STRING "zookeeper C client 3.4.0 win32"

/* Define to the one symbol short name of this package. */
#define PACKAGE_TARNAME "c-client-src"

/* Define to the home page for this package. */
#define PACKAGE_URL ""

/* Define to the version of this package. */
#define PACKAGE_VERSION "3.4.0"

/* poll() second argument type */
#define POLL_NFDS_TYPE

/* Define to 1 if you have the ANSI C header files. */
#define STDC_HEADERS

/* Define to 1 if you can safely include both <sys/time.h> and <time.h>. */
#define TIME_WITH_SYS_TIME

/* Version number of package */
#define VERSION "3.4.0"

/* Define to empty if `const' does not conform to ANSI C. */
/* #undef const */

/* Define to `__inline__' or `__inline' if that's what the C compiler
   calls it, or to nothing if 'inline' is not supported under any name.  */
#ifndef __cplusplus
#define inline __inline
#endif
#ifdef WIN32
#define __attribute__(x)
#define __func__ __FUNCTION__

#ifndef _WIN32_WINNT_NT4
#define _WIN32_WINNT_NT4 0x0400
#endif

#define NTDDI_VERSION _WIN32_WINNT_NT4
#define _WIN32_WINNT _WIN32_WINNT_NT4

#define _CRT_SECURE_NO_WARNINGS
#define WIN32_LEAN_AND_MEAN
#include <Windows.h>
#include <Winsock2.h>
#include <winstdint.h>
#include <process.h>
#include <ws2tcpip.h>
#undef AF_INET6
#undef min
#undef max

#include <errno.h>

#define strtok_r strtok_s
#define localtime_r(a,b) localtime_s(b,a)
#define get_errno() errno=GetLastError()
#define random rand
#define snprintf _snprintf

#define ACL ZKACL  // Conflict with windows API

#define EAI_ADDRFAMILY WSAEINVAL
#define EHOSTDOWN EPIPE
#define ESTALE ENODEV

#define EWOULDBLOCK WSAEWOULDBLOCK
#define EINPROGRESS WSAEINPROGRESS

typedef int pid_t;
#endif
