#define _GNU_SOURCE
#include <sys/auxv.h>

#define SVE_BIT (1UL << 22)
#define SME_MASK ((0xffUL << 23) | (0x3fUL << 37) | (0x3fUL << 57))
/* Linux arm64 hwcap.h: SVE/SME only; preserve NEON and unrelated capabilities. */
#define HWCAP_MASK (SVE_BIT | (0x7ffUL << 37))
#define HWCAP2_MASK (SME_MASK | (0x3fUL << 1) | (0xfUL << 9) | \
                     (1UL << 33) | (1UL << 36) | (1UL << 45))

#ifdef WX_HWCAP_PROBE
int main(void)
{
    return !((getauxval(AT_HWCAP2) & SME_MASK) &&
             !(getauxval(AT_HWCAP) & SVE_BIT));
}
#else
#include <dlfcn.h>
#include <errno.h>

unsigned long getauxval(unsigned long type)
{
    unsigned long (*real_getauxval)(unsigned long) = dlsym(RTLD_NEXT, "getauxval");
    if (!real_getauxval) {
        errno = ENOENT;
        return 0;
    }
    unsigned long value = real_getauxval(type);
    if (type == AT_HWCAP)
        return value & ~HWCAP_MASK;
    if (type == AT_HWCAP2)
        return value & ~HWCAP2_MASK;
    return value;
}

unsigned long __getauxval(unsigned long) __attribute__((alias("getauxval"), nothrow, leaf));
#endif
