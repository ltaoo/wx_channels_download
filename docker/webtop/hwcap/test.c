#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <stdlib.h>
#include <sys/auxv.h>

#ifdef WX_HWCAP_FIXTURE
unsigned long getauxval(unsigned long type)
{
    if (type == AT_HWCAP || type == AT_HWCAP2) {
        const char *value = getenv(type == AT_HWCAP ? "TEST_HWCAP" : "TEST_HWCAP2");
        /* The loader can call this before environ is initialized. */
        return value ? strtoul(value, NULL, 0) : 0;
    }
    unsigned long (*real_getauxval)(unsigned long) = dlsym(RTLD_NEXT, "getauxval");
    return real_getauxval(type);
}
#else
#include <assert.h>
#include <unistd.h>

int main(int argc, char **argv)
{
    assert(argc == 3);
    unsigned long (*alias_getauxval)(unsigned long) = dlsym(RTLD_DEFAULT, "__getauxval");
    assert(alias_getauxval);
    assert(getauxval(AT_HWCAP) == strtoul(argv[1], NULL, 0));
    assert(getauxval(AT_HWCAP2) == strtoul(argv[2], NULL, 0));
    assert(alias_getauxval(AT_HWCAP) == getauxval(AT_HWCAP));
    assert(alias_getauxval(AT_HWCAP2) == getauxval(AT_HWCAP2));
    assert(getauxval(AT_PAGESZ) == (unsigned long)sysconf(_SC_PAGESIZE));
    errno = 0;
    assert(getauxval(~0UL) == 0);
    assert(errno == ENOENT);
    return 0;
}
#endif
