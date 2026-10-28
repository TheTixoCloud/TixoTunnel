<!-- Artifact review by ir_spoof - https://t.me/ir_spoof -->
# Security and privacy notes

The supplied executable is an obfuscated Linux amd64 Go program. Its SHA-256 is
`084ccd5c63605f58a655ab8b53d75552c1e878a4c0fc4168cd0b45f183d8ed1a`
(13,143,728 bytes). The supplied obfuscated Bash script's SHA-256 is
`17b30852df24a4ef3dedec85ade0b31659fd9a5ba4a2a410a1eacac52bc8efa1`.
These are provenance identifiers, not endorsements or release core hashes.

Static code references established vendor communication beyond mere strings:
the original contains periodic telemetry with `server_ip`, app version and
transport type, vendor license/nonce requests, and a public-IP lookup. The script
also sends the first local address to a geolocation service twice at startup.
It downloads executable archives over unauthenticated plain HTTP and evaluates
raw prompt input as a root shell expression.

The independent implementation contains none of those vendor request paths. Its
helper performs local installation using an expected checksum, never downloads
updates automatically, never evaluates user input as shell code, and uses typed
configuration plus private random secrets. Original/patched premium executables,
license records, private packet captures and workstation identity are not part
of the source or runtime bundle.

Local tokens and PSKs authenticate/encrypt the two peers. They are not
telemetry. Copy a peer secret only over a trusted private channel and keep its
file at mode 0600. Reusing a public example token or embedding a secret in a
command line, screenshot, source repository or release archive defeats that
protection. The provided manager generates secrets without printing them.

No positive evidence of credential theft, file infection or ransomware was
established in the inspected original paths. That is not a malware-free
certificate: the binary is obfuscated and finite static/dynamic inspection
cannot exclude every possible behavior. Untrusted original execution belongs
only in an isolated disposable VM. The distribution is independently written
source and its rebuilt core, rather than a patched vendor binary presented as
safe.

The replacement uses new cryptographic framing. TLS identity checks are on by
default and the original problematic IPX nonce construction is not copied.
These choices do not replace independent protocol/security review. The measured
[test report](test-report.md) describes actual VM coverage and any observed
limits.

An offline install is not anonymity: peers see the network addresses involved,
configured hostnames can require DNS, and an administrator can inspect local
configurations and service logs. Choosing a source bind or raw spoof address
does not provide an anonymous public egress service.

The MIT notice covers independent code. Dependency license and copyright
notices must remain with redistributions; `ir_spoof` attribution must not be
used to misidentify third-party components as newly authored code.
