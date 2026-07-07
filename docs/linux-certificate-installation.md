# Linux Certificate Installation Guide

This document describes the permissions and installation steps required to install a trusted CA certificate on common Linux distributions.

## Summary

Installing a certificate into the system trust store usually requires `root` or `sudo`.

The installer must write to system-owned directories such as `/usr/local/share/ca-certificates`, `/etc/pki/ca-trust/source/anchors`, or `/etc/ca-certificates/trust-source/anchors`, then refresh the distribution's generated CA bundle.

Browser-specific stores, Java stores, containers, Flatpak apps, and Snap apps may require additional handling.

## Certificate Format

The certificate should be a CA certificate in PEM format:

```text
-----BEGIN CERTIFICATE-----
...
-----END CERTIFICATE-----
```

Some tools accept DER input, but system CA installation is most reliable when the certificate is written as PEM. Debian-style `update-ca-certificates` expects local certificates under `/usr/local/share/ca-certificates` to use a `.crt` suffix.

## Distribution Matrix

| Distribution family | Certificate path | Refresh command | Permission |
| --- | --- | --- | --- |
| Debian, Ubuntu, Linux Mint | `/usr/local/share/ca-certificates/<name>.crt` | `update-ca-certificates` | root required |
| Alpine | `/usr/local/share/ca-certificates/<name>.crt` | `update-ca-certificates` | root required |
| Gentoo | `/usr/local/share/ca-certificates/<name>.crt` | `update-ca-certificates` | root required |
| RHEL, CentOS, Rocky Linux, AlmaLinux, Fedora | `/etc/pki/ca-trust/source/anchors/<name>.crt` | `update-ca-trust extract` | root required |
| Arch, Manjaro | `/etc/ca-certificates/trust-source/anchors/<name>.crt` | `trust extract-compat` | root required |
| openSUSE, SLES | `/usr/share/pki/trust/anchors/<name>.pem` | `update-ca-certificates` | root required |

## Debian, Ubuntu, Linux Mint

Install:

```bash
sudo install -m 0644 cert.crt /usr/local/share/ca-certificates/<name>.crt
sudo update-ca-certificates
```

Uninstall:

```bash
sudo rm /usr/local/share/ca-certificates/<name>.crt
sudo update-ca-certificates
```

## Alpine

Install:

```bash
sudo install -m 0644 cert.crt /usr/local/share/ca-certificates/<name>.crt
sudo update-ca-certificates
```

If `ca-certificates` is not installed:

```bash
sudo apk add ca-certificates
```

Uninstall:

```bash
sudo rm /usr/local/share/ca-certificates/<name>.crt
sudo update-ca-certificates
```

## RHEL, CentOS, Rocky Linux, AlmaLinux, Fedora

Install:

```bash
sudo install -m 0644 cert.crt /etc/pki/ca-trust/source/anchors/<name>.crt
sudo update-ca-trust extract
```

Uninstall:

```bash
sudo rm /etc/pki/ca-trust/source/anchors/<name>.crt
sudo update-ca-trust extract
```

## Arch, Manjaro

Install:

```bash
sudo install -m 0644 cert.crt /etc/ca-certificates/trust-source/anchors/<name>.crt
sudo trust extract-compat
```

Alternative:

```bash
sudo trust anchor --store cert.crt
sudo trust extract-compat
```

Uninstall:

```bash
sudo rm /etc/ca-certificates/trust-source/anchors/<name>.crt
sudo trust extract-compat
```

## openSUSE, SLES

Install:

```bash
sudo install -m 0644 cert.pem /usr/share/pki/trust/anchors/<name>.pem
sudo update-ca-certificates
```

Uninstall:

```bash
sudo rm /usr/share/pki/trust/anchors/<name>.pem
sudo update-ca-certificates
```

## NSS Stores for Desktop Browsers

System trust store installation does not always cover every browser.

NSS databases are used by Firefox and some Chromium builds. User NSS databases usually do not require root, but if the program was elevated with `sudo`, it must write to the original user's home directory, not root's home directory.

Common NSS paths:

```text
/etc/pki/nssdb
$HOME/.pki/nssdb
$HOME/.mozilla/firefox/<profile>
```

Install:

```bash
certutil -d sql:$HOME/.pki/nssdb -A -n "<name>" -t "CT,C,C" -i cert.crt
```

Firefox profile example:

```bash
certutil -d sql:$HOME/.mozilla/firefox/<profile> -A -n "<name>" -t "CT,C,C" -i cert.crt
```

Uninstall:

```bash
certutil -d sql:$HOME/.pki/nssdb -D -n "<name>"
certutil -d sql:$HOME/.mozilla/firefox/<profile> -D -n "<name>"
```

If the NSS database does not exist:

```bash
mkdir -p "$HOME/.pki/nssdb"
certutil -d sql:$HOME/.pki/nssdb -N --empty-password
```

## Java Trust Store

Java applications may use their own trust store instead of the system store.

Install:

```bash
sudo keytool -importcert \
  -alias "<name>" \
  -file cert.crt \
  -keystore "$JAVA_HOME/lib/security/cacerts"
```

The common default password is `changeit`, but it can be different.

Uninstall:

```bash
sudo keytool -delete \
  -alias "<name>" \
  -keystore "$JAVA_HOME/lib/security/cacerts"
```

## Containers

Certificates installed on the host are not automatically trusted inside containers. Install the certificate inside the image or container.

Debian, Ubuntu, Alpine:

```dockerfile
COPY cert.crt /usr/local/share/ca-certificates/<name>.crt
RUN update-ca-certificates
```

RHEL, CentOS, Fedora:

```dockerfile
COPY cert.crt /etc/pki/ca-trust/source/anchors/<name>.crt
RUN update-ca-trust extract
```

The Docker build stage usually runs as root, so these commands can write to system trust directories.

## Flatpak and Snap

Flatpak and Snap applications can be isolated from the host system trust store. Some apps use bundled runtimes or sandbox-specific certificate configuration.

When a system CA install does not affect a sandboxed app, check the app's runtime documentation and whether the certificate must be added inside the sandbox or application-specific store.

## Implementation Notes

For programmatic installation:

- Keep root checks for system trust store installation.
- Use the externally supplied certificate name as the single source of truth.
- Derive only the filesystem-safe certificate file name from that external name.
- Use the original external name as the NSS nickname.
- Do not hardcode certificate names such as `SunnyRoot` or `WeChatAppEx_CA`.
- In `sudo` flows, use `SUDO_USER` or `PKEXEC_UID` to find the invoking desktop user's home directory.
- If writing a user NSS database while running as root, restore ownership to the original user.
- Treat system trust store refresh failure as a hard error.
- Treat NSS browser store failure as a warning when the target machine may be a server without desktop components.
